package auth

import (
	"errors"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// tokenTTL — срок жизни выданного JWT.
const tokenTTL = 24 * time.Hour

// defaultSecretKey — секрет подписи JWT по умолчанию, используемый, если не
// задана переменная окружения SECRET_KEY. Для боевого окружения секрет
// следует переопределять через SECRET_KEY.
const defaultSecretKey = "gophermart-default-secret-key"

// secretKey возвращает секрет подписи JWT: значение SECRET_KEY, если оно
// задано, иначе defaultSecretKey.
func secretKey() []byte {
	if v := os.Getenv("SECRET_KEY"); v != "" {
		return []byte(v)
	}
	return []byte(defaultSecretKey)
}

// ErrInvalidToken возвращается, если токен не прошёл проверку подписи,
// истёк или имеет некорректный формат.
var ErrInvalidToken = errors.New("invalid token")

// Claims — данные, зашифрованные в JWT.
type Claims struct {
	UserID int64 `json:"user_id"`
	jwt.RegisteredClaims
}

// GenerateToken выпускает подписанный JWT для пользователя userID со сроком
// действия tokenTTL.
func GenerateToken(userID int64) (string, error) {
	claims := Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(tokenTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(secretKey())
}

// ParseToken проверяет подпись и срок действия токена tokenString и
// возвращает идентификатор пользователя, зашифрованный в нём.
func ParseToken(tokenString string) (int64, error) {
	claims := &Claims{}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return secretKey(), nil
	})
	if err != nil || !token.Valid {
		return 0, ErrInvalidToken
	}

	return claims.UserID, nil
}
