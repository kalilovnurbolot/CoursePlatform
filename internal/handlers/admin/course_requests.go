package admin

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/s/onlineCourse/internal/handlers"
	"github.com/s/onlineCourse/internal/models"
)

// HandleCourseRequestsPage renders the admin course-approval page.
func (s *Service) HandleCourseRequestsPage(w http.ResponseWriter, r *http.Request) {
	roleID, userID := s.GetUserRoleID(r)
	session, _ := s.Store.Get(r, "session")
	lang := s.DetectLang(r)

	data := handlers.PageData{
		Title:           "Course Requests",
		IsAuthenticated: userID != 0,
		UserID:          userID,
		RoleID:          roleID,
		UserName:        toString(session.Values["name"]),
		UserPictureURL:  toString(session.Values["picture_url"]),
		CurrentPath:     r.URL.Path,
		Lang:            lang,
		TransJSON:       handlers.BuildTransJSON(lang),
	}

	s.Tmpl.ExecuteTemplate(w, "course_requests.html", data)
}

// GetCourseRequestsAPI returns courses with admin_status = pending_review.
// GET /api/admin/course-requests
func (s *Service) GetCourseRequestsAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var courses []models.Course
	if err := s.DB.Preload("Author").Preload("Modules.Lessons").
		Where("admin_status = ?", "pending_review").
		Order("updated_at desc").
		Find(&courses).Error; err != nil {
		jsonError(w, "Database error", http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(courses)
}

// ReviewCourseRequestAPI approves or rejects a pending course.
// PUT /api/admin/course-requests/{id}
func (s *Service) ReviewCourseRequestAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	id, _ := strconv.Atoi(mux.Vars(r)["id"])

	var req struct {
		Action     string `json:"action"`      // "approve" | "reject"
		ReviewNote string `json:"review_note"` // filled when rejecting
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	var course models.Course
	if err := s.DB.First(&course, id).Error; err != nil {
		jsonError(w, "Course not found", http.StatusNotFound)
		return
	}
	if course.AdminStatus != "pending_review" {
		jsonError(w, "Course is not pending review", http.StatusConflict)
		return
	}

	updates := map[string]interface{}{"review_note": ""}
	switch req.Action {
	case "approve":
		updates["admin_status"] = "approved"
	case "reject":
		if req.ReviewNote == "" {
			jsonError(w, "review_note is required when rejecting", http.StatusBadRequest)
			return
		}
		updates["admin_status"] = "rejected"
		updates["review_note"] = req.ReviewNote
	default:
		jsonError(w, "action must be 'approve' or 'reject'", http.StatusBadRequest)
		return
	}

	if err := s.DB.Model(&models.Course{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		jsonError(w, "Failed to update course", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":           id,
		"admin_status": updates["admin_status"],
	})
}

// GetAdminAllCoursesAPI returns all courses for the admin panel (all statuses, all authors).
// GET /api/admin/courses-all
func (s *Service) GetAdminAllCoursesAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var courses []models.Course
	if err := s.DB.Preload("Author").Order("created_at desc").Find(&courses).Error; err != nil {
		jsonError(w, "Database error", http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(courses)
}
