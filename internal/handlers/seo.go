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

// JSONLDLesson returns LearningResource schema for a lesson page.
func JSONLDLesson(lesson models.Lesson, course models.Course, lessonURL, courseURL string) template.JS {
	base := siteBaseURL()
	return buildJSONLD(map[string]any{
		"@context":   "https://schema.org",
		"@type":      "LearningResource",
		"name":       lesson.Title,
		"url":        lessonURL,
		"inLanguage": course.Language,
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
	})
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
