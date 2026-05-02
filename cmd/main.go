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
	"github.com/s/onlineCourse/internal/i18n"
	"github.com/s/onlineCourse/internal/middleware"
	"github.com/s/onlineCourse/internal/models"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("Warning: .env not found, using system environment variables.")
	}

	if err := i18n.Load("locales"); err != nil {
		log.Fatal("Failed to load translations:", err)
	}

	db, err := database.Connect()
	if err != nil {
		log.Fatal("DB connection error:", err)
	}

	if err := database.AutoMigrate(db); err != nil {
		log.Fatal("Migration error:", err)
	}

	if err := database.Seed(db); err != nil {
		log.Printf("Warning: seed error: %v", err)
	}

	clientId := os.Getenv("GOOGLE_CLIENT_ID")
	clientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")
	redirectURL := os.Getenv("GOOGLE_REDIRECT_URL")

	if clientId == "" || clientSecret == "" || redirectURL == "" {
		log.Fatal("Error: GOOGLE_* environment variables are not set")
	}

	oauthConfig := auth.InitGoogleOAuthConfig(clientId, clientSecret, redirectURL)

	sessionKey := os.Getenv("SESSION_KEY")
	if sessionKey == "" {
		sessionKey = "super-secret-default-key"
		log.Println("Warning: SESSION_KEY not set, using default.")
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

	r.HandleFunc("/robots.txt", h.HandleRobotsTxt).Methods("GET")
	r.HandleFunc("/sitemap.xml", h.HandleSitemapXML).Methods("GET")

	r.HandleFunc("/", h.HandleMain).Methods("GET")
	r.HandleFunc("/about", h.HandleAboutPage).Methods("GET")
	r.HandleFunc("/api/home", h.GetHomeDataAPI).Methods("GET")
	r.HandleFunc("/api/language", h.HandleSetLanguage).Methods("POST")

	r.HandleFunc("/auth/google/login", h.HandleGoogleLogin).Methods("GET")
	r.HandleFunc("/auth/google/callback", h.HandleGoogleCallback).Methods("GET")
	r.HandleFunc("/logout", h.HandleLogout).Methods("GET", "POST")

	r.HandleFunc("/personal", h.HandleProfile).Methods("GET")
	r.HandleFunc("/cabinet", userMiddleware(h.HandleCabinet)).Methods("GET")
	r.HandleFunc("/certificate/{code}", h.HandleVerifyCertificate).Methods("GET")

	// Admin pages
	r.HandleFunc("/admin/dashboard", adminMiddleware(adminService.HandleAdminPage)).Methods("GET")
	r.HandleFunc("/admin/reports", adminMiddleware(adminService.HandleReportPage)).Methods("GET")
	r.HandleFunc("/admin/courses", adminMiddleware(adminService.HandleCoursePage)).Methods("GET")
	r.HandleFunc("/admin/users", adminMiddleware(adminService.HandleUsersPage)).Methods("GET")

	// Admin API
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

	// Student
	r.HandleFunc("/api/courses/{id}/structure", adminService.GetCourseStructure).Methods("GET")
	r.HandleFunc("/api/enroll", userMiddleware(adminService.SubmitEnrollment)).Methods("POST")
	r.HandleFunc("/my-courses", userMiddleware(h.HandleStudentDashboard)).Methods("GET")
	r.HandleFunc("/course/{id:[0-9]+}/learn", h.HandleCourseLearn).Methods("GET")
	r.HandleFunc("/api/course/{id:[0-9]+}/lesson/{lesson_id:[0-9]+}/quiz", userMiddleware(h.SaveQuizAttemptAPI)).Methods("POST")
	r.HandleFunc("/api/course/{id:[0-9]+}/lesson/{lesson_id:[0-9]+}/done", userMiddleware(h.MarkLessonReadAPI)).Methods("POST")
	r.HandleFunc("/course/{id:[0-9]+}/lesson/{lesson_id:[0-9]+}", h.HandleLessonView).Methods("GET")

	// Public user profile
	r.HandleFunc("/user/{public_id}", h.HandleUserProfilePage).Methods("GET")

	// Studio (all authenticated users — author-scoped)
	r.HandleFunc("/studio", userMiddleware(h.HandleStudioPage)).Methods("GET")
	r.HandleFunc("/api/studio/courses", userMiddleware(h.StudioGetCoursesAPI)).Methods("GET")
	r.HandleFunc("/api/studio/courses", userMiddleware(h.StudioCreateCourseAPI)).Methods("POST")
	r.HandleFunc("/api/studio/courses/{id:[0-9]+}", userMiddleware(h.StudioUpdateCourseAPI)).Methods("PUT")
	r.HandleFunc("/api/studio/courses/{id:[0-9]+}", userMiddleware(h.StudioDeleteCourseAPI)).Methods("DELETE")
	r.HandleFunc("/api/studio/courses/{id:[0-9]+}/submit", userMiddleware(h.StudioSubmitCourseAPI)).Methods("POST")
	r.HandleFunc("/api/studio/courses/{id:[0-9]+}/structure", userMiddleware(h.StudioGetCourseStructureAPI)).Methods("GET")
	r.HandleFunc("/api/studio/modules", userMiddleware(h.StudioCreateModuleAPI)).Methods("POST")
	r.HandleFunc("/api/studio/modules/{id:[0-9]+}", userMiddleware(h.StudioUpdateModuleAPI)).Methods("PUT")
	r.HandleFunc("/api/studio/modules/{id:[0-9]+}", userMiddleware(h.StudioDeleteModuleAPI)).Methods("DELETE")
	r.HandleFunc("/api/studio/lessons", userMiddleware(h.StudioCreateLessonAPI)).Methods("POST")
	r.HandleFunc("/api/studio/lessons/{id:[0-9]+}", userMiddleware(h.StudioUpdateLessonAPI)).Methods("PUT")
	r.HandleFunc("/api/studio/lessons/{id:[0-9]+}", userMiddleware(h.StudioDeleteLessonAPI)).Methods("DELETE")
	r.HandleFunc("/api/studio/lessons/{id:[0-9]+}", userMiddleware(h.StudioGetLessonAPI)).Methods("GET")
	r.HandleFunc("/api/studio/lessons/{id:[0-9]+}/content", userMiddleware(h.StudioUpdateLessonContentAPI)).Methods("PUT")

	// Admin — course review requests
	r.HandleFunc("/admin/course-requests", adminMiddleware(adminService.HandleCourseRequestsPage)).Methods("GET")
	r.HandleFunc("/api/admin/course-requests", adminMiddleware(adminService.GetCourseRequestsAPI)).Methods("GET")
	r.HandleFunc("/api/admin/course-requests/{id:[0-9]+}", adminMiddleware(adminService.ReviewCourseRequestAPI)).Methods("PUT")

	// Admin — user management
	r.HandleFunc("/api/admin/users", adminMiddleware(adminService.GetUsersAPI)).Methods("GET")
	r.HandleFunc("/api/admin/users/{id:[0-9]+}/role", adminMiddleware(adminService.UpdateUserRoleAPI)).Methods("PUT")

	// Comments & Reviews
	r.HandleFunc("/api/lessons/{id}/comments", userMiddleware(h.AddCommentAPI)).Methods("POST")
	r.HandleFunc("/api/lessons/{id}/comments", h.GetCommentsAPI).Methods("GET")
	r.HandleFunc("/api/courses/{id}/reviews", userMiddleware(h.AddReviewAPI)).Methods("POST")
	r.HandleFunc("/api/courses/{id}/reviews", h.GetReviewsAPI).Methods("GET")

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	corsHandler := corsMiddleware(r)
	fmt.Printf("Server started: http://localhost:%s\n", port)
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
