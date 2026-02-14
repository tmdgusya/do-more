package provider

import (
	"context"
	"testing"
)

type mockProvider struct {
	name   string
	output string
	err    error
}

func (m *mockProvider) Name() string {
	return m.name
}

func (m *mockProvider) Run(ctx context.Context, prompt string, workDir string) (string, error) {
	return m.output, m.err
}

func TestRegisterAndGetProvider(t *testing.T) {
	registry := NewProviderRegistry()
	mock := &mockProvider{name: "mock", output: "done"}

	registry.Register(mock)

	p, ok := registry.Get("mock")
	if !ok {
		t.Fatal("expected provider to be registered")
	}
	if p.Name() != "mock" {
		t.Errorf("Name() = %q, want %q", p.Name(), "mock")
	}
}

func TestGetUnregisteredProvider(t *testing.T) {
	registry := NewProviderRegistry()

	_, ok := registry.Get("nonexistent")
	if ok {
		t.Fatal("expected provider to not be found")
	}
}

func TestListProviders(t *testing.T) {
	registry := NewProviderRegistry()
	registry.Register(&mockProvider{name: "alpha"})
	registry.Register(&mockProvider{name: "beta"})

	names := registry.List()
	if len(names) != 2 {
		t.Fatalf("len(List()) = %d, want 2", len(names))
	}
}

func TestFormatModelsWithConfigured(t *testing.T) {
	available := []string{"claude", "kimi", "opencode"}
	result := FormatModels(available, "claude")

	expected := "  * claude (configured)\n  - kimi\n  - opencode\n"
	if result != expected {
		t.Errorf("FormatModels() =\n%q\nwant\n%q", result, expected)
	}
}

func TestFormatModelsNoneConfigured(t *testing.T) {
	available := []string{"claude", "kimi", "opencode"}
	result := FormatModels(available, "")

	expected := "  - claude\n  - kimi\n  - opencode\n"
	if result != expected {
		t.Errorf("FormatModels() =\n%q\nwant\n%q", result, expected)
	}
}

func TestFormatModelsConfiguredNotInList(t *testing.T) {
	available := []string{"claude", "kimi"}
	result := FormatModels(available, "nonexistent")

	expected := "  - claude\n  - kimi\n"
	if result != expected {
		t.Errorf("FormatModels() =\n%q\nwant\n%q", result, expected)
	}
}
