// prompt_test.go
package main

import (
	"strings"
	"testing"
)

func TestBuildPrompt(t *testing.T) {
	task := &Task{
		Title:       "Add login endpoint",
		Description: "Create POST /api/login",
		Learnings:   "Use bcrypt for passwords",
	}
	gates := []string{"go test ./...", "golangci-lint run"}

	prompt := BuildPrompt(task, gates, "")

	if !strings.Contains(prompt, "Add login endpoint") {
		t.Error("prompt should contain task title")
	}
	if !strings.Contains(prompt, "Create POST /api/login") {
		t.Error("prompt should contain task description")
	}
	if !strings.Contains(prompt, "Use bcrypt for passwords") {
		t.Error("prompt should contain learnings")
	}
	if !strings.Contains(prompt, "go test ./...") {
		t.Error("prompt should contain gates")
	}
}

func TestBuildPromptWithGateFailures(t *testing.T) {
	task := &Task{
		Title:       "Fix tests",
		Description: "Make tests pass",
	}
	gates := []string{"go test ./..."}
	gateOutput := "FAIL: TestFoo expected 1 got 2"

	prompt := BuildPrompt(task, gates, gateOutput)

	if !strings.Contains(prompt, gateOutput) {
		t.Error("prompt should contain gate failure output")
	}
}

func TestBuildPromptNoLearnings(t *testing.T) {
	task := &Task{
		Title:       "New task",
		Description: "Do something",
		Learnings:   "",
	}

	prompt := BuildPrompt(task, []string{}, "")

	if strings.Contains(prompt, "Previous Learnings") {
		t.Error("prompt should not contain learnings section when empty")
	}
}
