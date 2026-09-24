// Package accrual реализует HTTP-клиент к внешней системе расчёта
// начислений баллов лояльности.
package accrual

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/dismoralzor/go-musthave-diploma/internal/models"
)

// Client — HTTP-клиент к системе расчёта начислений.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// New создаёт Client, обращающийся к системе начислений по адресу baseURL.
func New(baseURL string) *Client {
	return &Client{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
}

// Result — итог обработки заказа системой начислений.
type Result struct {
	// Status — статус заказа, приведённый к статусам gophermart
	// (models.OrderStatusProcessing/Invalid/Processed).
	Status string
	// Accrual — начисленная сумма баллов, если она известна.
	Accrual *float64
}

// ErrNotRegistered возвращается, если система начислений не знает о заказе
// с указанным номером (HTTP 204). Заказ в этом случае нужно оставить как
// есть и опросить позже.
var ErrNotRegistered = errors.New("accrual: order is not registered")

// ErrUnknownStatus возвращается, если система начислений ответила статусом,
// не входящим в известный набор (REGISTERED/PROCESSING/INVALID/PROCESSED).
// В отличие от известных статусов, такой ответ не приводится молча к
// PROCESSING: вызывающая сторона должна решить, как его обработать.
var ErrUnknownStatus = errors.New("accrual: unknown order status")

// TooManyRequestsError возвращается, если система начислений ответила
// 429 Too Many Requests, и указывает, сколько нужно подождать перед
// следующим запросом.
type TooManyRequestsError struct {
	RetryAfter time.Duration
}

// Error реализует интерфейс error.
func (e *TooManyRequestsError) Error() string {
	return fmt.Sprintf("accrual: too many requests, retry after %s", e.RetryAfter)
}

// accrualResponse — тело ответа системы начислений в формате JSON.
type accrualResponse struct {
	Order   string   `json:"order"`
	Status  string   `json:"status"`
	Accrual *float64 `json:"accrual,omitempty"`
}

// статусы, которые возвращает система начислений.
const (
	accrualStatusRegistered = "REGISTERED"
	accrualStatusProcessing = "PROCESSING"
	accrualStatusInvalid    = "INVALID"
	accrualStatusProcessed  = "PROCESSED"
)

// GetOrder запрашивает у системы начислений статус заказа number.
func (c *Client) GetOrder(ctx context.Context, number string) (Result, error) {
	url := c.baseURL + "/api/orders/" + number

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Result{}, fmt.Errorf("accrual: build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return Result{}, fmt.Errorf("accrual: do request: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		var body accrualResponse
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			return Result{}, fmt.Errorf("accrual: decode response: %w", err)
		}
		status, err := mapStatus(body.Status)
		if err != nil {
			return Result{}, fmt.Errorf("%w: %q", err, body.Status)
		}
		return Result{Status: status, Accrual: body.Accrual}, nil

	case http.StatusNoContent:
		return Result{}, ErrNotRegistered

	case http.StatusTooManyRequests:
		return Result{}, &TooManyRequestsError{RetryAfter: parseRetryAfter(resp.Header.Get("Retry-After"))}

	default:
		return Result{}, fmt.Errorf("accrual: unexpected status %d", resp.StatusCode)
	}
}

// mapStatus приводит статус, возвращённый системой начислений, к статусам
// заказов gophermart. Для нераспознанного статуса возвращает
// ErrUnknownStatus вместо того, чтобы молча считать его PROCESSING.
func mapStatus(status string) (string, error) {
	switch status {
	case accrualStatusRegistered, accrualStatusProcessing:
		return models.OrderStatusProcessing, nil
	case accrualStatusInvalid:
		return models.OrderStatusInvalid, nil
	case accrualStatusProcessed:
		return models.OrderStatusProcessed, nil
	default:
		return "", ErrUnknownStatus
	}
}

// parseRetryAfter разбирает значение заголовка Retry-After (в секундах).
// Если значение отсутствует или некорректно, возвращает одну секунду.
func parseRetryAfter(header string) time.Duration {
	seconds, err := strconv.Atoi(header)
	if err != nil || seconds < 0 {
		return time.Second
	}
	return time.Duration(seconds) * time.Second
}
