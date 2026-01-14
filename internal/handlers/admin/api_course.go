package admin

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"reflect"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/s/onlineCourse/internal/models"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// ==========================================
// 1. GET /api/courses (Список)
// 2. POST /api/courses (Создание)
// ==========================================
func (s *Service) HandleCoursesAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		s.getCourses(w, r)
	case http.MethodPost:
		s.createCourse(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// ==========================================
// 3. GET /api/courses/{id} (Детали)
// 4. PUT /api/courses/{id} (Обновление)
// 5. DELETE /api/courses/{id} (Удаление)
// ==========================================
func (s *Service) HandleCourseByIDAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Получаем ID из URL с помощью Gorilla Mux
	vars := mux.Vars(r)
	idStr := vars["id"]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		jsonError(w, "Invalid course ID", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.getCourseByID(w, r, id)
	case http.MethodPut:
		s.updateCourse(w, r, id)
	case http.MethodDelete:
		s.deleteCourse(w, r, id)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// -------------------------------------------------------------------------
// Вспомогательные функции (Логика)
// -------------------------------------------------------------------------

func (s *Service) getCourses(w http.ResponseWriter, r *http.Request) {
	var courses []models.Course

	session, err := s.Store.Get(r, "session")
	if err != nil {
		response := map[string]string{
			"error": "Вы не авторизованы",
			"code":  "401",
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	userIDvalue := session.Values["user_id"]
	userID, ok := userIDvalue.(uint)
	if !ok || userID == 0 {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	result := s.DB.Where("author_id = ?", userID).Preload("Author").Order("created_at desc").Find(&courses)
	if result.Error != nil {
		jsonError(w, "Database error", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(courses)
}

func (s *Service) createCourse(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		IsPublished bool   `json:"is_published"`
		Language    string `json:"language"`
		ImageURL    string `json:"image_url"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		jsonError(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	if input.Title == "" {
		jsonError(w, "Title is required", http.StatusBadRequest)
		return
	}

	session, err := s.Store.Get(r, "session")
	if err != nil {
		log.Println("Ошибка получения сессии:", err)
		jsonError(w, "Session error", http.StatusInternalServerError)
		return
	}

	val := session.Values["user_id"]
	if val == nil {
		jsonError(w, "User not identified (Empty Session)", http.StatusUnauthorized)
		return
	}

	var userID uint
	if v, ok := val.(uint); ok {
		userID = v
	} else if v, ok := val.(int); ok {
		userID = uint(v)
	} else if v, ok := val.(float64); ok {
		userID = uint(v)
	} else {
		jsonError(w, "User ID type mismatch", http.StatusUnauthorized)
		return
	}

	course := models.Course{
		Title:       input.Title,
		Description: input.Description,
		IsPublished: input.IsPublished,
		Language:    input.Language,
		ImageURL:    input.ImageURL,
		AuthorID:    userID,
	}

	if err := s.DB.Create(&course).Error; err != nil {
		log.Println("Ошибка БД при создании курса:", err)
		jsonError(w, "Failed to create course", http.StatusInternalServerError)
		return
	}

	s.DB.Preload("Author").First(&course, course.ID)

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(course)
}

func (s *Service) getCourseByID(w http.ResponseWriter, r *http.Request, id int) {
	var course models.Course

	if err := s.DB.Preload("Author").Preload("Modules.Lessons").First(&course, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			jsonError(w, "Course not found", http.StatusNotFound)
		} else {
			jsonError(w, "Database error", http.StatusInternalServerError)
		}
		return
	}

	json.NewEncoder(w).Encode(course)
}

func (s *Service) updateCourse(w http.ResponseWriter, r *http.Request, id int) {
	var course models.Course

	if err := s.DB.First(&course, id).Error; err != nil {
		jsonError(w, "Course not found", http.StatusNotFound)
		return
	}

	var input struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		IsPublished bool   `json:"is_published"`
		Language    string `json:"language"`
		ImageURL    string `json:"image_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		jsonError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	course.Title = input.Title
	course.Description = input.Description
	course.IsPublished = input.IsPublished
	course.Language = input.Language
	course.ImageURL = input.ImageURL

	if err := s.DB.Save(&course).Error; err != nil {
		jsonError(w, "Failed to update course", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(course)
}

func (s *Service) deleteCourse(w http.ResponseWriter, r *http.Request, id int) {
	result := s.DB.Delete(&models.Course{}, id)

	if result.Error != nil {
		jsonError(w, "Database error", http.StatusInternalServerError)
		return
	}

	if result.RowsAffected == 0 {
		jsonError(w, "Course not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Course deleted successfully"})
}

func jsonError(w http.ResponseWriter, message string, code int) {
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

// =======================
// MODULES API
// =======================

func (s *Service) CreateModuleAPI(w http.ResponseWriter, r *http.Request) {
	var input struct {
		CourseID uint   `json:"course_id"`
		Title    string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		jsonError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	module := models.Module{
		CourseID: input.CourseID,
		Title:    input.Title,
	}

	if err := s.DB.Create(&module).Error; err != nil {
		jsonError(w, "Failed to create module", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(module)
}

func (s *Service) UpdateModuleAPI(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.Atoi(vars["id"])

	var input struct {
		Title string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		jsonError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if err := s.DB.Model(&models.Module{}).Where("id = ?", id).Update("title", input.Title).Error; err != nil {
		jsonError(w, "Failed to update module", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Service) DeleteModuleAPI(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.Atoi(vars["id"])

	if err := s.DB.Delete(&models.Module{}, id).Error; err != nil {
		jsonError(w, "Failed to delete module", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}

// =======================
// LESSONS API
// =======================

func (s *Service) CreateLessonAPI(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ModuleID uint   `json:"module_id"`
		Title    string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		jsonError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	lesson := models.Lesson{
		ModuleID: input.ModuleID,
		Title:    input.Title,
	}

	if err := s.DB.Create(&lesson).Error; err != nil {
		jsonError(w, "Failed to create lesson", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(lesson)
}

func (s *Service) UpdateLessonAPI(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.Atoi(vars["id"])

	var input map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		jsonError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if err := s.DB.Model(&models.Lesson{}).Where("id = ?", id).Updates(input).Error; err != nil {
		jsonError(w, "Failed to update lesson", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Service) DeleteLessonAPI(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.Atoi(vars["id"])

	if err := s.DB.Delete(&models.Lesson{}, id).Error; err != nil {
		jsonError(w, "Failed to delete lesson", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}

// PUT /api/lessons/{id}/content
func (s *Service) UpdateLessonContentAPI(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	lessonID, _ := strconv.Atoi(vars["id"])

	var req struct {
		Blocks []struct {
			ID   uint           `json:"id"`
			Type string         `json:"type"`
			Data datatypes.JSON `json:"data"`
		} `json:"blocks"`
		ForceReset bool `json:"force_reset"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	tx := s.DB.Begin()

	var existingBlocks []models.ContentBlock
	if err := tx.Where("lesson_id = ?", lessonID).Find(&existingBlocks).Error; err != nil {
		tx.Rollback()
		jsonError(w, "Failed to fetch existing blocks", http.StatusInternalServerError)
		return
	}

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
		for id, _ := range existingMap {
			if !incomingIDs[id] {
				var count int64
				tx.Model(&models.QuizAttempt{}).Where("block_id = ?", id).Count(&count)
				if count > 0 {
					tx.Rollback()
					jsonError(w, "BLOCK_HAS_ANSWERS", http.StatusConflict)
					return
				}
			}
		}

		for _, input := range req.Blocks {
			if input.ID > 0 {
				existing, exists := existingMap[input.ID]
				if exists {
					typeChanged := input.Type != existing.Type
					dataChanged := !areJSONsEqual(input.Data, existing.Data)

					if typeChanged || dataChanged {
						var count int64
						tx.Model(&models.QuizAttempt{}).Where("block_id = ?", input.ID).Count(&count)
						if count > 0 {
							tx.Rollback()
							jsonError(w, "BLOCK_HAS_ANSWERS", http.StatusConflict)
							return
						}
					}
				}
			}
		}
	}

	for id := range existingMap {
		if !incomingIDs[id] {
			if err := tx.Where("block_id = ?", id).Delete(&models.QuizAttempt{}).Error; err != nil {
				tx.Rollback()
				jsonError(w, "Failed to delete attempts", http.StatusInternalServerError)
				return
			}
			if err := tx.Delete(&models.ContentBlock{}, id).Error; err != nil {
				tx.Rollback()
				jsonError(w, "Failed to delete block", http.StatusInternalServerError)
				return
			}
		}
	}

	for i, block := range req.Blocks {
		if block.ID > 0 {
			if req.ForceReset {
				existing, exists := existingMap[block.ID]
				if exists && (block.Type != existing.Type || !areJSONsEqual(existing.Data, block.Data)) {
					tx.Where("block_id = ?", block.ID).Delete(&models.QuizAttempt{})
				}
			}

			if err := tx.Model(&models.ContentBlock{}).Where("id = ?", block.ID).Updates(map[string]interface{}{
				"type":  block.Type,
				"data":  block.Data,
				"order": i,
			}).Error; err != nil {
				tx.Rollback()
				jsonError(w, "Failed to update block", http.StatusInternalServerError)
				return
			}
		} else {
			newBlock := models.ContentBlock{
				LessonID: uint(lessonID),
				Type:     block.Type,
				Order:    i,
				Data:     block.Data,
			}
			if err := tx.Create(&newBlock).Error; err != nil {
				tx.Rollback()
				jsonError(w, "Failed to create block", http.StatusInternalServerError)
				return
			}
		}
	}

	tx.Commit()

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "saved"})
}

func areJSONsEqual(a, b []byte) bool {
	var objA, objB interface{}

	if len(a) == 0 && len(b) == 0 {
		return true
	}

	if err := json.Unmarshal(a, &objA); err != nil {
		return bytes.Equal(a, b)
	}
	if err := json.Unmarshal(b, &objB); err != nil {
		return bytes.Equal(a, b)
	}
	return reflect.DeepEqual(objA, objB)
}

// GET /api/lessons/{id}
func (s *Service) GetLessonAPI(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.Atoi(vars["id"])

	var lesson models.Lesson

	err := s.DB.Preload("ContentBlocks", func(db *gorm.DB) *gorm.DB {
		return db.Order("content_blocks.order ASC")
	}).First(&lesson, id).Error

	if err != nil {
		jsonError(w, "Lesson not found", http.StatusNotFound)
		return
	}

	json.NewEncoder(w).Encode(lesson)
}

// Структура ответа
type CourseStructureResponse struct {
	Course        models.Course `json:"course"`
	IsAuth        bool          `json:"is_auth"`
	RequestStatus string        `json:"request_status"`
}

func (s *Service) GetCourseStructure(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.Atoi(vars["id"])

	var course models.Course
	err := s.DB.Preload("Modules").Preload("Modules.Lessons").First(&course, id).Error

	if err != nil {
		jsonError(w, "Курс не найден", http.StatusNotFound)
		return
	}

	resp := CourseStructureResponse{
		Course:        course,
		IsAuth:        false,
		RequestStatus: "",
	}

	session, _ := s.Store.Get(r, "session")
	val := session.Values["user_id"]

	if val != nil {
		var userID uint
		var isValidUser bool = false

		if v, ok := val.(uint); ok {
			userID = v
			isValidUser = true
		} else if v, ok := val.(int); ok {
			userID = uint(v)
			isValidUser = true
		} else if v, ok := val.(float64); ok {
			userID = uint(v)
			isValidUser = true
		}

		if isValidUser {
			resp.IsAuth = true
			var enrollment models.Enrollment
			if err := s.DB.Where("user_id = ? AND course_id = ?", userID, course.ID).First(&enrollment).Error; err == nil {
				resp.RequestStatus = enrollment.Status
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Service) SubmitEnrollment(w http.ResponseWriter, r *http.Request) {
	session, err := s.Store.Get(r, "session")
	if err != nil {
		jsonError(w, "Session error", http.StatusInternalServerError)
		return
	}

	val := session.Values["user_id"]
	if val == nil {
		jsonError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var userID uint
	if v, ok := val.(uint); ok {
		userID = v
	} else if v, ok := val.(int); ok {
		userID = uint(v)
	} else if v, ok := val.(float64); ok {
		userID = uint(v)
	} else {
		jsonError(w, "User ID type mismatch", http.StatusUnauthorized)
		return
	}

	var req struct {
		CourseID uint `json:"course_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	var existing models.Enrollment
	if err := s.DB.Where("user_id = ? AND course_id = ?", userID, req.CourseID).First(&existing).Error; err == nil {
		jsonError(w, "Заявка уже существует", http.StatusConflict)
		return
	}

	enrollment := models.Enrollment{
		UserID:   userID,
		CourseID: req.CourseID,
		Status:   "pending",
	}

	if err := s.DB.Create(&enrollment).Error; err != nil {
		jsonError(w, "Database error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":         "success",
		"request_status": "pending",
	})
}

// Compact JSON helper
func compactJSON(data []byte) string {
	dst := &bytes.Buffer{}
	if err := json.Compact(dst, data); err != nil {
		return string(data)
	}
	return dst.String()
}
