// main.go
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func defaultRegistry() *ProviderRegistry {
	registry := NewProviderRegistry()
	registry.Register(&ClaudeProvider{})
	registry.Register(&OpenCodeProvider{})
	registry.Register(&KimiProvider{})
	return registry
}

func main() {
	registry := defaultRegistry()

	rootCmd := &cobra.Command{
		Use:   "do-more",
		Short: "Autonomous AI coding loop orchestrator",
	}

	// --- init ---
	initCmd := &cobra.Command{
		Use:   "init",
		Short: "Create a do-more.json template in the current directory",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := filepath.Join(".", "do-more.json")
			if _, err := os.Stat(path); err == nil {
				return fmt.Errorf("do-more.json already exists")
			}
			cfg := &Config{
				Name:          filepath.Base(mustGetwd()),
				Provider:      "claude",
				Branch:        "feat/do-more",
				Gates:         []string{},
				MaxIterations: 10,
				Tasks: []Task{
					{
						ID:          "1",
						Title:       "Example task",
						Description: "Describe what needs to be done",
						Status:      StatusPending,
						Learnings:   "",
					},
				},
			}
			if err := SaveConfig(path, cfg); err != nil {
				return err
			}
			fmt.Println("[do-more] Created do-more.json")
			return nil
		},
	}

	// --- run ---
	var providerFlag string
	var maxIterationsFlag int
	var configFlag string

	runCmd := &cobra.Command{
		Use:   "run",
		Short: "Start the autonomous loop",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgPath := configFlag
			cfg, err := LoadConfig(cfgPath)
			if err != nil {
				return fmt.Errorf("loading %s: %w", cfgPath, err)
			}

			providerName := cfg.Provider
			if providerFlag != "" {
				providerName = providerFlag
			}

			if maxIterationsFlag > 0 {
				cfg.MaxIterations = maxIterationsFlag
				if err := SaveConfig(cfgPath, cfg); err != nil {
					return err
				}
			}

			workDir := filepath.Dir(cfgPath)
			if !filepath.IsAbs(workDir) {
				workDir = mustGetwd()
			}

			logger := &StdoutLogger{}
			return RunLoop(context.Background(), cfgPath, providerName, registry, workDir, logger)
		},
	}
	runCmd.Flags().StringVar(&providerFlag, "provider", "", "Override provider from config")
	runCmd.Flags().IntVar(&maxIterationsFlag, "max-iterations", 0, "Override max iterations per task")
	runCmd.Flags().StringVar(&configFlag, "config", "do-more.json", "Path to config file")

	// --- status ---
	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show task status summary",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := LoadConfig("do-more.json")
			if err != nil {
				return err
			}

			fmt.Printf("Project: %s\n", cfg.Name)
			fmt.Printf("Provider: %s\n", cfg.Provider)
			fmt.Printf("Branch: %s\n", cfg.Branch)
			fmt.Printf("Gates: %s\n", strings.Join(cfg.Gates, ", "))
			fmt.Println()

			for _, t := range cfg.Tasks {
				marker := " "
				switch t.Status {
				case StatusDone:
					marker = "✓"
				case StatusFailed:
					marker = "✗"
				case StatusInProgress:
					marker = "→"
				}
				fmt.Printf("  [%s] #%s %s (%s)\n", marker, t.ID, t.Title, t.Status)
			}
			return nil
		},
	}

	// --- providers ---
	providersCmd := &cobra.Command{
		Use:   "providers",
		Short: "List available providers",
		Run: func(cmd *cobra.Command, args []string) {
			for _, name := range registry.List() {
				fmt.Printf("  - %s\n", name)
			}
		},
	}

	rootCmd.AddCommand(initCmd, runCmd, statusCmd, providersCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func mustGetwd() string {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return wd
}
