package app

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"
)

type fakeConsumeContext struct {
	closed chan struct{}

	once sync.Once

	onDrain func()
	onStop  func()
}

type eventRecorder struct {
	mu     sync.Mutex
	events []string
}

func newFakeConsumeContext() *fakeConsumeContext {
	return &fakeConsumeContext{
		closed: make(chan struct{}),
	}
}

func (f *fakeConsumeContext) Drain() {
	if f.onDrain != nil {
		f.onDrain()
	}
}

func (f *fakeConsumeContext) Stop() {
	if f.onStop != nil {
		f.onStop()
	}
}

func (f *fakeConsumeContext) Closed() <-chan struct{} {
	return f.closed
}

func (f *fakeConsumeContext) finish() {
	f.once.Do(func() {
		close(f.closed)
	})
}

func (r *eventRecorder) add(event string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.events = append(r.events, event)
}

func (r *eventRecorder) snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	return append([]string(nil), r.events...)
}

type fakeNATSConnection struct {
	events   *eventRecorder
	drainErr error
}

func (f *fakeNATSConnection) Drain() error {
	f.events.add("nats.drain")
	return f.drainErr
}

func (f *fakeNATSConnection) Close() {
	f.events.add("nats.close")
}

func TestStopConsumerDrainWaitsForActiveCallback(t *testing.T) {
	consumer := newFakeConsumeContext()

	drainStarted := make(chan struct{})
	callbackFinished := make(chan struct{})

	consumer.onDrain = func() {
		close(drainStarted)
	}

	go func() {
		<-drainStarted

		time.Sleep(20 * time.Millisecond)

		consumer.finish()
		close(callbackFinished)
	}()

	workerCtx, workerCancel := context.WithCancel(
		context.Background(),
	)

	a := &Application{
		consumer:         consumer,
		workerCancel:     workerCancel,
		consumerStopWait: 100 * time.Millisecond,
	}

	err := a.stopConsumer(context.Background(), true)
	if err != nil {
		t.Fatalf("stopConsumer returned error: %v", err)
	}

	select {
	case <-callbackFinished:
	default:
		t.Fatal("consumer returned before callback finished")
	}

	select {
	case <-workerCtx.Done():
	default:
		t.Fatal("worker context was not canceled")
	}
}

func TestStopConsumerForcedCancelsWorker(t *testing.T) {
	consumer := newFakeConsumeContext()

	stopCalled := false

	consumer.onStop = func() {
		stopCalled = true
		consumer.finish()
	}

	workerCtx, workerCancel := context.WithCancel(
		context.Background(),
	)

	a := &Application{
		consumer:     consumer,
		workerCancel: workerCancel,
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		time.Second,
	)
	defer cancel()

	if err := a.stopConsumer(ctx, false); err != nil {
		t.Fatalf("stopConsumer returned error: %v", err)
	}

	if !stopCalled {
		t.Fatal("Consumer Stop was not called")
	}

	select {
	case <-workerCtx.Done():
	default:
		t.Fatal("worker context was not canceled")
	}
}

func TestShutdownClosesResourcesAfterConsumer(t *testing.T) {
	events := &eventRecorder{}
	consumer := newFakeConsumeContext()

	consumer.onDrain = func() {
		events.add("consumer.drain")
		consumer.finish()
	}

	natsConn := &fakeNATSConnection{
		events: events,
	}

	a := &Application{
		consumer: consumer,
		nats:     natsConn,

		httpShutdown: func(context.Context) error {
			events.add("http.shutdown")
			return nil
		},

		closeStorageFn: func() error {
			events.add("storage.close")
			return nil
		},
	}

	err := a.Shutdown(context.Background())
	if err != nil {
		t.Fatalf("Shutdown returned error: %v", err)
	}

	expected := []string{
		"http.shutdown",
		"consumer.drain",
		"nats.drain",
		"storage.close",
	}

	actual := events.snapshot()

	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf(
			"unexpected shutdown order\nexpected: %v\nactual:   %v",
			expected,
			actual,
		)
	}
}

func TestShutdownDoesNotCloseResourcesWhileConsumerRunning(
	t *testing.T,
) {
	events := &eventRecorder{}
	consumer := newFakeConsumeContext()

	consumer.onDrain = func() {
		events.add("consumer.drain")
		// Intentionally do not close Consumer.
	}

	consumer.onStop = func() {
		events.add("consumer.stop")
		// Simulate a callback that does not exit in time.
	}

	workerCtx, workerCancel := context.WithCancel(
		context.Background(),
	)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		time.Second,
	)
	defer cancel()

	a := &Application{
		consumer:     consumer,
		workerCancel: workerCancel,

		// Short timeout for this test only.
		consumerStopWait: 10 * time.Millisecond,

		httpShutdown: func(context.Context) error {
			events.add("http.shutdown")
			cancel()
			return nil
		},

		nats: &fakeNATSConnection{
			events: events,
		},

		closeStorageFn: func() error {
			events.add("storage.close")
			return nil
		},
	}

	err := a.Shutdown(ctx)

	if err == nil {
		t.Fatal("expected shutdown error")
	}

	if !errors.Is(err, context.Canceled) &&
		!errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf(
			"expected context cancellation or deadline, got: %v",
			err,
		)
	}

	if a.consumerStopped() {
		t.Fatal("Consumer should still be running")
	}

	select {
	case <-workerCtx.Done():
	default:
		t.Fatal("worker context was not canceled")
	}

	expected := []string{
		"http.shutdown",
		"consumer.drain",
		"consumer.stop",
	}

	actual := events.snapshot()

	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf(
			"unexpected events\nexpected: %v\nactual:   %v",
			expected,
			actual,
		)
	}
}

func TestCloseIsIdempotent(t *testing.T) {
	events := &eventRecorder{}
	consumer := newFakeConsumeContext()

	consumer.onStop = func() {
		events.add("consumer.stop")
		consumer.finish()
	}

	a := &Application{
		consumer: consumer,

		nats: &fakeNATSConnection{
			events: events,
		},

		closeStorageFn: func() error {
			events.add("storage.close")
			return nil
		},
	}

	if err := a.Close(); err != nil {
		t.Fatalf("first Close returned error: %v", err)
	}

	if err := a.Close(); err != nil {
		t.Fatalf("second Close returned error: %v", err)
	}

	expected := []string{
		"consumer.stop",
		"nats.close",
		"storage.close",
	}

	actual := events.snapshot()

	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf(
			"unexpected close sequence\nexpected: %v\nactual:   %v",
			expected,
			actual,
		)
	}
}
