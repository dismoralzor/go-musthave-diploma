package auth

import (
	"context"
	"net/http"
	"strings"
)

// contextKey — приватный тип ключей контекста пакета auth, чтобы избежать
// коллизий с ключами других пакетов.
type contextKey int

// userIDContextKey — ключ контекста для идентификатора аутентифицированного
// пользователя.
const userIDContextKey contextKey = iota

// cookieName — имя cookie, в которой также может передаваться токен.
const cookieName = "token"

// Middleware проверяет наличие и валидность JWT в запросе (заголовок
// Authorization в формате "Bearer <token>" или cookie "token") и кладёт
// идентификатор пользователя в контекст запроса. При отсутствии или
// невалидности токена отвечает 401 Unauthorized.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := extractToken(r)
		if token == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		userID, err := ParseToken(token)
		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), userIDContextKey, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// extractToken достаёт JWT из заголовка Authorization или, если его нет, из
// cookie "token". Возвращает пустую строку, если токен не найден.
func extractToken(r *http.Request) string {
	if h := r.Header.Get("Authorization"); h != "" {
		const prefix = "Bearer "
		if strings.HasPrefix(h, prefix) {
			return strings.TrimPrefix(h, prefix)
		}
	}

	if c, err := r.Cookie(cookieName); err == nil {
		return c.Value
	}

	return ""
}

// UserIDFromContext возвращает идентификатор пользователя, положенный в
// контекст middleware Middleware, и признак его наличия.
func UserIDFromContext(ctx context.Context) (int64, bool) {
	userID, ok := ctx.Value(userIDContextKey).(int64)
	return userID, ok
}
