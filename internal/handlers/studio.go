package handlers

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"reflect"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/s/onlineCourse/internal/i18n"
	"github.com/s/onlineCourse/internal/models"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// HandleStudioPage renders the user's course studio page.
func (h *Handler) HandleStudioPage(w http.ResponseWriter, r *http.Request) {
	roleID, userID := h.GetUserRoleID(r)
	if userID == 0 {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	session, _ := h.Store.Get(r, "session")
	lang := h.DetectLang(r)

	data := PageData{
		Title:          i18n.T(lang, "studio.title"),
		IsAuthenticated: true,
		UserID:          userID,
		RoleID:          roleID,
		UserName:        toString(session.Values["name"]),
		UserPictureURL:  toString(session.Values["picture_url"]),
		Email:           toString(session.Values["email"]),
		CurrentPath:     r.URL.Path,
		Lang:            lang,
		TransJSON:       BuildTransJSON(lang),
	}

	if err := h.Tmpl.ExecuteTemplate(w, "studio.html", data); err != nil {
		log.Printf("HandleStudioPage: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// ─────────────────────────────────────────────
// STUDIO COURSE APIs  (author-scoped)
// ─────────────────────────────────────────────

// GET  /api/studio/courses
func (h *Handler) StudioGetCoursesAPI(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.GetAuthenticatedUserID(r)
	if !ok {
		studioJSONError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	var courses []models.Course
	if err := h.DB.Preload("Author").Where("author_id = ?", userID).
		Order("created_at desc").Find(&courses).Error; err != nil {
		studioJSONError(w, "Database error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(courses)
}

// POST /api/studio/courses
func (h *Handler) StudioCreateCourseAPI(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.GetAuthenticatedUserID(r)
	if !ok {
		studioJSONError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var input struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		IsOpen      bool   `json:"is_open"`
		Language    string `json:"language"`
		ImageURL    string `json:"image_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		studioJSONError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	if input.Title == "" {
		studioJSONError(w, "Title is required", http.StatusBadRequest)
		return
	}

	course := models.Course{
		Title:       input.Title,
		Description: input.Description,
		IsOpen:      input.IsOpen,
		Language:    input.Language,
		ImageURL:    input.ImageURL,
		AuthorID:    userID,
		AdminStatus: "draft",
		IsPublished: false,
	}
	if err := h.DB.Create(&course).Error; err != nil {
		studioJSONError(w, "Failed to create course", http.StatusInternalServerError)
		return
	}
	h.DB.Preload("Author").First(&course, course.ID)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(course)
}

// PUT /api/studio/courses/{id}
func (h *Handler) StudioUpdateCourseAPI(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.GetAuthenticatedUserID(r)
	if !ok {
		studioJSONError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	id, _ := strconv.Atoi(mux.Vars(r)["id"])

	var course models.Course
	if err := h.DB.First(&course, id).Error; err != nil {
		studioJSONError(w, "Course not found", http.StatusNotFound)
		return
	}
	if course.AuthorID != userID {
		studioJSONError(w, "Forbidden", http.StatusForbidden)
		return
	}
	if course.AdminStatus == "pending_review" {
		studioJSONError(w, "Cannot edit a course that is pending review", http.StatusConflict)
		return
	}

	var input struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		IsOpen      bool   `json:"is_open"`
		Language    string `json:"language"`
		ImageURL    string `json:"image_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		studioJSONError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	course.Title = input.Title
	course.Description = input.Description
	course.IsOpen = input.IsOpen
	course.Language = input.Language
	course.ImageURL = input.ImageURL
	// Editing after rejection resets to draft
	if course.AdminStatus == "rejected" {
		course.AdminStatus = "draft"
		course.ReviewNote = ""
	}

	if err := h.DB.Save(&course).Error; err != nil {
		studioJSONError(w, "Failed to update course", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(course)
}

// DELETE /api/studio/courses/{id}
func (h *Handler) StudioDeleteCourseAPI(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.GetAuthenticatedUserID(r)
	if !ok {
		studioJSONError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	id, _ := strconv.Atoi(mux.Vars(r)["id"])

	var course models.Course
	if err := h.DB.First(&course, id).Error; err != nil {
		studioJSONError(w, "Course not found", http.StatusNotFound)
		return
	}
	if course.AuthorID != userID {
		studioJSONError(w, "Forbidden", http.StatusForbidden)
		return
	}
	if course.AdminStatus == "approved" {
		studioJSONError(w, "Cannot delete an approved course", http.StatusConflict)
		return
	}

	if err := h.DB.Delete(&models.Course{}, id).Error; err != nil {
		studioJSONError(w, "Failed to delete", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "deleted"})
}

// POST /api/studio/courses/{id}/submit
func (h *Handler) StudioSubmitCourseAPI(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.GetAuthenticatedUserID(r)
	if !ok {
		studioJSONError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	id, _ := strconv.Atoi(mux.Vars(r)["id"])

	var course models.Course
	if err := h.DB.First(&course, id).Error; err != nil {
		studioJSONError(w, "Course not found", http.StatusNotFound)
		return
	}
	if course.AuthorID != userID {
		studioJSONError(w, "Forbidden", http.StatusForbidden)
		return
	}
	if course.AdminStatus != "draft" && course.AdminStatus != "rejected" {
		studioJSONError(w, "Course is already submitted or approved", http.StatusConflict)
		return
	}

	// Count lessons — require at least 1 before submitting
	var lessonCount int64
	h.DB.Model(&models.Lesson{}).
		Joins("JOIN modules ON modules.id = lessons.module_id").
		Where("modules.course_id = ?", id).
		Count(&lessonCount)
	if lessonCount == 0 {
		studioJSONError(w, "Add at least one lesson before submitting", http.StatusBadRequest)
		return
	}

	if err := h.DB.Model(&course).Updates(map[string]interface{}{
		"admin_status": "pending_review",
		"review_note":  "",
	}).Error; err != nil {
		studioJSONError(w, "Failed to submit", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"admin_status": "pending_review"})
}

// ─────────────────────────────────────────────
// STUDIO MODULE APIs
// ─────────────────────────────────────────────

// POST /api/studio/modules
func (h *Handler) StudioCreateModuleAPI(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.GetAuthenticatedUserID(r)
	if !ok {
		studioJSONError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var input struct {
		CourseID uint   `json:"course_id"`
		Title    string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		studioJSONError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	if !h.studioIsAuthor(userID, input.CourseID) {
		studioJSONError(w, "Forbidden", http.StatusForbidden)
		return
	}

	module := models.Module{CourseID: input.CourseID, Title: input.Title}
	if err := h.DB.Create(&module).Error; err != nil {
		studioJSONError(w, "Failed to create module", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(module)
}

// PUT /api/studio/modules/{id}
func (h *Handler) StudioUpdateModuleAPI(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.GetAuthenticatedUserID(r)
	if !ok {
		studioJSONError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	id, _ := strconv.Atoi(mux.Vars(r)["id"])

	var module models.Module
	if err := h.DB.First(&module, id).Error; err != nil {
		studioJSONError(w, "Module not found", http.StatusNotFound)
		return
	}
	if !h.studioIsAuthor(userID, module.CourseID) {
		studioJSONError(w, "Forbidden", http.StatusForbidden)
		return
	}

	var input struct{ Title string `json:"title"` }
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		studioJSONError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	h.DB.Model(&models.Module{}).Where("id = ?", id).Update("title", input.Title)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// DELETE /api/studio/modules/{id}
func (h *Handler) StudioDeleteModuleAPI(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.GetAuthenticatedUserID(r)
	if !ok {
		studioJSONError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	id, _ := strconv.Atoi(mux.Vars(r)["id"])

	var module models.Module
	if err := h.DB.First(&module, id).Error; err != nil {
		studioJSONError(w, "Module not found", http.StatusNotFound)
		return
	}
	if !h.studioIsAuthor(userID, module.CourseID) {
		studioJSONError(w, "Forbidden", http.StatusForbidden)
		return
	}
	h.DB.Delete(&models.Module{}, id)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}

// ─────────────────────────────────────────────
// STUDIO LESSON APIs
// ─────────────────────────────────────────────

// POST /api/studio/lessons
func (h *Handler) StudioCreateLessonAPI(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.GetAuthenticatedUserID(r)
	if !ok {
		studioJSONError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var input struct {
		ModuleID uint   `json:"module_id"`
		Title    string `json:"title"`
		IsFree   bool   `json:"is_free"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		studioJSONError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	if !h.studioIsAuthorByModule(userID, input.ModuleID) {
		studioJSONError(w, "Forbidden", http.StatusForbidden)
		return
	}

	lesson := models.Lesson{ModuleID: input.ModuleID, Title: input.Title, IsFree: input.IsFree}
	if err := h.DB.Create(&lesson).Error; err != nil {
		studioJSONError(w, "Failed to create lesson", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(lesson)
}

// PUT /api/studio/lessons/{id}
func (h *Handler) StudioUpdateLessonAPI(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.GetAuthenticatedUserID(r)
	if !ok {
		studioJSONError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	id, _ := strconv.Atoi(mux.Vars(r)["id"])

	var lesson models.Lesson
	if err := h.DB.First(&lesson, id).Error; err != nil {
		studioJSONError(w, "Lesson not found", http.StatusNotFound)
		return
	}
	if !h.studioIsAuthorByModule(userID, lesson.ModuleID) {
		studioJSONError(w, "Forbidden", http.StatusForbidden)
		return
	}

	var input map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		studioJSONError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	h.DB.Model(&models.Lesson{}).Where("id = ?", id).Updates(input)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// DELETE /api/studio/lessons/{id}
func (h *Handler) StudioDeleteLessonAPI(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.GetAuthenticatedUserID(r)
	if !ok {
		studioJSONError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	id, _ := strconv.Atoi(mux.Vars(r)["id"])

	var lesson models.Lesson
	if err := h.DB.First(&lesson, id).Error; err != nil {
		studioJSONError(w, "Lesson not found", http.StatusNotFound)
		return
	}
	if !h.studioIsAuthorByModule(userID, lesson.ModuleID) {
		studioJSONError(w, "Forbidden", http.StatusForbidden)
		return
	}
	h.DB.Delete(&models.Lesson{}, id)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}

// GET /api/studio/lessons/{id}
func (h *Handler) StudioGetLessonAPI(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.GetAuthenticatedUserID(r)
	if !ok {
		studioJSONError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	id, _ := strconv.Atoi(mux.Vars(r)["id"])

	var lesson models.Lesson
	if err := h.DB.Preload("ContentBlocks", func(db *gorm.DB) *gorm.DB {
		return db.Order("content_blocks.order ASC")
	}).First(&lesson, id).Error; err != nil {
		studioJSONError(w, "Lesson not found", http.StatusNotFound)
		return
	}
	if !h.studioIsAuthorByModule(userID, lesson.ModuleID) {
		studioJSONError(w, "Forbidden", http.StatusForbidden)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(lesson)
}

// PUT /api/studio/lessons/{id}/content
func (h *Handler) StudioUpdateLessonContentAPI(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.GetAuthenticatedUserID(r)
	if !ok {
		studioJSONError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	lessonID, _ := strconv.Atoi(mux.Vars(r)["id"])

	var lesson models.Lesson
	if err := h.DB.First(&lesson, lessonID).Error; err != nil {
		studioJSONError(w, "Lesson not found", http.StatusNotFound)
		return
	}
	if !h.studioIsAuthorByModule(userID, lesson.ModuleID) {
		studioJSONError(w, "Forbidden", http.StatusForbidden)
		return
	}

	var req struct {
		Blocks []struct {
			ID   uint           `json:"id"`
			Type string         `json:"type"`
			Data datatypes.JSON `json:"data"`
		} `json:"blocks"`
		ForceReset bool `json:"force_reset"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		studioJSONError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	tx := h.DB.Begin()

	var existingBlocks []models.ContentBlock
	tx.Where("lesson_id = ?", lessonID).Find(&existingBlocks)

	existingMap := make(map[uint]models.ContentBlock)
	for _, b := range existingBlocks {
		existingMap[b.ID] = b
	}
	incomingIDs := make(map[uint]bool)
	for _, b := range req.Blocks {
		if b.ID > 0 {
			incomingIDs[b.ID] = true
		}
	}

	if !req.ForceReset {
		for id := range existingMap {
			if !incomingIDs[id] {
				var count int64
				tx.Model(&models.QuizAttempt{}).Where("block_id = ?", id).Count(&count)
				if count > 0 {
					tx.Rollback()
					studioJSONError(w, "BLOCK_HAS_ANSWERS", http.StatusConflict)
					return
				}
			}
		}
		for _, input := range req.Blocks {
			if input.ID > 0 {
				if existing, ok := existingMap[input.ID]; ok {
					if input.Type != existing.Type || !studioAreJSONsEqual(input.Data, existing.Data) {
						var count int64
						tx.Model(&models.QuizAttempt{}).Where("block_id = ?", input.ID).Count(&count)
						if count > 0 {
							tx.Rollback()
							studioJSONError(w, "BLOCK_HAS_ANSWERS", http.StatusConflict)
							return
						}
					}
				}
			}
		}
	}

	for id := range existingMap {
		if !incomingIDs[id] {
			tx.Where("block_id = ?", id).Delete(&models.QuizAttempt{})
			tx.Delete(&models.ContentBlock{}, id)
		}
	}

	for i, block := range req.Blocks {
		if block.ID > 0 {
			if req.ForceReset {
				if existing, ok := existingMap[block.ID]; ok {
					if block.Type != existing.Type || !studioAreJSONsEqual(existing.Data, block.Data) {
						tx.Where("block_id = ?", block.ID).Delete(&models.QuizAttempt{})
					}
				}
			}
			tx.Model(&models.ContentBlock{}).Where("id = ?", block.ID).Updates(map[string]interface{}{
				"type":  block.Type,
				"data":  block.Data,
				"order": i,
			})
		} else {
			tx.Create(&models.ContentBlock{
				LessonID: uint(lessonID),
				Type:     block.Type,
				Order:    i,
				Data:     block.Data,
			})
		}
	}

	tx.Commit()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "saved"})
}

// GET /api/studio/courses/{id}/structure
func (h *Handler) StudioGetCourseStructureAPI(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.GetAuthenticatedUserID(r)
	if !ok {
		studioJSONError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	id, _ := strconv.Atoi(mux.Vars(r)["id"])

	var course models.Course
	if err := h.DB.Preload("Modules.Lessons").First(&course, id).Error; err != nil {
		studioJSONError(w, "Course not found", http.StatusNotFound)
		return
	}
	if course.AuthorID != userID {
		studioJSONError(w, "Forbidden", http.StatusForbidden)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(course)
}

// ─────────────────────────────────────────────
// helpers
// ─────────────────────────────────────────────

func (h *Handler) studioIsAuthor(userID uint, courseID uint) bool {
	var course models.Course
	return h.DB.Select("author_id").First(&course, courseID).Error == nil &&
		course.AuthorID == userID
}

func (h *Handler) studioIsAuthorByModule(userID uint, moduleID uint) bool {
	var module models.Module
	if h.DB.Select("course_id").First(&module, moduleID).Error != nil {
		return false
	}
	return h.studioIsAuthor(userID, module.CourseID)
}

func studioJSONError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func studioAreJSONsEqual(a, b []byte) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	var objA, objB interface{}
	if err := json.Unmarshal(a, &objA); err != nil {
		return bytes.Equal(a, b)
	}
	if err := json.Unmarshal(b, &objB); err != nil {
		return bytes.Equal(a, b)
	}
	return reflect.DeepEqual(objA, objB)
}
