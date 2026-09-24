package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/dismoralzor/go-musthave-diploma/internal/auth"
	"github.com/dismoralzor/go-musthave-diploma/internal/models"
	"github.com/dismoralzor/go-musthave-diploma/internal/storage"
)

// setAuthToken отдаёт токен клиенту двумя способами: заголовком
// Authorization ("Bearer <token>") и cookie "token", чтобы подходить как
// клиентам на заголовке, так и клиентам на cookie.
func setAuthToken(w http.ResponseWriter, token string) {
	w.Header().Set("Authorization", "Bearer "+token)
	http.SetCookie(w, &http.Cookie{
		Name:  "token",
		Value: token,
		Path:  "/",
	})
}

// Register обрабатывает регистрацию пользователя: создаёт запись в
// хранилище и, в случае успеха, выдаёт JWT.
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req models.Credentials
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if req.Login == "" || req.Password == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	hash, err := auth.Hash(req.Password)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	userID, err := h.users.CreateUser(r.Context(), req.Login, hash)
	if err != nil {
		if errors.Is(err, storage.ErrLoginTaken) {
			w.WriteHeader(http.StatusConflict)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	token, err := auth.GenerateToken(userID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	setAuthToken(w, token)
	w.WriteHeader(http.StatusOK)
}

// Login обрабатывает аутентификацию пользователя по логину и паролю и, в
// случае успеха, выдаёт JWT.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req models.Credentials
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if req.Login == "" || req.Password == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	user, err := h.users.GetUserByLogin(r.Context(), req.Login)
	if err != nil {
		if errors.Is(err, storage.ErrUserNotFound) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if !auth.Check(user.PasswordHash, req.Password) {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	token, err := auth.GenerateToken(user.ID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	setAuthToken(w, token)
	w.WriteHeader(http.StatusOK)
}
