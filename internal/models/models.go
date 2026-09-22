// Package models содержит доменные модели и DTO для JSON-запросов/ответов
// сервиса gophermart.
package models

// User — учётная запись пользователя.
type User struct {
	ID           int64
	Login        string
	PasswordHash string
}

// RegisterRequest — тело запроса на регистрацию пользователя.
type RegisterRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

// LoginRequest — тело запроса на аутентификацию пользователя.
type LoginRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}
