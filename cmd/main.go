package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/mux" // ⬅️ Импортируем Gorilla Mux
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
	// ---------------------------
	// 0. Загрузка переменных окружения
	// ---------------------------
	err := godotenv.Load()
	if err != nil {
		log.Println("Предупреждение: Не удалось загрузить файл .env. Используются системные переменные.")
	}

	// ---------------------------
	// 1. Подключаем GORM (База данных)
	// ---------------------------
	db, err := database.Connect()
	if err != nil {
		log.Fatal("Ошибка подключения к БД:", err)
	}

	// ---------------------------
	// 2. Делаем миграции
	// ---------------------------
	if err := database.AutoMigrate(db); err != nil {
		log.Fatal("Ошибка миграции:", err)
	}

	// ---------------------------
	// 3. Запускаем сиды (если нужно)
	// ---------------------------
	// if err := database.Seed(db); err != nil {
	//    log.Println("Ошибка сидов (возможно, данные уже есть):", err)
	// }

	// ---------------------------
	// 4. Настраиваем Google OAuth
	// ---------------------------
	clientId := os.Getenv("GOOGLE_CLIENT_ID")
	clientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")
	redirectURL := os.Getenv("GOOGLE_REDIRECT_URL")

	if clientId == "" || clientSecret == "" || redirectURL == "" {
		log.Fatal("Ошибка: Переменные GOOGLE_... не установлены в .env")
	}

	oauthConfig := auth.InitGoogleOAuthConfig(clientId, clientSecret, redirectURL)

	// ---------------------------
	// 5. Настройка сессий
	// ---------------------------
	sessionKey := os.Getenv("SESSION_KEY")
	if sessionKey == "" {
		sessionKey = "super-secret-default-key" // Только для разработки!
		log.Println("Внимание: SESSION_KEY не задан, используется дефолтный.")
	}
	store := sessions.NewCookieStore([]byte(sessionKey))
	// Настройки безопасности куки
	store.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   86400 * 7,
		HttpOnly: true,
		Secure:   false, // Поставьте true, если используете HTTPS
	}

	// ---------------------------
	// 6. Инициализация Хендлеров
	// ---------------------------
	h := handlers.NewHandler(db, store, oauthConfig)

	// Встраиваем основной Handler в Admin Service
	adminService := admin.Service{
		Handler: *h,
	}

	// Middleware для проверки прав админа
	adminMiddleware := middleware.RequiredRole(h, models.RoleAdmin)
	userMiddleware := middleware.RequiredRole(h, models.RoleUser)

	// ---------------------------
	// 7. Роутинг с Gorilla Mux
	// ---------------------------
	r := mux.NewRouter()

	// --- Статические файлы (CSS, JS, Images) ---
	// Будет раздавать файлы из папки "./static" по URL "/static/..."
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))

	// --- Публичные маршруты ---
	r.HandleFunc("/", h.HandleMain).Methods("GET")
	r.HandleFunc("/auth/google/login", h.HandleGoogleLogin).Methods("GET")
	r.HandleFunc("/auth/google/callback", h.HandleGoogleCallback).Methods("GET")
	r.HandleFunc("/logout", h.HandleLogout).Methods("GET", "POST")

	// --- Защищенные маршруты пользователя ---
	// (Можно добавить middleware.AuthRequired, если есть)
	r.HandleFunc("/personal", h.HandleProfile).Methods("GET")

	// --- АДМИН ПАНЕЛЬ (Страницы HTML) ---
	// Используем adminMiddleware для защиты
	r.HandleFunc("/admin/dashboard", adminMiddleware(adminService.HandleAdminPage)).Methods("GET")
	r.HandleFunc("/admin/reports", adminMiddleware(adminService.HandleReportPage)).Methods("GET")
	r.HandleFunc("/admin/courses", adminMiddleware(adminService.HandleCoursePage)).Methods("GET")
	r.HandleFunc("/admin/users", adminMiddleware(adminService.HandleUsersPage)).Methods("GET")

	// --- АДМИН API (JSON для JS фронтенда) ---

	// ... API Курсов (старое) ...
	r.HandleFunc("/api/courses", adminMiddleware(adminService.HandleCoursesAPI)).Methods("GET", "POST")
	r.HandleFunc("/api/courses/{id}", adminMiddleware(adminService.HandleCourseByIDAPI)).Methods("GET", "PUT", "DELETE")

	// --- API Модулей ---
	r.HandleFunc("/api/modules", adminMiddleware(adminService.CreateModuleAPI)).Methods("POST")
	r.HandleFunc("/api/modules/{id}", adminMiddleware(adminService.UpdateModuleAPI)).Methods("PUT")
	r.HandleFunc("/api/modules/{id}", adminMiddleware(adminService.DeleteModuleAPI)).Methods("DELETE")

	// --- API Уроков ---
	r.HandleFunc("/api/lessons", adminMiddleware(adminService.CreateLessonAPI)).Methods("POST")
	r.HandleFunc("/api/lessons/{id}", adminMiddleware(adminService.UpdateLessonAPI)).Methods("PUT")
	r.HandleFunc("/api/lessons/{id}", adminMiddleware(adminService.DeleteLessonAPI)).Methods("DELETE")
	r.HandleFunc("/api/lessons/{id}", adminMiddleware(adminService.GetLessonAPI)).Methods("GET")
	r.HandleFunc("/api/lessons/{id}/content", adminMiddleware(adminService.UpdateLessonContentAPI)).Methods("PUT")

	// --- Страница HTML ---
	r.HandleFunc("/admin/enrollments", adminMiddleware(adminService.HandleEnrollmentsPage)).Methods("GET")

	// --- API JSON ---
	r.HandleFunc("/api/admin/enrollments", adminMiddleware(adminService.GetEnrollmentsAPI)).Methods("GET")
	r.HandleFunc("/api/admin/enrollments/{id}", adminMiddleware(adminService.UpdateEnrollmentStatusAPI)).Methods("PUT")

	// --- API info для курса

	r.HandleFunc("/api/courses/{id}/structure", adminService.GetCourseStructure).Methods("GET")
	r.HandleFunc("/api/enroll", userMiddleware(adminService.SubmitEnrollment)).Methods("POST")
	// ---------------------------
	// 8. Запуск сервера
	// ---------------------------
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	corsHandler := corsMiddleware(r)
	fmt.Printf("Сервер запущен: http://localhost:%s\n", port)
	// ВАЖНО: Передаем `r` (наш роутер), а не `nil`
	log.Fatal(http.ListenAndServe(":"+port, corsHandler))
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Разрешаем запросы с любого источника (для разработки)
		// В продакшене лучше ставить конкретный домен, например "http://mysite.com"
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		// Если браузер спрашивает "можно ли?", отвечаем "можно" и выходим
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
