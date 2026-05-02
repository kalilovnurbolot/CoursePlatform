package handlers

import (
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/s/onlineCourse/internal/i18n"
	"github.com/s/onlineCourse/internal/models"
)

// HandleUserProfilePage renders /user/{public_id} — public profile showing the user's approved courses.
func (h *Handler) HandleUserProfilePage(w http.ResponseWriter, r *http.Request) {
	publicID := mux.Vars(r)["public_id"]
	if publicID == "" {
		http.NotFound(w, r)
		return
	}

	var profileUser models.User
	if err := h.DB.Where("public_id = ?", publicID).First(&profileUser).Error; err != nil {
		http.NotFound(w, r)
		return
	}

	// Load this user's approved + published courses
	var courses []models.Course
	h.DB.Preload("Author").Preload("Modules.Lessons").
		Where("author_id = ? AND admin_status = ? AND is_published = ?", profileUser.ID, "approved", true).
		Order("created_at desc").
		Find(&courses)

	// For the current viewer, check enrollment status per course
	roleID, viewerID := h.GetUserRoleID(r)
	session, _ := h.Store.Get(r, "session")
	lang := h.DetectLang(r)

	var enrollmentMap map[uint]string
	if viewerID != 0 {
		var enrollments []models.Enrollment
		h.DB.Where("user_id = ?", viewerID).Find(&enrollments)
		enrollmentMap = make(map[uint]string, len(enrollments))
		for _, e := range enrollments {
			enrollmentMap[e.CourseID] = e.Status
		}
	}

	profileCourses := make([]ProfileCourseView, 0, len(courses))
	for _, c := range courses {
		total := 0
		for _, m := range c.Modules {
			total += len(m.Lessons)
		}
		profileCourses = append(profileCourses, ProfileCourseView{
			Course:           c,
			LessonCount:      total,
			EnrollmentStatus: enrollmentMap[c.ID],
		})
	}

	// Count authored courses for stats
	var authoredCount int64
	h.DB.Model(&models.Course{}).
		Where("author_id = ? AND admin_status = ? AND is_published = ?", profileUser.ID, "approved", true).
		Count(&authoredCount)

	profileURL := canonicalURL(r)
	data := PageData{
		Title:           profileUser.Name,
		Description:     truncate(profileUser.Name+" — "+i18n.T(lang, "userprofile.author_label")+" | CoursePlatform", 160),
		CanonicalURL:    profileURL,
		JSONLD:          JSONLDPerson(profileUser, profileURL),
		IsAuthenticated: viewerID != 0,
		UserID:          viewerID,
		RoleID:          roleID,
		UserName:        toString(session.Values["name"]),
		UserPictureURL:  toString(session.Values["picture_url"]),
		Lang:            lang,
		TransJSON:       BuildTransJSON(lang),
		ProfileUser:     &profileUser,
		ProfileCourses:  profileCourses,
	}
	_ = i18n.T // used via T func in templates
	_ = authoredCount

	if err := h.Tmpl.ExecuteTemplate(w, "user_profile.html", data); err != nil {
		log.Printf("HandleUserProfilePage: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// ProfileCourseView wraps a course with viewer-context data.
type ProfileCourseView struct {
	Course           models.Course
	LessonCount      int
	EnrollmentStatus string // pending | approved | rejected | ""
}
