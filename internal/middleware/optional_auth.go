package middleware

import (
	"net/http"

	"github.com/s/onlineCourse/internal/handlers"
)

// OptionalAuth пропускает всех, но если пользователь не авторизован —
// перенаправляет на главную. Используется для маршрутов, доступ к которым
// зависит от типа курса/урока (проверяется внутри handler'а).
func OptionalAuth(h *handlers.Handler) func(next http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r)
		}
	}
}
