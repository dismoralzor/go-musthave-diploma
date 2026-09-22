package worker

import (
	"context"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/dismoralzor/go-musthave-diploma/internal/accrual"
	"github.com/dismoralzor/go-musthave-diploma/internal/models"
)

// fakeStore — тестовая реализация Store, потокобезопасно накапливающая
// вызовы UpdateOrderStatus.
type fakeStore struct {
	mu      sync.Mutex
	orders  []models.Order
	updates []update
}

type update struct {
	number  string
	status  string
	accrual *float64
	userID  int64
}

func (s *fakeStore) GetOrdersForProcessing(ctx context.Context) ([]models.Order, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.orders, nil
}

func (s *fakeStore) UpdateOrderStatus(ctx context.Context, number, status string, accrual *float64, userID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.updates = append(s.updates, update{number: number, status: status, accrual: accrual, userID: userID})
	return nil
}

func (s *fakeStore) snapshot() []update {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]update, len(s.updates))
	copy(out, s.updates)
	return out
}

// fakeClient — тестовая реализация AccrualClient с заранее заданным
// результатом или ошибкой.
type fakeClient struct {
	result accrual.Result
	err    error
}

func (c *fakeClient) GetOrder(ctx context.Context, number string) (accrual.Result, error) {
	return c.result, c.err
}

func floatPtr(v float64) *float64 { return &v }

func TestProcessOrder_Success(t *testing.T) {
	store := &fakeStore{}
	client := &fakeClient{result: accrual.Result{Status: models.OrderStatusProcessed, Accrual: floatPtr(700)}}

	w := New(store, client, zap.NewNop(), 1, time.Second)
	w.processOrder(context.Background(), models.Order{Number: "123", UserID: 42})

	updates := store.snapshot()
	if len(updates) != 1 {
		t.Fatalf("got %d updates, want 1", len(updates))
	}
	if updates[0].number != "123" || updates[0].status != models.OrderStatusProcessed || updates[0].userID != 42 {
		t.Errorf("unexpected update: %+v", updates[0])
	}
	if updates[0].accrual == nil || *updates[0].accrual != 700 {
		t.Errorf("accrual = %v, want 700", updates[0].accrual)
	}
}

func TestProcessOrder_NotRegistered(t *testing.T) {
	store := &fakeStore{}
	client := &fakeClient{err: accrual.ErrNotRegistered}

	w := New(store, client, zap.NewNop(), 1, time.Second)
	w.processOrder(context.Background(), models.Order{Number: "123", UserID: 42})

	if len(store.snapshot()) != 0 {
		t.Errorf("expected no updates for unregistered order, got %d", len(store.snapshot()))
	}
}

func TestProcessOrder_TooManyRequests_SetsGate(t *testing.T) {
	store := &fakeStore{}
	client := &fakeClient{err: &accrual.TooManyRequestsError{RetryAfter: 50 * time.Millisecond}}

	w := New(store, client, zap.NewNop(), 1, time.Second)
	before := time.Now()
	w.processOrder(context.Background(), models.Order{Number: "123", UserID: 42})

	if len(store.snapshot()) != 0 {
		t.Errorf("expected no updates on 429, got %d", len(store.snapshot()))
	}

	pauseUntil := time.Unix(0, w.pauseUntil.Load())
	if !pauseUntil.After(before) {
		t.Errorf("pauseUntil = %v, want after %v", pauseUntil, before)
	}
}

func TestWaitForGate(t *testing.T) {
	w := New(&fakeStore{}, &fakeClient{}, zap.NewNop(), 1, time.Second)

	// Без выставленной паузы waitForGate не должен блокироваться.
	if !w.waitForGate(context.Background()) {
		t.Error("waitForGate() = false without pause, want true")
	}

	// С выставленной короткой паузой должен дождаться её истечения.
	w.pauseUntil.Store(time.Now().Add(30 * time.Millisecond).UnixNano())
	start := time.Now()
	if !w.waitForGate(context.Background()) {
		t.Error("waitForGate() = false, want true after pause elapses")
	}
	if time.Since(start) < 20*time.Millisecond {
		t.Error("waitForGate() returned before the pause elapsed")
	}

	// Отмена контекста во время ожидания должна прервать ожидание.
	w.pauseUntil.Store(time.Now().Add(time.Hour).UnixNano())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if w.waitForGate(ctx) {
		t.Error("waitForGate() = true with cancelled context, want false")
	}
}

func TestRun_ProcessesOrdersUntilCancelled(t *testing.T) {
	store := &fakeStore{orders: []models.Order{{Number: "123", UserID: 1, Status: models.OrderStatusNew}}}
	client := &fakeClient{result: accrual.Result{Status: models.OrderStatusProcessed, Accrual: floatPtr(100)}}

	w := New(store, client, zap.NewNop(), 2, 10*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		w.Run(ctx)
		close(done)
	}()

	// Даём воркеру время хотя бы на один цикл опроса.
	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run() did not return after context cancellation")
	}

	if len(store.snapshot()) == 0 {
		t.Error("expected at least one UpdateOrderStatus call")
	}
}
