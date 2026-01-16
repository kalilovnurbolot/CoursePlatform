package handlers

import (
	"context"
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"time"

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

	funcMap := template.FuncMap{
		"mod": func(i, j int) int {
			return i % j
		},
		"add": func(i, j int) int {
			return i + j
		},
		"formatTime": func(t *time.Time) string {
			if t == nil {
				return "Никогда"
			}
			return t.Format("02.01.2006 в 15:04")
		},
	}

	tmpl := template.New("").Funcs(funcMap)

	// 1. Парсим файлы в корне папки template (например, index.html)
	_, err := tmpl.ParseGlob("template/*.html")
	if err != nil {
		// Не фатально, если в корне нет html, но полезно знать
		log.Println("Warning parsing root templates:", err)
	}

	// 2. Парсим файлы во вложенных папках (например, template/admin/...)
	_, err = tmpl.ParseGlob("template/**/*.html")
	if err != nil {
		log.Fatal("Error parsing nested templates:", err)
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

	Lesson         models.Lesson
	PrevLessonID   uint
	IsLessonDone   bool
	AttemptsJSON   string
	CourseLanguage string
}

func (h *Handler) GetAuthenticatedUserID(r *http.Request) (uint, bool) {
	session, _ := h.Store.Get(r, "session")

	userIDValue := session.Values["user_id"]
	userID, ok := userIDValue.(uint)

	return userID, ok && userID != 0
}

func (h *Handler) GetUserRoleID(r *http.Request) (uint, uint) {
	session, _ := h.Store.Get(r, "session")

	userIDvalue := session.Values["user_id"]
	userID, _ := userIDvalue.(uint)

	roleID := models.RoleGuest

	if userID != 0 {
		var user models.User
		err := h.DB.Select("role_id").First(&user, userID).Error

		if err == nil {
			roleID = user.RoleID
		}
	}

	return roleID, userID
}

func (h *Handler) HandleMain(w http.ResponseWriter, r *http.Request) {
	roleID, userID := h.GetUserRoleID(r)
	session, _ := h.Store.Get(r, "session")

	data := PageData{
		Title:           "Главная",
		IsAuthenticated: userID != 0,
		UserID:          userID,
		RoleID:          roleID,
		UserName:        toString(session.Values["name"]),
		UserPictureURL:  toString(session.Values["picture_url"]),
	}

	// Добавляем проверку ошибки!
	if err := h.Tmpl.ExecuteTemplate(w, "index.html", data); err != nil {
		log.Printf("Error rendering index.html: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// GetHomeDataAPI - возвращает JSON с курсами и отзывами
func (h *Handler) GetHomeDataAPI(w http.ResponseWriter, r *http.Request) {
	var response struct {
		Courses []models.Course `json:"courses"`
		Reviews []models.Review `json:"reviews"`
	}

	// 1. Загружаем курсы
	if err := h.DB.Preload("Author").Where("is_published = ?", true).Find(&response.Courses).Error; err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// 2. Загружаем отзывы
	h.DB.Preload("User").Preload("Course").
		Where("rating >= ?", 4).
		Order("created_at desc").
		Limit(6).
		Find(&response.Reviews)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func toString(v interface{}) string {
	s, _ := v.(string)
	return s
}

func (h *Handler) HandleAdmin(w http.ResponseWriter, r *http.Request) {}

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

func (h *Handler) HandleForbiddenPage(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusForbidden)
	tmpl, err := template.ParseFiles("template/exceptions/403.html")
	if err != nil {
		http.Error(w, "Could not load template", http.StatusInternalServerError)
		return
	}
	tmpl.Execute(w, nil)
}
