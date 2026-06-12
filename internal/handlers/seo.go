package handlers

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"time"

	"github.com/s/onlineCourse/internal/models"
)

func (h *Handler) HandleRobotsTxt(w http.ResponseWriter, r *http.Request) {
	baseURL := siteBaseURL()
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintf(w, `User-agent: *
Allow: /
Allow: /about
Allow: /course/
Allow: /user/
Allow: /certificate/

Disallow: /admin/
Disallow: /api/
Disallow: /studio
Disallow: /cabinet
Disallow: /my-courses
Disallow: /auth/
Disallow: /logout

Sitemap: %s/sitemap.xml
`, baseURL)
}

type sitemapURL struct {
	Loc        string `xml:"loc"`
	LastMod    string `xml:"lastmod,omitempty"`
	ChangeFreq string `xml:"changefreq,omitempty"`
	Priority   string `xml:"priority,omitempty"`
}

type sitemapURLSet struct {
	XMLName xml.Name     `xml:"urlset"`
	Xmlns   string       `xml:"xmlns,attr"`
	URLs    []sitemapURL `xml:"url"`
}

func (h *Handler) HandleSitemapXML(w http.ResponseWriter, r *http.Request) {
	base := siteBaseURL()
	today := time.Now().UTC().Format("2006-01-02")

	urls := []sitemapURL{
		{Loc: base + "/", ChangeFreq: "daily", Priority: "1.0", LastMod: today},
		{Loc: base + "/about", ChangeFreq: "monthly", Priority: "0.7", LastMod: today},
	}

	var courses []models.Course
	h.DB.Select("id, updated_at").
		Where("is_published = ? AND admin_status = ?", true, "approved").
		Find(&courses)

	for _, c := range courses {
		urls = append(urls, sitemapURL{
			Loc:        fmt.Sprintf("%s/course/%d/learn", base, c.ID),
			LastMod:    c.UpdatedAt.UTC().Format("2006-01-02"),
			ChangeFreq: "weekly",
			Priority:   "0.9",
		})
	}

	// Individual lesson pages — only open courses or free lessons are crawlable.
	type lessonRow struct {
		LessonID  uint
		CourseID  uint
		UpdatedAt time.Time
	}
	var lessons []lessonRow
	h.DB.Raw(`
		SELECT l.id AS lesson_id, m.course_id AS course_id, l.updated_at
		FROM lessons l
		JOIN modules m ON m.id = l.module_id
		JOIN courses c ON c.id = m.course_id
		WHERE c.is_published = true
		  AND c.admin_status = 'approved'
		  AND (c.is_open = true OR l.is_free = true)
		  AND l.deleted_at IS NULL
		  AND m.deleted_at IS NULL
	`).Scan(&lessons)

	for _, l := range lessons {
		urls = append(urls, sitemapURL{
			Loc:        fmt.Sprintf("%s/course/%d/lesson/%d", base, l.CourseID, l.LessonID),
			LastMod:    l.UpdatedAt.UTC().Format("2006-01-02"),
			ChangeFreq: "monthly",
			Priority:   "0.8",
		})
	}

	var users []models.User
	h.DB.Select("public_id").
		Where("public_id != ''").
		Find(&users)

	for _, u := range users {
		urls = append(urls, sitemapURL{
			Loc:        fmt.Sprintf("%s/user/%s", base, u.PublicID),
			ChangeFreq: "weekly",
			Priority:   "0.5",
		})
	}

	urlset := sitemapURLSet{
		Xmlns: "http://www.sitemaps.org/schemas/sitemap/0.9",
		URLs:  urls,
	}

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Write([]byte(xml.Header))
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	enc.Encode(urlset)
}

func siteBaseURL() string {
	if u := os.Getenv("APP_URL"); u != "" {
		return u
	}
	return "http://localhost:8080"
}

// canonicalURL builds the full canonical URL for the current request path.
func canonicalURL(r *http.Request) string {
	return siteBaseURL() + r.URL.Path
}

// buildJSONLD marshals v to JSON and returns it as a safe template.JS value.
func buildJSONLD(v any) template.JS {
	b, _ := json.Marshal(v)
	return template.JS(b)
}

