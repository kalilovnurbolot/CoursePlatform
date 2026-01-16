package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/joho/godotenv"
	"github.com/s/onlineCourse/internal/auth"
	"github.com/s/onlineCourse/internal/database"
	"github.com/s/onlineCourse/internal/handlers"
	"github.com/s/onlineCourse/internal/handlers/admin"
	"github.com/s/onlineCourse/internal/middleware"
	"github.com/s/onlineCourse/internal/models"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("Предупреждение: Не удалось загрузить файл .env. Используются системные переменные.")
	}

	db, err := database.Connect()
	if err != nil {
		log.Fatal("Ошибка подключения к БД:", err)
	}

	if err := database.AutoMigrate(db); err != nil {
		log.Fatal("Ошибка миграции:", err)
	}

	if err := database.Seed(db); err != nil {
		log.Printf("Предупреждение: ошибка сидинга: %v", err)
	}

	clientId := os.Getenv("GOOGLE_CLIENT_ID")
	clientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")
	redirectURL := os.Getenv("GOOGLE_REDIRECT_URL")

	if clientId == "" || clientSecret == "" || redirectURL == "" {
		log.Fatal("Ошибка: Переменные GOOGLE_... не установлены в .env")
	}

	oauthConfig := auth.InitGoogleOAuthConfig(clientId, clientSecret, redirectURL)

	sessionKey := os.Getenv("SESSION_KEY")
	if sessionKey == "" {
		sessionKey = "super-secret-default-key"
		log.Println("Внимание: SESSION_KEY не задан, используется дефолтный.")
	}
	store := sessions.NewCookieStore([]byte(sessionKey))
	store.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   86400 * 7,
		HttpOnly: true,
		Secure:   false,
	}

	h := handlers.NewHandler(db, store, oauthConfig)
	adminService := admin.Service{Handler: *h}

	adminMiddleware := middleware.RequiredRole(h, models.RoleAdmin)
	userMiddleware := middleware.RequiredRole(h, models.RoleUser)

	r := mux.NewRouter()
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))

	r.HandleFunc("/", h.HandleMain).Methods("GET")
	r.HandleFunc("/api/home", h.GetHomeDataAPI).Methods("GET") // <--- NEW

	r.HandleFunc("/auth/google/login", h.HandleGoogleLogin).Methods("GET")
	r.HandleFunc("/auth/google/callback", h.HandleGoogleCallback).Methods("GET")
	r.HandleFunc("/logout", h.HandleLogout).Methods("GET", "POST")

	r.HandleFunc("/personal", h.HandleProfile).Methods("GET")

	// Админка
	r.HandleFunc("/admin/dashboard", adminMiddleware(adminService.HandleAdminPage)).Methods("GET")
	r.HandleFunc("/admin/reports", adminMiddleware(adminService.HandleReportPage)).Methods("GET")
	r.HandleFunc("/admin/courses", adminMiddleware(adminService.HandleCoursePage)).Methods("GET")
	r.HandleFunc("/admin/users", adminMiddleware(adminService.HandleUsersPage)).Methods("GET")

	// API Админки
	r.HandleFunc("/api/courses", adminMiddleware(adminService.HandleCoursesAPI)).Methods("GET", "POST")
	r.HandleFunc("/api/courses/{id}", adminMiddleware(adminService.HandleCourseByIDAPI)).Methods("GET", "PUT", "DELETE")
	r.HandleFunc("/api/modules", adminMiddleware(adminService.CreateModuleAPI)).Methods("POST")
	r.HandleFunc("/api/modules/{id}", adminMiddleware(adminService.UpdateModuleAPI)).Methods("PUT")
	r.HandleFunc("/api/modules/{id}", adminMiddleware(adminService.DeleteModuleAPI)).Methods("DELETE")
	r.HandleFunc("/api/lessons", adminMiddleware(adminService.CreateLessonAPI)).Methods("POST")
	r.HandleFunc("/api/lessons/{id}", adminMiddleware(adminService.UpdateLessonAPI)).Methods("PUT")
	r.HandleFunc("/api/lessons/{id}", adminMiddleware(adminService.DeleteLessonAPI)).Methods("DELETE")
	r.HandleFunc("/api/lessons/{id}", adminMiddleware(adminService.GetLessonAPI)).Methods("GET")
	r.HandleFunc("/api/lessons/{id}/content", adminMiddleware(adminService.UpdateLessonContentAPI)).Methods("PUT")
	r.HandleFunc("/admin/enrollments", adminMiddleware(adminService.HandleEnrollmentsPage)).Methods("GET")
	r.HandleFunc("/api/admin/enrollments", adminMiddleware(adminService.GetEnrollmentsAPI)).Methods("GET")
	r.HandleFunc("/api/admin/enrollments/{id}", adminMiddleware(adminService.UpdateEnrollmentStatusAPI)).Methods("PUT")

	// Студент
	r.HandleFunc("/api/courses/{id}/structure", adminService.GetCourseStructure).Methods("GET")
	r.HandleFunc("/api/enroll", userMiddleware(adminService.SubmitEnrollment)).Methods("POST")
	r.HandleFunc("/my-courses", userMiddleware(h.HandleStudentDashboard)).Methods("GET")
	r.HandleFunc("/course/{id:[0-9]+}/learn", userMiddleware(h.HandleCourseLearn)).Methods("GET")
	r.HandleFunc("/api/course/{id:[0-9]+}/lesson/{lesson_id:[0-9]+}/quiz", userMiddleware(h.SaveQuizAttemptAPI)).Methods("POST")
	r.HandleFunc("/api/course/{id:[0-9]+}/lesson/{lesson_id:[0-9]+}/done", userMiddleware(h.MarkLessonReadAPI)).Methods("POST")
	r.HandleFunc("/course/{id:[0-9]+}/lesson/{lesson_id:[0-9]+}", userMiddleware(h.HandleLessonView)).Methods("GET")

	// --- КОММЕНТАРИИ И ОТЗЫВЫ ---
	r.HandleFunc("/api/lessons/{id}/comments", userMiddleware(h.AddCommentAPI)).Methods("POST")
	r.HandleFunc("/api/lessons/{id}/comments", userMiddleware(h.GetCommentsAPI)).Methods("GET")
	r.HandleFunc("/api/courses/{id}/reviews", userMiddleware(h.AddReviewAPI)).Methods("POST")
	r.HandleFunc("/api/courses/{id}/reviews", h.GetReviewsAPI).Methods("GET")

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	corsHandler := corsMiddleware(r)
	fmt.Printf("Сервер запущен: http://localhost:%s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, corsHandler))
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
