// Package logger настраивает глобальный структурный логгер сервиса на базе
// zap и предоставляет HTTP-middleware для логирования запросов.
package logger

import (
	"net/http"
	"time"

	"go.uber.org/zap"
)

// Log — глобальный логгер пакета. По умолчанию равен zap.NewNop(), чтобы
// использование пакета до вызова Initialize не приводило к панике на nil.
var Log = zap.NewNop()

// Initialize создаёт логгер с заданным уровнем логирования (например,
// "debug", "info", "warn", "error") и сохраняет его в Log.
func Initialize(level string) error {
	lvl, err := zap.ParseAtomicLevel(level)
	if err != nil {
		return err
	}

	cfg := zap.NewProductionConfig()
	cfg.Level = lvl

	zl, err := cfg.Build()
	if err != nil {
		return err
	}

	Log = zl
	return nil
}

// responseWriter оборачивает http.ResponseWriter, чтобы перехватить код
// статуса и размер тела ответа для логирования.
type responseWriter struct {
	http.ResponseWriter
	status int
	size   int
}

// WriteHeader запоминает код статуса и делегирует вызов оригинальному
// http.ResponseWriter.
func (w *responseWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

// Write накапливает размер записанного тела ответа и делегирует вызов
// оригинальному http.ResponseWriter.
func (w *responseWriter) Write(b []byte) (int, error) {
	n, err := w.ResponseWriter.Write(b)
	w.size += n
	return n, err
}

// RequestLogger — middleware, логирующее метод, URI, код статуса, размер
// ответа и длительность обработки каждого HTTP-запроса.
func RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r)

		Log.Info("request",
			zap.String("method", r.Method),
			zap.String("uri", r.RequestURI),
			zap.Int("status", rw.status),
			zap.Int("size", rw.size),
			zap.Duration("duration", time.Since(start)),
		)
	})
}