// JSONLDHome returns Organization + WebSite schema for the home page.
func JSONLDHome() template.JS {
	base := siteBaseURL()
	return buildJSONLD(map[string]any{
		"@context": "https://schema.org",
		"@graph": []any{
			map[string]any{
				"@type": "Organization",
				"name":  "CoursePlatform",
				"url":   base,
			},
			map[string]any{
				"@type": "WebSite",
				"name":  "CoursePlatform",
				"url":   base,
				"potentialAction": map[string]any{
					"@type":       "SearchAction",
					"target":      base + "/?search={search_term_string}",
					"query-input": "required name=search_term_string",
				},
			},
		},
	})
}

// JSONLDCourse returns Course schema for a course page.
func JSONLDCourse(c models.Course, url string) template.JS {
	base := siteBaseURL()
	data := map[string]any{
		"@context":    "https://schema.org",
		"@type":       "Course",
		"name":        c.Title,
		"description": c.Description,
		"url":         url,
		"inLanguage":  c.Language,
		"provider": map[string]any{
			"@type": "Organization",
			"name":  "CoursePlatform",
			"url":   base,
		},
		"offers": map[string]any{
			"@type":         "Offer",
			"price":         "0",
			"priceCurrency": "USD",
			"availability":  "https://schema.org/InStock",
		},
	}
	if c.ImageURL != "" {
		data["image"] = c.ImageURL
	}
	if c.Author.Name != "" {
		data["author"] = map[string]any{
			"@type": "Person",
			"name":  c.Author.Name,
		}
	}
	return buildJSONLD(data)
}

// ExtractLessonDescription pulls plain text from the first text ContentBlock.
func ExtractLessonDescription(blocks []models.ContentBlock, fallback string) string {
	for _, b := range blocks {
		if b.Type != "text" {
			continue
		}
		var d struct {
			Text    string `json:"text"`
			Content string `json:"content"`
		}
		if err := json.Unmarshal(b.Data, &d); err != nil {
			continue
		}
		raw := d.Text
		if raw == "" {
			raw = d.Content
		}
		if raw != "" {
			return truncate(raw, 160)
		}
	}
	return fallback
}

// JSONLDLesson returns LearningResource + BreadcrumbList schema for a lesson page.
func JSONLDLesson(lesson models.Lesson, course models.Course, lessonURL, courseURL string) template.JS {
	base := siteBaseURL()
	desc := ExtractLessonDescription(lesson.ContentBlocks, lesson.Title+" — "+course.Title)

	resource := map[string]any{
		"@context":             "https://schema.org",
		"@type":                "LearningResource",
		"name":                 lesson.Title,
		"description":          desc,
		"url":                  lessonURL,
		"inLanguage":           course.Language,
		"learningResourceType": "lesson",
		"isPartOf": map[string]any{
			"@type": "Course",
			"name":  course.Title,
			"url":   courseURL,
		},
		"provider": map[string]any{
			"@type": "Organization",
			"name":  "CoursePlatform",
			"url":   base,
		},
	}
	if course.Author.Name != "" {
		resource["author"] = map[string]any{
			"@type": "Person",
			"name":  course.Author.Name,
		}
	}
	if course.ImageURL != "" {
		resource["image"] = course.ImageURL
	}

	breadcrumb := map[string]any{
		"@context": "https://schema.org",
		"@type":    "BreadcrumbList",
		"itemListElement": []any{
			map[string]any{"@type": "ListItem", "position": 1, "name": "CoursePlatform", "item": base},
			map[string]any{"@type": "ListItem", "position": 2, "name": course.Title, "item": courseURL},
			map[string]any{"@type": "ListItem", "position": 3, "name": lesson.Title, "item": lessonURL},
		},
	}

	return buildJSONLD([]any{resource, breadcrumb})
}

// JSONLDPerson returns Person schema for a public user profile page.
func JSONLDPerson(u models.User, profileURL string) template.JS {
	data := map[string]any{
		"@context": "https://schema.org",
		"@type":    "Person",
		"name":     u.Name,
		"url":      profileURL,
	}
	if u.Picture != "" {
		data["image"] = u.Picture
	}
	return buildJSONLD(data)
}
