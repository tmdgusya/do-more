# Models Command Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a `do-more models` command that shows which provider/model is configured in the project config and lists all available providers, clearly marking the active one.

**Architecture:** Add a `FormatModels` function to the provider package that takes available provider names and the configured provider, returning formatted output. Wire it into a new Cobra `models` command in main.go that loads the config to determine the configured provider, then prints the formatted list. Gracefully handle missing config (just list available providers without marking any).

**Tech Stack:** Go, Cobra CLI framework, existing provider registry + config packages

---

### Task 1: Write failing tests for FormatModels

**Files:**
- Modify: `internal/provider/provider_test.go`

**Step 1: Write the failing tests**

Add these tests to the bottom of `internal/provider/provider_test.go`:

```go
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
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/provider/ -run TestFormatModels -v`
Expected: FAIL with `undefined: FormatModels`

---

### Task 2: Implement FormatModels

**Files:**
- Modify: `internal/provider/provider.go`

**Step 1: Add FormatModels function**

Add these imports to the existing import block in `internal/provider/provider.go`:

```go
"fmt"
"strings"
```

Add this function after the `List()` method:

```go
func FormatModels(available []string, configured string) string {
	var b strings.Builder
	for _, name := range available {
		if name == configured {
			fmt.Fprintf(&b, "  * %s (configured)\n", name)
		} else {
			fmt.Fprintf(&b, "  - %s\n", name)
		}
	}
	return b.String()
}
```

**Step 2: Run tests to verify they pass**

Run: `go test ./internal/provider/ -run TestFormatModels -v`
Expected: PASS (all 3 tests)

**Step 3: Run all tests to verify nothing is broken**

Run: `go test ./... -v`
Expected: All tests PASS

**Step 4: Commit**

```bash
git add internal/provider/provider.go internal/provider/provider_test.go
git commit -m "feat: add FormatModels function for displaying available models"
```

---

### Task 3: Add models command to CLI

**Files:**
- Modify: `cmd/do-more/main.go`

**Step 1: Add the models command**

Add this block after the `providersCmd` definition (after line 146, before the `rootCmd.AddCommand` call) in `cmd/do-more/main.go`:

```go
	// --- models ---
	var modelsConfigFlag string

	modelsCmd := &cobra.Command{
		Use:   "models",
		Short: "Show available and configured models",
		Run: func(cmd *cobra.Command, args []string) {
			var configured string
			cfg, err := config.LoadConfig(modelsConfigFlag)
			if err == nil {
				configured = cfg.Provider
			}
			fmt.Print(provider.FormatModels(registry.List(), configured))
		},
	}
	modelsCmd.Flags().StringVar(&modelsConfigFlag, "config", "do-more.json", "Path to config file")
```

Then update the `rootCmd.AddCommand` call to include `modelsCmd`:

```go
	rootCmd.AddCommand(initCmd, runCmd, statusCmd, providersCmd, modelsCmd)
```

**Step 2: Run all tests to verify nothing is broken**

Run: `go test ./... -v`
Expected: All tests PASS

**Step 3: Build and manually verify**

Run: `go build -o do-more ./cmd/do-more && ./do-more models`
Expected output (with a `do-more.json` present):
```
  * claude (configured)
  - kimi
  - opencode
```

Run: `./do-more models --config /nonexistent/path`
Expected output (no config found, graceful fallback):
```
  - claude
  - kimi
  - opencode
```

**Step 4: Commit**

```bash
git add cmd/do-more/main.go
git commit -m "feat: add models command to show configured and available models"
```
