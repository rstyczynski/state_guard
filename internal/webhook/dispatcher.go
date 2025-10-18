package webhook

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/rstyczynski/fsm_v2/internal/fsm"
)

// WebhookTask represents a webhook notification task
type WebhookTask struct {
	Sink  Sink
	Event *Event
}

// Dispatcher manages webhook notifications with a worker pool
type Dispatcher struct {
	workers   int
	taskQueue chan WebhookTask
	wg        sync.WaitGroup
	ctx       context.Context
	cancel    context.CancelFunc
	assetType *fsm.AssetType
	sinks     map[string]Sink // webhook config index -> sink
	mu        sync.RWMutex
}

// NewDispatcher creates a new webhook dispatcher with a worker pool
func NewDispatcher(workers int, queueSize int, assetType *fsm.AssetType) (*Dispatcher, error) {
	if workers <= 0 {
		workers = 5 // Default to 5 workers
	}
	if queueSize <= 0 {
		queueSize = 100 // Default queue size
	}

	ctx, cancel := context.WithCancel(context.Background())

	d := &Dispatcher{
		workers:   workers,
		taskQueue: make(chan WebhookTask, queueSize),
		ctx:       ctx,
		cancel:    cancel,
		assetType: assetType,
		sinks:     make(map[string]Sink),
	}

	// Initialize sinks from asset type webhooks
	if err := d.initializeSinks(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to initialize sinks: %w", err)
	}

	// Start worker pool
	d.start()

	return d, nil
}

// initializeSinks creates sink instances for each webhook configuration
func (d *Dispatcher) initializeSinks() error {
	if d.assetType == nil {
		return nil // No webhooks configured
	}

	for i, webhookConfig := range d.assetType.Webhooks {
		key := fmt.Sprintf("webhook_%d", i)

		// Create HTTP sink
		sink := NewHTTPSink(
			webhookConfig.URL,
			webhookConfig.Method,
			webhookConfig.GetTimeout(),
			webhookConfig.Headers,
		)

		d.sinks[key] = sink
	}

	return nil
}

// start starts the worker pool
func (d *Dispatcher) start() {
	for i := 0; i < d.workers; i++ {
		d.wg.Add(1)
		go d.worker(i)
	}
}

// worker processes webhook tasks from the queue
func (d *Dispatcher) worker(id int) {
	defer d.wg.Done()

	for {
		select {
		case <-d.ctx.Done():
			return
		case task, ok := <-d.taskQueue:
			if !ok {
				return
			}
			d.processTask(id, task)
		}
	}
}

// processTask sends a webhook notification
func (d *Dispatcher) processTask(workerID int, task WebhookTask) {
	// Use background context for HTTP request so it's not affected by dispatcher shutdown
	// The timeout will still apply from the webhook configuration
	ctx, cancel := context.WithTimeout(context.Background(), task.Sink.(*HTTPSink).Timeout())
	defer cancel()

	err := task.Sink.Send(ctx, task.Event)
	if err != nil {
		// Log error but don't fail the state transition
		log.Printf("[Worker %d] Failed to send webhook to %s: %v",
			workerID, task.Sink.(*HTTPSink).URL(), err)
	}
}

// Notify sends webhook notifications for a state transition
// This is non-blocking - notifications are queued for worker pool
func (d *Dispatcher) Notify(instanceID, assetType, fromState, toState string) {
	if d.assetType == nil || len(d.assetType.Webhooks) == 0 {
		return // No webhooks configured
	}

	event := &Event{
		InstanceID: instanceID,
		AssetType:  assetType,
		FromState:  fromState,
		ToState:    toState,
		Timestamp:  time.Now(),
		Metadata:   make(map[string]string),
	}

	// Match webhooks and queue tasks
	for i, webhookConfig := range d.assetType.Webhooks {
		// Check on_exit condition
		if webhookConfig.Matches(fsm.OnExit, fromState, toState) {
			d.queueTask(i, event)
		}

		// Check on_transition condition
		if webhookConfig.Matches(fsm.OnTransition, fromState, toState) {
			d.queueTask(i, event)
		}

		// Check on_enter condition
		if webhookConfig.Matches(fsm.OnEnter, fromState, toState) {
			d.queueTask(i, event)
		}
	}
}

// queueTask adds a webhook task to the queue
func (d *Dispatcher) queueTask(webhookIndex int, event *Event) {
	key := fmt.Sprintf("webhook_%d", webhookIndex)

	d.mu.RLock()
	sink, exists := d.sinks[key]
	d.mu.RUnlock()

	if !exists {
		log.Printf("Webhook sink %s not found", key)
		return
	}

	task := WebhookTask{
		Sink:  sink,
		Event: event,
	}

	// Non-blocking send - if queue is full, log and drop
	select {
	case d.taskQueue <- task:
		log.Printf("Webhook queued: %s %s → %s", event.InstanceID, event.FromState, event.ToState)
	default:
		log.Printf("Webhook queue full, dropping notification for %s", event.InstanceID)
	}
}

// Flush waits for all pending webhooks to be sent (with timeout)
func (d *Dispatcher) Flush(timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for time.Now().Before(deadline) {
		if d.QueueLength() == 0 {
			// Queue is empty, wait a bit more for in-flight requests
			time.Sleep(50 * time.Millisecond)
			return
		}
		<-ticker.C
	}

	if d.QueueLength() > 0 {
		log.Printf("Warning: %d webhooks still pending after flush timeout", d.QueueLength())
	}
}

// Close shuts down the dispatcher and waits for all workers to finish
func (d *Dispatcher) Close() error {
	// Wait for pending webhooks to be sent
	d.Flush(2 * time.Second)

	// Signal workers to stop
	d.cancel()

	// Close task queue
	close(d.taskQueue)

	// Wait for all workers to finish with timeout
	done := make(chan struct{})
	go func() {
		d.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Workers finished
	case <-time.After(10 * time.Second):
		log.Printf("Warning: Some webhook workers did not finish in time")
	}

	// Close all sinks
	d.mu.Lock()
	defer d.mu.Unlock()

	for _, sink := range d.sinks {
		if err := sink.Close(); err != nil {
			log.Printf("Error closing sink: %v", err)
		}
	}

	return nil
}

// WorkerCount returns the number of workers
func (d *Dispatcher) WorkerCount() int {
	return d.workers
}

// QueueLength returns the current number of tasks in the queue
func (d *Dispatcher) QueueLength() int {
	return len(d.taskQueue)
}
