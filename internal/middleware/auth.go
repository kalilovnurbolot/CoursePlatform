package middleware

import (
	"net/http"

	"github.com/s/onlineCourse/internal/handlers"
	"github.com/s/onlineCourse/internal/models"
)

// RequiredRole создает Middleware, требующее определенного RoleID.
// Теперь он принимает требуемый RoleID как int.
func RequiredRole(h *handlers.Handler, requiredRoleID uint) func(next http.HandlerFunc) http.HandlerFunc {
	// Возвращаем функцию-обертку, которая принимает следующий обработчик (next)
	return func(next http.HandlerFunc) http.HandlerFunc {
		// Возвращаем сам Middleware-обработчик
		return func(w http.ResponseWriter, r *http.Request) {

			// 1. Проверка Аутентификации
			userID, isAuthenticated := h.GetAuthenticatedUserID(r)

			if !isAuthenticated {
				// Перенаправление неаутентифицированных пользователей
				http.Redirect(w, r, "/", http.StatusSeeOther)
				return
			}

			// 2. Получение данных пользователя для проверки Роли
			var user models.User
			if err := h.DB.First(&user, userID).Error; err != nil {
				// Если пользователь не найден в БД, это проблема авторизации
				http.Error(w, "User not found or database error", http.StatusUnauthorized)
				return
			}

			// 3. Динамическая Проверка RoleID
			if user.RoleID != requiredRoleID {
				// Если роль пользователя не соответствует требуемой
				//http.Error(w, "Access Denied: Insufficient permissions", http.StatusForbidden)
				//return

				h.HandleForbiddenPage(w, r)
				return
			}

			// 4. Если все проверки пройдены, вызываем следующий обработчик
			next.ServeHTTP(w, r)
		}
	}
}
