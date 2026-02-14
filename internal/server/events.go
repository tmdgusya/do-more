package server

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/tmdgusya/do-more/internal/loop"
)

const (
	EventLoopStarted      = "loop_started"
	EventLoopCompleted    = "loop_completed"
	EventLoopError        = "loop_error"
	EventLoopStopped      = "loop_stopped"
	EventTaskStarted      = "task_started"
	EventIterationStarted = "iteration_started"
	EventProviderInvoked  = "provider_invoked"
	EventProviderFinished = "provider_finished"
	EventGateResult       = "gate_result"
	EventTaskDone         = "task_done"
	EventTaskFailed       = "task_failed"
	EventLogMessage       = "log_message"
)

// Event represents a structured event emitted during loop execution.
type Event struct {
	Type      string         `json:"type"`
	TaskID    string         `json:"taskId,omitempty"`
	Data      map[string]any `json:"data,omitempty"`
	Timestamp time.Time      `json:"timestamp"`
}

// JSON serializes the event to a JSON string for SSE transport.
func (e Event) JSON() string {
	b, err := json.Marshal(e)
	if err != nil {
		return fmt.Sprintf(`{"type":"error","data":{"message":%q}}`, err.Error())
	}
	return string(b)
}

// EventHub is a pub/sub fan-out broadcaster for events.
// Subscribers receive events on buffered channels. Slow subscribers
// are skipped (non-blocking broadcast) to prevent backpressure.
type EventHub struct {
	mu          sync.RWMutex
	subscribers map[chan Event]struct{}
}

func NewEventHub() *EventHub {
	return &EventHub{
		subscribers: make(map[chan Event]struct{}),
	}
}

// Subscribe creates and returns a buffered channel that will receive
// broadcast events. The caller must call Unsubscribe when done.
func (h *EventHub) Subscribe() chan Event {
	ch := make(chan Event, 64)
	h.mu.Lock()
	h.subscribers[ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

func (h *EventHub) Unsubscribe(ch chan Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.subscribers[ch]; ok {
		delete(h.subscribers, ch)
		close(ch)
	}
}

// Broadcast sends an event to all subscribers. Non-blocking: if a
// subscriber's channel buffer is full, that subscriber is skipped.
func (h *EventHub) Broadcast(event Event) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for ch := range h.subscribers {
		select {
		case ch <- event:
		default:
		}
	}
}

var _ loop.Logger = (*EventLogger)(nil)

// EventLogger implements loop.Logger. It prints to stdout (preserving
// CLI output) and emits structured events by parsing known log patterns.
type EventLogger struct {
	hub *EventHub
}

func NewEventLogger(hub *EventHub) *EventLogger {
	return &EventLogger{hub: hub}
}

func (l *EventLogger) Log(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	fmt.Printf("[do-more] %s\n", msg)

	event := parseLogMessage(msg)
	event.Timestamp = time.Now()
	l.hub.Broadcast(event)
}

func parseLogMessage(msg string) Event {
	var iter, maxIter int
	var taskID, title string
	if n, _ := fmt.Sscanf(msg, "── Iteration %d/%d ── Task #%s", &iter, &maxIter, &taskID); n == 3 {
		taskID = strings.TrimSuffix(taskID, ":")
		prefix := fmt.Sprintf("── Iteration %d/%d ── Task #%s: ", iter, maxIter, taskID)
		title = strings.TrimPrefix(msg, prefix)
		return Event{
			Type:   EventIterationStarted,
			TaskID: taskID,
			Data: map[string]any{
				"iteration":     iter,
				"maxIterations": maxIter,
				"title":         title,
			},
		}
	}

	if strings.HasPrefix(msg, "Invoking ") && strings.HasSuffix(msg, "...") {
		providerName := strings.TrimSuffix(strings.TrimPrefix(msg, "Invoking "), "...")
		return Event{
			Type: EventProviderInvoked,
			Data: map[string]any{"provider": providerName},
		}
	}

	if msg == "Provider finished" {
		return Event{Type: EventProviderFinished}
	}

	if strings.HasPrefix(msg, "Running gate: ") && strings.HasSuffix(msg, "  ✓") {
		cmd := strings.TrimSuffix(strings.TrimPrefix(msg, "Running gate: "), "  ✓")
		return Event{
			Type: EventGateResult,
			Data: map[string]any{"command": cmd, "passed": true},
		}
	}

	if strings.HasPrefix(msg, "Running gate: ") && strings.HasSuffix(msg, "  ✗") {
		cmd := strings.TrimSuffix(strings.TrimPrefix(msg, "Running gate: "), "  ✗")
		return Event{
			Type: EventGateResult,
			Data: map[string]any{"command": cmd, "passed": false},
		}
	}

	if strings.HasPrefix(msg, "Task #") && strings.HasSuffix(msg, ": done") {
		id := strings.TrimSuffix(strings.TrimPrefix(msg, "Task #"), ": done")
		return Event{
			Type:   EventTaskDone,
			TaskID: id,
		}
	}

	if strings.HasPrefix(msg, "Task #") && strings.HasSuffix(msg, ": failed (max iterations reached)") {
		id := strings.TrimSuffix(strings.TrimPrefix(msg, "Task #"), ": failed (max iterations reached)")
		return Event{
			Type:   EventTaskFailed,
			TaskID: id,
		}
	}

	if strings.HasPrefix(msg, "Starting with default provider: ") {
		providerName := strings.TrimPrefix(msg, "Starting with default provider: ")
		return Event{
			Type: EventLoopStarted,
			Data: map[string]any{"provider": providerName},
		}
	}

	if strings.HasPrefix(msg, "Starting with provider: ") {
		providerName := strings.TrimPrefix(msg, "Starting with provider: ")
		return Event{
			Type: EventLoopStarted,
			Data: map[string]any{"provider": providerName},
		}
	}

	return Event{
		Type: EventLogMessage,
		Data: map[string]any{"message": msg},
	}
}
