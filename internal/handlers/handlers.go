package handlers

import (
	"context"
	"encoding/json"
	"html/template"
	"log"
	"net/http"

	"github.com/gorilla/sessions"
	"golang.org/x/oauth2"

	"github.com/s/onlineCourse/internal/models"
	"github.com/s/onlineCourse/internal/storage"

	"gorm.io/gorm"
)

type Handler struct {
	DB     *gorm.DB
	Store  *sessions.CookieStore
	Config *oauth2.Config
	Tmpl   *template.Template
}

func NewHandler(db *gorm.DB, store *sessions.CookieStore, config *oauth2.Config) *Handler {

	// 1. Определяем функцию mod и создаем карту функций
	funcMap := template.FuncMap{
		"mod": func(i, j int) int {
			return i % j
		},
		"add": func(i, j int) int {
			return i + j
		},
	}

	// 2. Инициализируем шаблон, регистрируем функции, а ЗАТЕМ парсим файлы
	// Важно: template.New("") создает пустой шаблон, к которому мы привязываем функции.
	tmpl, err := template.New("").Funcs(funcMap).ParseFiles(
		"template/layouts/layout.html",
		"template/layouts/header.html",
		"template/layouts/footer.html",
		"template/index.html",
		"template/personal/profile.html",

		"template/admin/index.html",
		"template/admin/users.html",
		"template/admin/report.html",
		"template/admin/coursePage.html",
		"template/admin/enrollment.html",
		"template/layouts/adminBar.html",
		"template/layouts/headerPersonal.html",

		"template/student/dashboard.html",
		"template/student/courseContents.html",
		"template/student/lessonView.html",
	)

	if err != nil {
		log.Fatal(err)
	}

	return &Handler{
		DB:     db,
		Store:  store,
		Config: config,
		Tmpl:   tmpl,
	}
}

type PageData struct {
	Title           string
	IsAuthenticated bool
	UserID          uint
	RoleID          uint
	Email           string
	UserName        string
	UserPictureURL  string
	CurrentPath     string

	Courses         []models.Course
	ColorPool       []string
	Enrollments     []models.Enrollment
	StudentCourses  []StudentCourseView
	Course          models.Course
	DoneLessonsMap  map[uint]bool
	TotalLessons    int
	ProgressPercent int
	NextLessonID    uint

	Lesson       models.Lesson
	PrevLessonID uint
	IsLessonDone bool
	AttemptsJSON string
}

func (h *Handler) GetAuthenticatedUserID(r *http.Request) (uint, bool) {
	session, _ := h.Store.Get(r, "session")

	userIDValue := session.Values["user_id"]
	userID, ok := userIDValue.(uint)

	return userID, ok && userID != 0
}

// GetUserRoleID retrieves the user's role ID from the database.
// It returns models.RoleGuest (0) if the user is not authenticated or not found.
// Возвращаем int для RoleID (чтобы соответствовать константам models.RoleGuest)
// и uint для UserID.

func (h *Handler) GetUserRoleID(r *http.Request) (uint, uint) { // ✅ Исправленное объявление
	session, _ := h.Store.Get(r, "session")

	userIDvalue := session.Values["user_id"]
	userID, _ := userIDvalue.(uint)

	// Default to the guest role (assuming models.RoleGuest is int)
	roleID := models.RoleGuest // RoleID типа int

	if userID != 0 {
		var user models.User
		// Используем .Select("role_id")
		err := h.DB.Select("role_id").First(&user, userID).Error

		if err == nil {
			roleID = user.RoleID // user.RoleID - int
		}
	}

	// Возвращаем оба значения
	return roleID, userID // ✅ Корректный возврат
}

