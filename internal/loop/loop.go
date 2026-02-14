package loop

import (
	"context"
	"fmt"

	"github.com/tmdgusya/do-more/internal/config"
	"github.com/tmdgusya/do-more/internal/gate"
	"github.com/tmdgusya/do-more/internal/prompt"
	"github.com/tmdgusya/do-more/internal/provider"
)

type Logger interface {
	Log(format string, args ...any)
}

type StdoutLogger struct{}

func (l *StdoutLogger) Log(format string, args ...any) {
	fmt.Printf("[do-more] "+format+"\n", args...)
}

func RunLoop(ctx context.Context, cfgPath string, providerName string, registry *provider.ProviderRegistry, workDir string, logger Logger) error {
	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	logger.Log("Starting with default provider: %s", providerName)

	for {
		task := cfg.NextPendingTask()
		if task == nil {
			break
		}

		task.Status = config.StatusInProgress
		if err := config.SaveConfig(cfgPath, cfg); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}

		// Resolve provider per-task
		effectiveProvider := task.EffectiveProvider(providerName)
		p, ok := registry.Get(effectiveProvider)
		if !ok {
			task.Status = config.StatusFailed
			task.Learnings += fmt.Sprintf("\nUnknown provider: %q", effectiveProvider)
			logger.Log("Task #%s: failed (unknown provider: %s)", task.ID, effectiveProvider)
			if err := config.SaveConfig(cfgPath, cfg); err != nil {
				return fmt.Errorf("saving config: %w", err)
			}
			continue
		}

		var gateOutput string
		completed := false

		for iteration := 1; iteration <= cfg.MaxIterations; iteration++ {
			logger.Log("── Iteration %d/%d ── Task #%s: %s", iteration, cfg.MaxIterations, task.ID, task.Title)

			pr := prompt.BuildPrompt(task, cfg.Gates, gateOutput)

			logger.Log("Invoking %s...", p.Name())
			output, err := p.Run(ctx, pr, workDir)
			if err != nil {
				logger.Log("Provider error: %v", err)
				if iteration >= cfg.MaxIterations {
					task.Status = config.StatusFailed
					task.Learnings += fmt.Sprintf("\nFailed after %d iterations. Last error: %v", iteration, err)
					break
				}
				gateOutput = fmt.Sprintf("Provider error: %v\nOutput: %s", err, output)
				continue
			}

			logger.Log("Provider finished")

			results, err := gate.RunGates(ctx, cfg.Gates, workDir)
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
				task.Status = config.StatusDone
				completed = true
				logger.Log("Task #%s: done", task.ID)
				break
			}

			if iteration >= cfg.MaxIterations {
				task.Status = config.StatusFailed
				task.Learnings += fmt.Sprintf("\nFailed after %d iterations. Gates did not pass.", iteration)
				logger.Log("Task #%s: failed (max iterations reached)", task.ID)
				break
			}

			gateOutput = gate.GateFailureSummary(results)
		}

		if err := config.SaveConfig(cfgPath, cfg); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}

		_ = completed
	}

	// Print summary
	done := 0
	failed := 0
	for _, t := range cfg.Tasks {
		switch t.Status {
		case config.StatusDone:
			done++
		case config.StatusFailed:
			failed++
		}
	}
	total := len(cfg.Tasks)
	logger.Log("── Summary ──")
	logger.Log("%d/%d tasks done, %d failed", done, total, failed)

	return nil
}
