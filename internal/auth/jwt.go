package auth

import (
	"crypto/rand"
	"errors"
	"os"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/dismoralzor/go-musthave-diploma/internal/logger"
)

// tokenTTL — срок жизни выданного JWT.
const tokenTTL = 24 * time.Hour

// randomSecretSize — длина случайного секрета в байтах, генерируемого,
// когда SECRET_KEY не задан.
const randomSecretSize = 32

var (
	secretOnce  sync.Once
	secretValue []byte
	secretErr   error
)

// secretKey возвращает секрет подписи JWT: значение переменной окружения
// SECRET_KEY (читается заново при каждом вызове, чтобы её можно было менять
// в тестах через t.Setenv), а если она не задана — случайный секрет,
// сгенерированный один раз при первом обращении и закэшированный на всё
// время жизни процесса. Случайный секрет не переживает перезапуск —
// выданные им токены после рестарта станут невалидными, но старт сервиса
// не должен падать из-за отсутствия SECRET_KEY.
func secretKey() ([]byte, error) {
	if v := os.Getenv("SECRET_KEY"); v != "" {
		return []byte(v), nil
	}

	secretOnce.Do(func() {
		b := make([]byte, randomSecretSize)
		if _, err := rand.Read(b); err != nil {
			secretErr = err
			return
		}
		secretValue = b
		logger.Log.Warn("SECRET_KEY не задан, использован случайный секрет, токены не переживут перезапуск")
	})

	return secretValue, secretErr
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
	key, err := secretKey()
	if err != nil {
		return "", err
	}

	claims := Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(tokenTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(key)
}

// ParseToken проверяет подпись и срок действия токена tokenString и
// возвращает идентификатор пользователя, зашифрованный в нём.
func ParseToken(tokenString string) (int64, error) {
	key, err := secretKey()
	if err != nil {
		return 0, err
	}

	claims := &Claims{}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return key, nil
	})
	if err != nil || !token.Valid {
		return 0, ErrInvalidToken
	}

	return claims.UserID, nil
}
