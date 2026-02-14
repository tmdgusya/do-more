package server

import (
	"encoding/json"
	"testing"
	"time"
)

func TestEventHubFanOut(t *testing.T) {
	hub := NewEventHub()
	ch1 := hub.Subscribe()
	ch2 := hub.Subscribe()
	ch3 := hub.Subscribe()
	defer hub.Unsubscribe(ch1)
	defer hub.Unsubscribe(ch2)
	defer hub.Unsubscribe(ch3)

	event := Event{Type: EventTaskDone, TaskID: "42", Timestamp: time.Now()}
	hub.Broadcast(event)

	for i, ch := range []chan Event{ch1, ch2, ch3} {
		select {
		case got := <-ch:
			if got.Type != EventTaskDone {
				t.Errorf("subscriber %d: got type %q, want %q", i, got.Type, EventTaskDone)
			}
			if got.TaskID != "42" {
				t.Errorf("subscriber %d: got taskID %q, want %q", i, got.TaskID, "42")
			}
		case <-time.After(100 * time.Millisecond):
			t.Errorf("subscriber %d: timed out waiting for event", i)
		}
	}
}

func TestEventHubNonBlocking(t *testing.T) {
	hub := NewEventHub()
	ch := hub.Subscribe()
	defer hub.Unsubscribe(ch)

	for i := 0; i < 100; i++ {
		hub.Broadcast(Event{Type: EventLogMessage, Timestamp: time.Now()})
	}

	done := make(chan struct{})
	go func() {
		hub.Broadcast(Event{Type: EventTaskDone, Timestamp: time.Now()})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Broadcast blocked on full subscriber channel")
	}
}

func TestEventHubUnsubscribe(t *testing.T) {
	hub := NewEventHub()
	ch := hub.Subscribe()

	hub.Broadcast(Event{Type: EventLoopStarted, Timestamp: time.Now()})
	select {
	case <-ch:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected event before unsubscribe")
	}

	hub.Unsubscribe(ch)

	hub.Broadcast(Event{Type: EventTaskDone, Timestamp: time.Now()})

	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("received event after unsubscribe")
		}
	case <-time.After(50 * time.Millisecond):
	}
}

func TestEventLoggerParsing(t *testing.T) {
	tests := []struct {
		name     string
		msg      string
		wantType string
		wantID   string
		checkFn  func(t *testing.T, e Event)
	}{
		{
			name:     "loop started",
			msg:      "Starting with provider: claude",
			wantType: EventLoopStarted,
			checkFn: func(t *testing.T, e Event) {
				if e.Data["provider"] != "claude" {
					t.Errorf("provider = %v, want claude", e.Data["provider"])
				}
			},
		},
		{
			name:     "iteration started",
			msg:      "── Iteration 2/10 ── Task #3: Add login endpoint",
			wantType: EventIterationStarted,
			wantID:   "3",
			checkFn: func(t *testing.T, e Event) {
				if e.Data["iteration"] != 2 {
					t.Errorf("iteration = %v, want 2", e.Data["iteration"])
				}
				if e.Data["maxIterations"] != 10 {
					t.Errorf("maxIterations = %v, want 10", e.Data["maxIterations"])
				}
				if e.Data["title"] != "Add login endpoint" {
					t.Errorf("title = %v, want 'Add login endpoint'", e.Data["title"])
				}
			},
		},
		{
			name:     "provider invoked",
			msg:      "Invoking claude...",
			wantType: EventProviderInvoked,
			checkFn: func(t *testing.T, e Event) {
				if e.Data["provider"] != "claude" {
					t.Errorf("provider = %v, want claude", e.Data["provider"])
				}
			},
		},
		{
			name:     "provider finished",
			msg:      "Provider finished",
			wantType: EventProviderFinished,
		},
		{
			name:     "gate passed",
			msg:      "Running gate: go test ./...  ✓",
			wantType: EventGateResult,
			checkFn: func(t *testing.T, e Event) {
				if e.Data["command"] != "go test ./..." {
					t.Errorf("command = %v, want 'go test ./...'", e.Data["command"])
				}
				if e.Data["passed"] != true {
					t.Errorf("passed = %v, want true", e.Data["passed"])
				}
			},
		},
		{
			name:     "gate failed",
			msg:      "Running gate: golangci-lint run  ✗",
			wantType: EventGateResult,
			checkFn: func(t *testing.T, e Event) {
				if e.Data["command"] != "golangci-lint run" {
					t.Errorf("command = %v, want 'golangci-lint run'", e.Data["command"])
				}
				if e.Data["passed"] != false {
					t.Errorf("passed = %v, want false", e.Data["passed"])
				}
			},
		},
		{
			name:     "task done",
			msg:      "Task #5: done",
			wantType: EventTaskDone,
			wantID:   "5",
		},
		{
			name:     "task failed",
			msg:      "Task #7: failed (max iterations reached)",
			wantType: EventTaskFailed,
			wantID:   "7",
		},
		{
			name:     "provider error becomes log message",
			msg:      "Provider error: context canceled",
			wantType: EventLogMessage,
			checkFn: func(t *testing.T, e Event) {
				if e.Data["message"] != "Provider error: context canceled" {
					t.Errorf("message = %v", e.Data["message"])
				}
			},
		},
		{
			name:     "summary becomes log message",
			msg:      "── Summary ──",
			wantType: EventLogMessage,
		},
		{
			name:     "unknown format becomes log message",
			msg:      "something unexpected happened",
			wantType: EventLogMessage,
			checkFn: func(t *testing.T, e Event) {
				if e.Data["message"] != "something unexpected happened" {
					t.Errorf("message = %v", e.Data["message"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := parseLogMessage(tt.msg)
			if event.Type != tt.wantType {
				t.Errorf("type = %q, want %q", event.Type, tt.wantType)
			}
			if tt.wantID != "" && event.TaskID != tt.wantID {
				t.Errorf("taskID = %q, want %q", event.TaskID, tt.wantID)
			}
			if tt.checkFn != nil {
				tt.checkFn(t, event)
			}
		})
	}
}

func TestEventJSON(t *testing.T) {
	event := Event{
		Type:      EventTaskDone,
		TaskID:    "99",
		Data:      map[string]any{"extra": "info"},
		Timestamp: time.Date(2026, 2, 14, 12, 0, 0, 0, time.UTC),
	}

	jsonStr := event.JSON()

	var decoded map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &decoded); err != nil {
		t.Fatalf("JSON() produced invalid JSON: %v\noutput: %s", err, jsonStr)
	}

	if decoded["type"] != EventTaskDone {
		t.Errorf("type = %v, want %v", decoded["type"], EventTaskDone)
	}
	if decoded["taskId"] != "99" {
		t.Errorf("taskId = %v, want 99", decoded["taskId"])
	}
}
