// Package worker реализует фоновый опрос системы начислений и обновление
// статусов заказов: одна горутина-fetcher опрашивает хранилище и
// раздаёт заказы через канал пулу воркеров, которые обращаются к системе
// начислений.
package worker

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	"github.com/dismoralzor/go-musthave-diploma/internal/accrual"
	"github.com/dismoralzor/go-musthave-diploma/internal/models"
)

// Store описывает доступ к заказам, необходимый воркеру.
type Store interface {
	GetOrdersForProcessing(ctx context.Context) ([]models.Order, error)
	UpdateOrderStatus(ctx context.Context, number, status string, accrual *float64, userID int64) error
}

// AccrualClient описывает обращение к системе начислений, необходимое
// воркеру.
type AccrualClient interface {
	GetOrder(ctx context.Context, number string) (accrual.Result, error)
}

// Worker опрашивает хранилище на предмет необработанных заказов и обновляет
// их статусы по данным системы начислений.
type Worker struct {
	store        Store
	client       AccrualClient
	log          *zap.Logger
	numWorkers   int
	pollInterval time.Duration

	// pauseUntil — unix-время (наносекунды), до которого все воркеры
	// приостанавливают обращения к системе начислений из-за ответа 429.
	// Общий для всех воркеров, поэтому хранится как atomic.Int64.
	pauseUntil atomic.Int64
}

// New создаёт Worker, опрашивающий store с интервалом pollInterval и
// обрабатывающий заказы пулом из numWorkers горутин через client.
func New(store Store, client AccrualClient, log *zap.Logger, numWorkers int, pollInterval time.Duration) *Worker {
	return &Worker{
		store:        store,
		client:       client,
		log:          log,
		numWorkers:   numWorkers,
		pollInterval: pollInterval,
	}
}

// Run запускает fetcher и пул воркеров и блокируется до их завершения.
// Останавливается по отмене ctx.
func (w *Worker) Run(ctx context.Context) {
	jobs := make(chan models.Order)

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		w.fetch(ctx, jobs)
	}()

	for i := 0; i < w.numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w.process(ctx, jobs)
		}()
	}

	wg.Wait()
}

// fetch по тикеру запрашивает у хранилища необработанные заказы и
// отправляет их в jobs. Завершается по отмене ctx, закрывая jobs, чтобы
// воркеры тоже завершились.
func (w *Worker) fetch(ctx context.Context, jobs chan<- models.Order) {
	defer close(jobs)

	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			orders, err := w.store.GetOrdersForProcessing(ctx)
			if err != nil {
				w.log.Error("fetch orders for processing", zap.Error(err))
				continue
			}

			for _, o := range orders {
				select {
				case jobs <- o:
				case <-ctx.Done():
					return
				}
			}
		}
	}
}

// process читает заказы из jobs и опрашивает по ним систему начислений,
// пока канал не закроется или не отменится ctx.
func (w *Worker) process(ctx context.Context, jobs <-chan models.Order) {
	for {
		select {
		case <-ctx.Done():
			return
		case o, ok := <-jobs:
			if !ok {
				return
			}
			w.processOrder(ctx, o)
		}
	}
}

// processOrder опрашивает систему начислений по одному заказу и обновляет
// его статус в хранилище.
func (w *Worker) processOrder(ctx context.Context, o models.Order) {
	if !w.waitForGate(ctx) {
		return
	}

	result, err := w.client.GetOrder(ctx, o.Number)
	if err != nil {
		var tooMany *accrual.TooManyRequestsError
		switch {
		case errors.As(err, &tooMany):
			w.pauseUntil.Store(time.Now().Add(tooMany.RetryAfter).UnixNano())
		case errors.Is(err, accrual.ErrNotRegistered):
			// Система начислений пока не знает об этом заказе — оставляем
			// как есть, следующий тик fetcher вернёт его снова.
		default:
			w.log.Error("get order from accrual", zap.String("order", o.Number), zap.Error(err))
		}
		return
	}

	if err := w.store.UpdateOrderStatus(ctx, o.Number, result.Status, result.Accrual, o.UserID); err != nil {
		w.log.Error("update order status", zap.String("order", o.Number), zap.Error(err))
	}
}

// waitForGate ждёт, пока не истечёт общая пауза, выставленная после ответа
// 429 от системы начислений. Возвращает false, если ожидание было прервано
// отменой ctx.
func (w *Worker) waitForGate(ctx context.Context) bool {
	until := time.Unix(0, w.pauseUntil.Load())

	d := time.Until(until)
	if d <= 0 {
		return true
	}

	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}
