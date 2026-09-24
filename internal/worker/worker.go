// Package worker реализует фоновый опрос системы начислений и обновление
// статусов заказов: по тикеру забирается пачка заказов, атомарно
// помеченных в хранилище как взятые в обработку, и раздаётся пулу воркеров
// на errgroup.Group с ограничением на число одновременных горутин.
package worker

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/dismoralzor/go-musthave-diploma/internal/accrual"
	"github.com/dismoralzor/go-musthave-diploma/internal/models"
)

// claimBatchSize — сколько заказов забирается из хранилища за один тик.
// Значение выбрано с запасом относительно типичного numWorkers, чтобы пул
// воркеров не простаивал в ожидании следующего тика.
const claimBatchSize = 50

// Store описывает доступ к заказам, необходимый воркеру.
type Store interface {
	ClaimOrdersForProcessing(ctx context.Context, limit int) ([]models.Order, error)
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
// обрабатывающий заказы пулом не более чем из numWorkers одновременных
// горутин через client.
func New(store Store, client AccrualClient, log *zap.Logger, numWorkers int, pollInterval time.Duration) *Worker {
	return &Worker{
		store:        store,
		client:       client,
		log:          log,
		numWorkers:   numWorkers,
		pollInterval: pollInterval,
	}
}

// Run по тикеру забирает пачку заказов и обрабатывает их пулом не более чем
// из numWorkers одновременных горутин; g.Go блокируется при достижении
// лимита, что даёт естественный backpressure. Останавливается по отмене ctx
// и дожидается завершения уже начатых обращений к accrual.
func (w *Worker) Run(ctx context.Context) {
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(w.numWorkers)

	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

loop:
	for {
		select {
		case <-gctx.Done():
			break loop
		case <-ticker.C:
			orders, err := w.store.ClaimOrdersForProcessing(gctx, claimBatchSize)
			if err != nil {
				w.log.Error("claim orders for processing", zap.Error(err))
				continue
			}

			for _, o := range orders {
				g.Go(func() error {
					w.processOrder(gctx, o)
					return nil
				})
			}
		}
	}

	_ = g.Wait()
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
