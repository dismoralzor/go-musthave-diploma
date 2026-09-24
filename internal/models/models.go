// Package models содержит доменные модели и DTO для JSON-запросов/ответов
// сервиса gophermart.
package models

// User — учётная запись пользователя.
type User struct {
	ID           int64
	Login        string
	PasswordHash string
}

// Credentials — тело запроса на регистрацию или аутентификацию
// пользователя: оба запроса принимают одинаковую пару логин/пароль.
type Credentials struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}
