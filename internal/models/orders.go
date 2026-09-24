package models

import "time"

// Статусы обработки заказа.
const (
	// OrderStatusNew — заказ загружен, но ещё не отправлен в обработку.
	OrderStatusNew = "NEW"
	// OrderStatusProcessing — заказ обрабатывается системой начислений.
	OrderStatusProcessing = "PROCESSING"
	// OrderStatusInvalid — система начислений отказала в начислении.
	OrderStatusInvalid = "INVALID"
	// OrderStatusProcessed — начисление успешно рассчитано.
	OrderStatusProcessed = "PROCESSED"
)

// Order — заказ, загруженный пользователем для начисления баллов лояльности.
type Order struct {
	Number     string
	Status     string
	Accrual    *float64
	UploadedAt time.Time
	UserID     int64
}

// OrderResponse — представление заказа в ответе API.
type OrderResponse struct {
	Number     string    `json:"number"`
	Status     string    `json:"status"`
	Accrual    *float64  `json:"accrual,omitempty"`
	UploadedAt time.Time `json:"uploaded_at"`
}
