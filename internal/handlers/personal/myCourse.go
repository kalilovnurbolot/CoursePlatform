package personal

import (
	"encoding/json"
	"log"
	"math"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/s/onlineCourse/internal/handlers"
	"github.com/s/onlineCourse/internal/models"
)

func toString(v interface{}) string {
	s, _ := v.(string)
	return s
}

type Service struct {
	handlers.Handler
}

func jsonError(w http.ResponseWriter, message string, code int) {
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

// ==========================================
// 1. HTML Хендлер: Страница Заявок
// ==========================================
func (s *Service) HandleEnrollmentsPage(w http.ResponseWriter, r *http.Request) {
	// ... Тут твоя стандартная проверка сессии и роли (как в HandleCoursePage) ...
	// (Я пропущу код проверки сессии для краткости, используй свой шаблон)
	roleID, userID := s.GetUserRoleID(r)
	session, _ := s.Store.Get(r, "session")

	// 1. Загружаем список курсов
	var enrollments []models.Enrollment
	if err := s.DB.Preload("Author").Where("user_id = ?", userID).Find(&enrollments).Error; err != nil {
		log.Printf("Ошибка при получении заявок: %v", err)
		http.Error(w, "Не удалось загрузить данные курсов", http.StatusInternalServerError)
		return
	}

	courseIDs := enrollments.CourseId

	// Нам нужно передать список курсов для выпадающего списка фильтра
	var courses []models.Course
	s.DB.Select("id, title").Where("is_published = ?", true).Where("author_id = ?", userID).Find(&courses)
	session, err := s.Store.Get(r, "session")
	if err != nil {
		// Логируем ошибку и, возможно, перенаправляем на страницу входа или показываем ошибку сервера.
		// В продакшене тут лучше логгировать 'err'.
		http.Error(w, "Ошибка получения сессии. Попробуйте войти снова.", http.StatusInternalServerError)
		return
	}
	data := handlers.PageData{
		Title:           "Главная",
		IsAuthenticated: userID != 0,
		UserID:          userID,
		RoleID:          roleID,
		UserName:        toString(session.Values["name"]),
		UserPictureURL:  toString(session.Values["picture_url"]),
		CurrentPath:     r.URL.Path,

		// Передаем курсы в шаблон, чтобы заполнить <select>
		Courses: courses,
	}

	// Рендерим шаблон (создадим его ниже)
	s.Handler.Tmpl.ExecuteTemplate(w, "adminEnrollments", data)
}

// ==========================================
// 2. API: Получение списка с фильтрами
// ==========================================
// ==========================================
// API: Получение списка заявок с фильтрами
// ==========================================
func (s *Service) GetEnrollmentsAPI(w http.ResponseWriter, r *http.Request) {

	session, err := s.Store.Get(r, "session")
	if err != nil {
		// Логируем ошибку и, возможно, перенаправляем на страницу входа или показываем ошибку сервера.
		// В продакшене тут лучше логгировать 'err'.
		http.Error(w, "Ошибка получения сессии. Попробуйте войти снова.", http.StatusInternalServerError)
		return
	}

	userIDvalue := session.Values["user_id"]
	userID, ok := userIDvalue.(uint)
	if !ok || userID == 0 { // Добавим проверку, что userID не равен 0 для неаутентифицированных пользователей
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	var user models.User
	if err := s.DB.Preload("Role").First(&user, userID).Error; err != nil {
		// Если пользователь с ID из сессии не найден, это тоже проблема
		http.Error(w, "Пользователь не найден", http.StatusNotFound)
		return
	}

	// Если пользователь найден, но он не админ (RoleID не 2), перенаправляем его
	if user.RoleID != 2 {
		http.Redirect(w, r, "/", http.StatusForbidden)
		return
	}
	// 1. Парсим параметры

	query := r.URL.Query()

	page, _ := strconv.Atoi(query.Get("page"))
	if page < 1 {
		page = 1
	}

	limit, _ := strconv.Atoi(query.Get("limit"))
	if limit < 1 {
		limit = 10
	}

	search := query.Get("search")
	courseID := query.Get("course_id")
	status := query.Get("status")
	dateFrom := query.Get("date_from")
	dateTo := query.Get("date_to")

	var total int64

	// 2. Строим базовый запрос
	// Preload нужен, чтобы видеть имена студентов и названия курсов
	db := s.DB.Model(&models.Enrollment{})

	// 1. Грузим данные (чтобы они были в JSON)
	db = db.Preload("User").Preload("Course")

	// 2. Делаем JOIN для фильтрации
	// Важно: в Where указываем таблицу "courses" (во множественном числе, как в БД)
	db = db.Joins("JOIN courses ON courses.id = enrollments.course_id")
	db = db.Where("courses.author_id = ?", userID)

	// 3. Выполняем
	var enrollments []models.Enrollment
	db.Find(&enrollments)

	// --- ФИЛЬТРЫ ---

	// По Курсу
	if courseID != "" && courseID != "0" {
		// Указываем таблицу явно (enrollments.course_id), чтобы не было конфликтов
		db = db.Where("enrollments.course_id = ?", courseID)
	}

	// По Статусу
	if status != "" && status != "all" {
		db = db.Where("enrollments.status = ?", status)
	}

	// По Дате
	if dateFrom != "" {
		db = db.Where("enrollments.created_at >= ?", dateFrom)
	}
	if dateTo != "" {
		db = db.Where("enrollments.created_at <= ?", dateTo+" 23:59:59")
	}

	// Поиск (JOIN Users)
	if search != "" {
		searchLike := "%" + search + "%"
		// JOIN нужен, чтобы искать по имени юзера, хотя мы выбираем enrollments
		db = db.Joins("JOIN users ON users.id = enrollments.user_id").
			Where("users.name ILIKE ? OR users.email ILIKE ?", searchLike, searchLike)
	}

	// 3. Считаем Total (до пагинации)
	db.Count(&total)

	// 4. Пагинация и получение данных
	offset := (page - 1) * limit

	// Сортируем: сначала новые
	err = db.Order("enrollments.created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&enrollments).Error

	if err != nil {
		jsonError(w, "Ошибка базы данных: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 5. Формируем ответ
	totalPages := int(math.Ceil(float64(total) / float64(limit)))

	json.NewEncoder(w).Encode(map[string]interface{}{
		"data":  enrollments,
		"total": total,
		"page":  page,
		"pages": totalPages,
	})
}

// ==========================================
// API: Изменение статуса (Одобрить/Отклонить)
// ==========================================
func (s *Service) UpdateEnrollmentStatusAPI(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	// Читаем JSON body
	var req struct {
		Status string `json:"status"` // ожидаем "approved" или "rejected"
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Неверный формат JSON", http.StatusBadRequest)
		return
	}

	// Валидация статуса (опционально, но полезно)
	if req.Status != "approved" && req.Status != "rejected" && req.Status != "pending" {
		jsonError(w, "Недопустимый статус", http.StatusBadRequest)
		return
	}

	// Обновляем в БД
	// Используем Model(&models.Enrollment{}), чтобы GORM знал, какую таблицу обновлять
	if err := s.DB.Model(&models.Enrollment{}).Where("id = ?", id).Update("status", req.Status).Error; err != nil {
		jsonError(w, "Ошибка при обновлении статуса", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"result": "success"})
}
