// gate_test.go
package main

import (
	"context"
	"testing"
)

func TestRunGatesAllPass(t *testing.T) {
	gates := []string{"true", "echo hello"}
	results, err := RunGates(context.Background(), gates, t.TempDir())
	if err != nil {
		t.Fatalf("RunGates failed: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(results))
	}
	for i, r := range results {
		if !r.Passed {
			t.Errorf("gate %d (%q) failed: %s", i, r.Command, r.Output)
		}
	}
}

func TestRunGatesOneFails(t *testing.T) {
	gates := []string{"true", "false", "true"}
	results, err := RunGates(context.Background(), gates, t.TempDir())
	if err != nil {
		t.Fatalf("RunGates failed: %v", err)
	}
	if results[0].Passed != true {
		t.Error("gate 0 should pass")
	}
	if results[1].Passed != false {
		t.Error("gate 1 should fail")
	}
	if results[2].Passed != true {
		t.Error("gate 2 should pass")
	}
}

func TestRunGatesEmpty(t *testing.T) {
	results, err := RunGates(context.Background(), []string{}, t.TempDir())
	if err != nil {
		t.Fatalf("RunGates failed: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("len(results) = %d, want 0", len(results))
	}
}

func TestAllGatesPassed(t *testing.T) {
	passing := []GateResult{
		{Command: "true", Passed: true},
		{Command: "echo ok", Passed: true},
	}
	if !AllGatesPassed(passing) {
		t.Error("expected AllGatesPassed to return true")
	}

	failing := []GateResult{
		{Command: "true", Passed: true},
		{Command: "false", Passed: false, Output: "failed"},
	}
	if AllGatesPassed(failing) {
		t.Error("expected AllGatesPassed to return false")
	}
}

func TestGateFailureSummary(t *testing.T) {
	results := []GateResult{
		{Command: "true", Passed: true},
		{Command: "false", Passed: false, Output: "exit status 1"},
	}
	summary := GateFailureSummary(results)
	if summary == "" {
		t.Error("expected non-empty summary")
	}
}
