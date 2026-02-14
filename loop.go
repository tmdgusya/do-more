// loop.go
package main

import (
	"context"
	"fmt"
)

type Logger interface {
	Log(format string, args ...any)
}

type StdoutLogger struct{}

func (l *StdoutLogger) Log(format string, args ...any) {
	fmt.Printf("[do-more] "+format+"\n", args...)
}

func RunLoop(ctx context.Context, cfgPath string, providerName string, registry *ProviderRegistry, workDir string, logger Logger) error {
	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	provider, ok := registry.Get(providerName)
	if !ok {
		return fmt.Errorf("unknown provider: %q", providerName)
	}

	logger.Log("Starting with provider: %s", provider.Name())

	for {
		task := cfg.NextPendingTask()
		if task == nil {
			break
		}

		task.Status = StatusInProgress
		if err := SaveConfig(cfgPath, cfg); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}

		var gateOutput string
		completed := false

		for iteration := 1; iteration <= cfg.MaxIterations; iteration++ {
			logger.Log("── Iteration %d/%d ── Task #%s: %s", iteration, cfg.MaxIterations, task.ID, task.Title)

			prompt := BuildPrompt(task, cfg.Gates, gateOutput)

			logger.Log("Invoking %s...", provider.Name())
			output, err := provider.Run(ctx, prompt, workDir)
			if err != nil {
				logger.Log("Provider error: %v", err)
				if iteration >= cfg.MaxIterations {
					task.Status = StatusFailed
					task.Learnings += fmt.Sprintf("\nFailed after %d iterations. Last error: %v", iteration, err)
					break
				}
				gateOutput = fmt.Sprintf("Provider error: %v\nOutput: %s", err, output)
				continue
			}

			logger.Log("Provider finished")

			results, err := RunGates(ctx, cfg.Gates, workDir)
			if err != nil {
				return fmt.Errorf("running gates: %w", err)
			}

			allPassed := true
			for _, r := range results {
				if r.Passed {
					logger.Log("Running gate: %s  ✓", r.Command)
				} else {
					logger.Log("Running gate: %s  ✗", r.Command)
					allPassed = false
				}
			}

			if allPassed {
				task.Status = StatusDone
				completed = true
				logger.Log("Task #%s: done", task.ID)
				break
			}

			if iteration >= cfg.MaxIterations {
				task.Status = StatusFailed
				task.Learnings += fmt.Sprintf("\nFailed after %d iterations. Gates did not pass.", iteration)
				logger.Log("Task #%s: failed (max iterations reached)", task.ID)
				break
			}

			gateOutput = GateFailureSummary(results)
		}

		if err := SaveConfig(cfgPath, cfg); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}

		_ = completed
	}

	// Print summary
	done := 0
	failed := 0
	for _, t := range cfg.Tasks {
		switch t.Status {
		case StatusDone:
			done++
		case StatusFailed:
			failed++
		}
	}
	total := len(cfg.Tasks)
	logger.Log("── Summary ──")
	logger.Log("%d/%d tasks done, %d failed", done, total, failed)

	return nil
}
