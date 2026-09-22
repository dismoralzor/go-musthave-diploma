// Package auth отвечает за хеширование паролей, выпуск и проверку JWT, а
// также за middleware аутентификации HTTP-запросов.
package auth

import "golang.org/x/crypto/bcrypt"

// Hash возвращает bcrypt-хеш пароля password.
func Hash(password string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// Check сообщает, соответствует ли пароль password ранее вычисленному
// bcrypt-хешу hash.
func Check(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