func (h *Handler) HandleMain(w http.ResponseWriter, r *http.Request) {
	roleID, userID := h.GetUserRoleID(r)
	session, _ := h.Store.Get(r, "session")

	// 1. Загружаем список курсов
	var allCourses []models.Course
	if err := h.DB.Preload("Author").Where("is_published = ?", true).Find(&allCourses).Error; err != nil {
		log.Printf("Ошибка при получении курсов: %v", err)
		http.Error(w, "Не удалось загрузить данные курсов", http.StatusInternalServerError)
		return
	}

	// 2. Определяем пул градиентов (для визуального эффекта)
	colorPool := []string{
		"from-purple-500 to-indigo-600",
		"from-green-400 to-blue-500",
		"from-pink-500 to-red-500",
		"from-yellow-400 to-orange-500",
		"from-sky-400 to-cyan-500",
	}

	// 3. Создаем PageData и вкладываем курсы и цвета
	data := PageData{
		Title:           "Главная",
		IsAuthenticated: userID != 0,
		UserID:          userID,
		RoleID:          roleID,
		UserName:        toString(session.Values["name"]),
		UserPictureURL:  toString(session.Values["picture_url"]),

		// ⭐ Передаем загруженные данные и пул цветов
		Courses:   allCourses,
		ColorPool: colorPool,
	}

	// 4. Выполняем шаблон с единственным объектом 'data'
	h.Tmpl.ExecuteTemplate(w, "index.html", data)
}

func toString(v interface{}) string {
	s, _ := v.(string)
	return s
}

func (h *Handler) HandleAdmin(w http.ResponseWriter, r *http.Request) {

}

func (h *Handler) HandleProfile(w http.ResponseWriter, r *http.Request) {
	session, _ := h.Store.Get(r, "session")

	userIDvalue := session.Values["user_id"]
	userID, ok := userIDvalue.(uint)
	if !ok {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	var user models.User
	if err := h.DB.Preload("Role").First(&user, userID).Error; err != nil {
		http.Error(w, "Пользователь не найден", http.StatusNotFound)
		return
	}

	data := PageData{
		Title:           "Мой Профиль",
		IsAuthenticated: true,
		UserID:          user.ID,
		Email:           user.Email,
		UserName:        user.Name,
		UserPictureURL:  user.Picture,
		RoleID:          user.RoleID,
	}

	h.Tmpl.ExecuteTemplate(w, "profile.html", data)
}

func (h *Handler) HandleGoogleLogin(w http.ResponseWriter, r *http.Request) {
	url := h.Config.AuthCodeURL("random_state")
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

func (h *Handler) HandleGoogleCallback(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("state") != "random_state" {
		http.Error(w, "Invalid state", http.StatusUnauthorized)
		return
	}

	code := r.URL.Query().Get("code")
	token, err := h.Config.Exchange(context.Background(), code)
	if err != nil {
		http.Error(w, "Token exchange error", http.StatusBadRequest)
		return
	}

	client := h.Config.Client(context.Background(), token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		http.Error(w, "Google API error", http.StatusBadRequest)
		return
	}
	defer resp.Body.Close()

	var userInfo models.User
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		http.Error(w, "JSON decode error", http.StatusInternalServerError)
		return
	}

	// --- сохраняем через GORM ---
	userID, err := storage.SaveUser(h.DB, userInfo)
	if err != nil {
		http.Error(w, "DB save error", http.StatusInternalServerError)
		return
	}

	session, _ := h.Store.Get(r, "session")
	session.Values["user_id"] = userID
	session.Values["email"] = userInfo.Email
	session.Values["name"] = userInfo.Name
	session.Values["picture_url"] = userInfo.Picture
	session.Options = &sessions.Options{
		Path:     "/",
		HttpOnly: true,
		MaxAge:   86400 * 7,
	}
	session.Save(r, w)

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *Handler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	session, _ := h.Store.Get(r, "session")
	session.Options.MaxAge = -1
	session.Save(r, w)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// ... внутри internal/handlers/handlers.go

// HandleForbiddenPage отображает страницу ошибки 403.
func (h *Handler) HandleForbiddenPage(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusForbidden) // Устанавливаем статус 403

	// Предполагаем, что у вас есть функция для рендеринга шаблонов
	// (В реальном приложении нужно использовать шаблонизатор, например, html/template)

	// Простейший пример рендеринга:
	tmpl, err := template.ParseFiles("template/exceptions/403.html")
	if err != nil {
		http.Error(w, "Could not load template", http.StatusInternalServerError)
		return
	}
	tmpl.Execute(w, nil)
}

// ⚠️ Примечание: Убедитесь, что вы импортировали "html/template"
