package admin

import (
	"net/http"

	"github.com/s/onlineCourse/internal/handlers"
	"github.com/s/onlineCourse/internal/models"
)

type Service struct {
	handlers.Handler
}

func (serv Service) HandleAdminPage(w http.ResponseWriter, r *http.Request) {
	// 👇 Проверка ошибки при получении сессии
	session, err := serv.Store.Get(r, "session")
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
	if err := serv.DB.Preload("Role").First(&user, userID).Error; err != nil {
		// Если пользователь с ID из сессии не найден, это тоже проблема
		http.Error(w, "Пользователь не найден", http.StatusNotFound)
		return
	}

	// Если пользователь найден, но он не админ (RoleID не 2), перенаправляем его
	if user.RoleID != 2 {
		http.Redirect(w, r, "/", http.StatusForbidden)
		return
	}

	data := handlers.PageData{
		Title:           "Панель Администратора",
		IsAuthenticated: userID != 0,
		UserID:          userID,
		RoleID:          user.RoleID,
		UserName:        user.Name,
		UserPictureURL:  user.Picture,
		CurrentPath:     r.URL.Path,
	}

	serv.Tmpl.ExecuteTemplate(w, "adminIndex", data)
}
func (serv Service) HandleUsersPage(w http.ResponseWriter, r *http.Request) {
	// 👇 Проверка ошибки при получении сессии
	session, err := serv.Store.Get(r, "session")
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
	if err := serv.DB.Preload("Role").First(&user, userID).Error; err != nil {
		// Если пользователь с ID из сессии не найден, это тоже проблема
		http.Error(w, "Пользователь не найден", http.StatusNotFound)
		return
	}

	// Если пользователь найден, но он не админ (RoleID не 2), перенаправляем его
	if user.RoleID != 2 {
		http.Redirect(w, r, "/", http.StatusForbidden)
		return
	}

	data := handlers.PageData{
		Title:           "Панель Администратора",
		IsAuthenticated: userID != 0,
		UserID:          userID,
		RoleID:          user.RoleID,
		UserName:        user.Name,
		UserPictureURL:  user.Picture,
		CurrentPath:     r.URL.Path,
	}

	serv.Tmpl.ExecuteTemplate(w, "adminUsers", data)
}

func (serv Service) HandleCoursePage(w http.ResponseWriter, r *http.Request) {
	// 👇 Проверка ошибки при получении сессии
	session, err := serv.Store.Get(r, "session")
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
	if err := serv.DB.Preload("Role").First(&user, userID).Error; err != nil {
		// Если пользователь с ID из сессии не найден, это тоже проблема
		http.Error(w, "Пользователь не найден", http.StatusNotFound)
		return
	}

	// Если пользователь найден, но он не админ (RoleID не 2), перенаправляем его
	if user.RoleID != 2 {
		http.Redirect(w, r, "/", http.StatusForbidden)
		return
	}
	data := handlers.PageData{
		Title:           "Панель Администратора",
		IsAuthenticated: userID != 0,
		UserID:          userID,
		RoleID:          user.RoleID,
		UserName:        user.Name,
		UserPictureURL:  user.Picture,
		CurrentPath:     r.URL.Path,
	}

	serv.Tmpl.ExecuteTemplate(w, "adminCourse", data)
}

func (serv Service) HandleReportPage(w http.ResponseWriter, r *http.Request) {
	// 👇 Проверка ошибки при получении сессии
	session, err := serv.Store.Get(r, "session")
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
	if err := serv.DB.Preload("Role").First(&user, userID).Error; err != nil {
		// Если пользователь с ID из сессии не найден, это тоже проблема
		http.Error(w, "Пользователь не найден", http.StatusNotFound)
		return
	}

	// Если пользователь найден, но он не админ (RoleID не 2), перенаправляем его
	if user.RoleID != 2 {
		http.Redirect(w, r, "/", http.StatusForbidden)
		return
	}

	data := handlers.PageData{
		Title:           "Панель Администратора",
		IsAuthenticated: userID != 0,
		UserID:          userID,
		RoleID:          user.RoleID,
		UserName:        user.Name,
		UserPictureURL:  user.Picture,
		CurrentPath:     r.URL.Path,
	}

	serv.Tmpl.ExecuteTemplate(w, "adminReport", data)
}
