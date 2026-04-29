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

	"github.com/s/onlineCourse/internal/i18n"
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
		"mod": func(a, b int) int { return a % b },
		"add": func(a, b int) int { return a + b },
		"formatTime": func(t *time.Time) string {
			if t == nil {
				return "—"
			}
			return t.Format("02.01.2006 в 15:04")
		},
		"T": i18n.T,
	}

	tmpl := template.New("").Funcs(funcMap)

	_, err := tmpl.ParseGlob("template/*.html")
	if err != nil {
		log.Println("Warning parsing root templates:", err)
	}

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
	Lang            string
	TransJSON       template.JS

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

	IsCourseOpen bool
	IsLessonFree bool
}

// DetectLang resolves the best language for a request.
// Priority: lang cookie → authenticated user DB lang → Accept-Language → default "ru".
func (h *Handler) DetectLang(r *http.Request) string {
	if c, err := r.Cookie("lang"); err == nil && i18n.IsSupported(c.Value) {
		return c.Value
	}

	if userID, ok := h.GetAuthenticatedUserID(r); ok {
		var u models.User
		if h.DB.Select("language").First(&u, userID).Error == nil && i18n.IsSupported(u.Language) {
			return u.Language
		}
	}

	if header := r.Header.Get("Accept-Language"); header != "" {
		return i18n.FromAcceptLanguage(header)
	}

	return i18n.DefaultLang
}

// buildTransJSON serialises all translations for lang into a template.JS value
// safe to embed inside a <script> tag as a JS object literal.
func buildTransJSON(lang string) template.JS {
	data, _ := json.Marshal(i18n.GetAll(lang))
	return template.JS(data)
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
		if h.DB.Select("role_id").First(&user, userID).Error == nil {
			roleID = user.RoleID
		}
	}

	return roleID, userID
}

func (h *Handler) HandleMain(w http.ResponseWriter, r *http.Request) {
	roleID, userID := h.GetUserRoleID(r)
	session, _ := h.Store.Get(r, "session")
	lang := h.DetectLang(r)

	data := PageData{
		Title:           i18n.T(lang, "nav.home"),
		IsAuthenticated: userID != 0,
		UserID:          userID,
		RoleID:          roleID,
		UserName:        toString(session.Values["name"]),
		UserPictureURL:  toString(session.Values["picture_url"]),
		Lang:            lang,
		TransJSON:       buildTransJSON(lang),
	}

	if err := h.Tmpl.ExecuteTemplate(w, "index.html", data); err != nil {
		log.Printf("Error rendering index.html: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *Handler) GetHomeDataAPI(w http.ResponseWriter, r *http.Request) {
	var response struct {
		Courses     []models.Course `json:"courses"`
		OpenCourses []models.Course `json:"open_courses"`
		Reviews     []models.Review `json:"reviews"`
	}

	if err := h.DB.Preload("Author").Where("is_published = ?", true).Find(&response.Courses).Error; err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	h.DB.Preload("Author").Preload("Modules.Lessons").
		Where("is_published = ? AND is_open = ?", true, true).
		Find(&response.OpenCourses)

	h.DB.Preload("User").Preload("Course").
		Where("rating >= ?", 4).
		Order("created_at desc").
		Limit(6).
		Find(&response.Reviews)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// HandleSetLanguage saves the chosen language to a cookie and, if the user is
// authenticated, persists it to the database.
func (h *Handler) HandleSetLanguage(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Lang string `json:"lang"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || !i18n.IsSupported(body.Lang) {
		http.Error(w, "invalid lang", http.StatusBadRequest)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "lang",
		Value:    body.Lang,
		Path:     "/",
		MaxAge:   86400 * 365,
		HttpOnly: false, // readable by JS for confirmation
		SameSite: http.SameSiteLaxMode,
	})

	if userID, ok := h.GetAuthenticatedUserID(r); ok {
		h.DB.Model(&models.User{}).Where("id = ?", userID).Update("language", body.Lang)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"lang": body.Lang})
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
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	lang := h.DetectLang(r)

	data := PageData{
		Title:           i18n.T(lang, "nav.profile"),
		IsAuthenticated: true,
		UserID:          user.ID,
		Email:           user.Email,
		UserName:        user.Name,
		UserPictureURL:  user.Picture,
		RoleID:          user.RoleID,
		Lang:            lang,
		TransJSON:       buildTransJSON(lang),
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

	// If a lang cookie was set before login, persist it to the user record.
	if c, err := r.Cookie("lang"); err == nil && i18n.IsSupported(c.Value) {
		h.DB.Model(&models.User{}).Where("id = ?", userID).Update("language", c.Value)
	} else {
		// Restore previously saved language preference to cookie.
		var u models.User
		if h.DB.Select("language").First(&u, userID).Error == nil && i18n.IsSupported(u.Language) {
			http.SetCookie(w, &http.Cookie{
				Name:     "lang",
				Value:    u.Language,
				Path:     "/",
				MaxAge:   86400 * 365,
				HttpOnly: false,
				SameSite: http.SameSiteLaxMode,
			})
		}
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

// langFromContext is a helper used by other handler packages via DetectLang.
func LangKey() contextKey { return contextKey("lang") }

type contextKey string

// SetLangCookie writes the lang cookie. Exported for use in tests / other packages.
func SetLangCookie(w http.ResponseWriter, lang string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "lang",
		Value:    lang,
		Path:     "/",
		MaxAge:   86400 * 365,
		HttpOnly: false,
		SameSite: http.SameSiteLaxMode,
	})
}

// DetectLangFromRequest is a package-level helper for use in sub-packages.
func DetectLangFromRequest(r *http.Request) string {
	if c, err := r.Cookie("lang"); err == nil && i18n.IsSupported(c.Value) {
		return c.Value
	}
	if header := r.Header.Get("Accept-Language"); header != "" {
		return i18n.FromAcceptLanguage(header)
	}
	return i18n.DefaultLang
}

// BuildTransJSON is exported so sub-packages (admin, personal) can use it.
func BuildTransJSON(lang string) template.JS {
	data, _ := json.Marshal(i18n.GetAll(lang))
	return template.JS(data)
}

