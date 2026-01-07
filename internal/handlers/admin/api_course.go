package admin

import (
	"encoding/json"
	"log"
	"net/http"
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
		// Логируем ошибку и, возможно, перенаправляем на страницу входа или показываем ошибку сервера.
		// В продакшене тут лучше логгировать 'err'.
		response := map[string]string{
			"error": "Вы не авторизованы",
			"code":  "401",
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	userIDvalue := session.Values["user_id"]
	userID, ok := userIDvalue.(uint)
	if !ok || userID == 0 { // Добавим проверку, что userID не равен 0 для неаутентифицированных пользователей
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	// Preload("Author") важен, чтобы на фронте отобразилось имя создателя
	result := s.DB.Where("author_id = ?", userID).Preload("Author").Order("created_at desc").Find(&courses)
	if result.Error != nil {
		jsonError(w, "Database error", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(courses)
}

func (s *Service) createCourse(w http.ResponseWriter, r *http.Request) {
	// Структура для парсинга входящего JSON
	var input struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		IsPublished bool   `json:"is_published"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		jsonError(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	if input.Title == "" {
		jsonError(w, "Title is required", http.StatusBadRequest)
		return
	}

	// === ИСПРАВЛЕНИЕ ЗДЕСЬ ===
	// 1. Получаем сессию. Имя должно быть "session" (как в main.go/auth.go)
	session, err := s.Store.Get(r, "session")
	if err != nil {
		log.Println("Ошибка получения сессии:", err)
		jsonError(w, "Session error", http.StatusInternalServerError)
		return
	}

	// 2. Ищем ключ "user_id" (как в admin.go)
	val := session.Values["user_id"]
	if val == nil {
		log.Println("Ошибка: user_id не найден в сессии")
		jsonError(w, "User not identified (Empty Session)", http.StatusUnauthorized)
		return
	}

	// 3. Безопасное приведение типа (иногда GORM сохраняет как int, иногда uint)
	var userID uint
	if v, ok := val.(uint); ok {
		userID = v
	} else if v, ok := val.(int); ok {
		userID = uint(v)
	} else if v, ok := val.(float64); ok { // Иногда JSON числа
		userID = uint(v)
	} else {
		log.Printf("Ошибка типа user_id: %T %v", val, val)
		jsonError(w, "User ID type mismatch", http.StatusUnauthorized)
		return
	}
	// =========================

	course := models.Course{
		Title:       input.Title,
		Description: input.Description,
		IsPublished: input.IsPublished,
		AuthorID:    userID, // Привязываем к админу
	}

	if err := s.DB.Create(&course).Error; err != nil {
		log.Println("Ошибка БД при создании курса:", err)
		jsonError(w, "Failed to create course", http.StatusInternalServerError)
		return
	}

	// Подгружаем автора для ответа, чтобы фронт сразу мог отрисовать карточку красиво
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

	// 1. Проверяем, существует ли курс
	if err := s.DB.First(&course, id).Error; err != nil {
		jsonError(w, "Course not found", http.StatusNotFound)
		return
	}

	// 2. Парсим данные для обновления
	var input struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		IsPublished bool   `json:"is_published"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		jsonError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// 3. Обновляем поля
	course.Title = input.Title
	course.Description = input.Description
	course.IsPublished = input.IsPublished

	if err := s.DB.Save(&course).Error; err != nil {
		jsonError(w, "Failed to update course", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(course)
}

func (s *Service) deleteCourse(w http.ResponseWriter, r *http.Request, id int) {
	// Удаляем курс по ID
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

// Helper для отправки ошибок в JSON
func jsonError(w http.ResponseWriter, message string, code int) {
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

// =======================
// MODULES API
// =======================

// POST /api/modules (Создать модуль)
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

// PUT /api/modules/{id} (Обновить название модуля)
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

// DELETE /api/modules/{id} (Удалить модуль)
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

// POST /api/lessons (Создать урок)
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

// PUT /api/lessons/{id} (Обновить урок - название или контент)
func (s *Service) UpdateLessonAPI(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.Atoi(vars["id"])

	// Используем map, чтобы можно было обновлять поля по отдельности
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

// DELETE /api/lessons/{id} (Удалить урок)
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

// ... (Ваши существующие методы) ...

// GET /api/lessons/{id} (Получить детали урока, включая контент)
// ... импорты (gorm, datatypes, models, etc)

// PUT /api/lessons/{id}/content
// Этот метод полностью перезаписывает контент урока
func (s *Service) UpdateLessonContentAPI(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	lessonID, _ := strconv.Atoi(vars["id"])

	// Принимаем массив блоков с фронта
	var inputBlocks []struct {
		Type string         `json:"type"`
		Data datatypes.JSON `json:"data"` // Фронт шлет объект, мы кладем его в JSON
	}

	if err := json.NewDecoder(r.Body).Decode(&inputBlocks); err != nil {
		jsonError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Начинаем транзакцию, чтобы данные не потерялись при сбое
	tx := s.DB.Begin()

	// 1. Удаляем старые блоки для этого урока (Hard Delete)
	if err := tx.Where("lesson_id = ?", lessonID).Delete(&models.ContentBlock{}).Error; err != nil {
		tx.Rollback()
		jsonError(w, "Failed to clear old content", http.StatusInternalServerError)
		return
	}

	// 2. Создаем новые блоки с правильным порядком
	for i, block := range inputBlocks {
		newBlock := models.ContentBlock{
			LessonID: uint(lessonID),
			Type:     block.Type,
			Order:    i, // Порядок берем из индекса массива
			Data:     block.Data,
		}
		if err := tx.Create(&newBlock).Error; err != nil {
			tx.Rollback()
			jsonError(w, "Failed to save block", http.StatusInternalServerError)
			return
		}
	}

	tx.Commit()

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "saved"})
}

// GET /api/lessons/{id}
// Теперь нужно подгружать блоки с сортировкой
func (s *Service) GetLessonAPI(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.Atoi(vars["id"])

	var lesson models.Lesson

	// Preload блоков с сортировкой по order
	err := s.DB.Preload("ContentBlocks", func(db *gorm.DB) *gorm.DB {
		return db.Order("content_blocks.order ASC") // Важно! Сортируем по порядку
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

// ==========================================
// 1. GET: Получить структуру (Для открытия модалки)
// ==========================================
func (s *Service) GetCourseStructure(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.Atoi(vars["id"])

	// 1. Грузим курс + модули + уроки
	var course models.Course
	// Используем Preload для связей. Если у тебя нет сортировки (Order), просто Preload.
	err := s.DB.Preload("Modules").Preload("Modules.Lessons").First(&course, id).Error

	if err != nil {
		jsonError(w, "Курс не найден", http.StatusNotFound)
		return
	}

	// Готовим ответ по умолчанию (как для гостя)
	resp := CourseStructureResponse{
		Course:        course,
		IsAuth:        false,
		RequestStatus: "",
	}

	// 2. Пытаемся определить пользователя (ТВОЙ КОД)
	session, _ := s.Store.Get(r, "session")
	val := session.Values["user_id"]

	// Если user_id есть в сессии, значит юзер авторизован
	if val != nil {
		var userID uint
		var isValidUser bool = false

		// Твоя логика приведения типов
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

		// Если удалось достать ID
		if isValidUser {
			resp.IsAuth = true // Ставим флаг, что юзер вошел

			// 3. Проверяем статус заявки в БД
			var enrollment models.Enrollment
			// Ищем запись: user_id + course_id
			if err := s.DB.Where("user_id = ? AND course_id = ?", userID, course.ID).First(&enrollment).Error; err == nil {
				resp.RequestStatus = enrollment.Status // "pending", "approved"...
			}
		}
	}

	// Отправляем JSON
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// ==========================================
// 2. POST: Отправить заявку (Кнопка "Отправить")
// ==========================================
func (s *Service) SubmitEnrollment(w http.ResponseWriter, r *http.Request) {
	// 1. Строгая проверка авторизации (ТВОЙ КОД)
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
	// Твоя логика приведения типов
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

	// 2. Читаем ID курса из тела запроса
	var req struct {
		CourseID uint `json:"course_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// 3. Проверяем, нет ли уже заявки
	var existing models.Enrollment
	if err := s.DB.Where("user_id = ? AND course_id = ?", userID, req.CourseID).First(&existing).Error; err == nil {
		// Заявка уже есть
		jsonError(w, "Заявка уже существует", http.StatusConflict)
		return
	}

	// 4. Создаем заявку
	enrollment := models.Enrollment{
		UserID:   userID,
		CourseID: req.CourseID,
		Status:   "pending", // Статус "На рассмотрении"
	}

	if err := s.DB.Create(&enrollment).Error; err != nil {
		jsonError(w, "Database error", http.StatusInternalServerError)
		return
	}

	// Успех
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":         "success",
		"request_status": "pending",
	})
}
