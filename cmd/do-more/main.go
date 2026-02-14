package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/tmdgusya/do-more/internal/config"
	"github.com/tmdgusya/do-more/internal/loop"
	"github.com/tmdgusya/do-more/internal/provider"
	"github.com/tmdgusya/do-more/internal/server"
)

func defaultRegistry() *provider.ProviderRegistry {
	registry := provider.NewProviderRegistry()
	registry.Register(&provider.ClaudeProvider{})
	registry.Register(&provider.OpenCodeProvider{})
	registry.Register(&provider.KimiProvider{})
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
			cfg := &config.Config{
				Name:          filepath.Base(mustGetwd()),
				Provider:      "claude",
				Branch:        "feat/do-more",
				Gates:         []string{},
				MaxIterations: 10,
				Tasks: []config.Task{
					{
						ID:          "1",
						Title:       "Example task",
						Description: "Describe what needs to be done",
						Status:      config.StatusPending,
						Learnings:   "",
					},
				},
			}
			if err := config.SaveConfig(path, cfg); err != nil {
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
			cfg, err := config.LoadConfig(cfgPath)
			if err != nil {
				return fmt.Errorf("loading %s: %w", cfgPath, err)
			}

			providerName := cfg.Provider
			if providerFlag != "" {
				providerName = providerFlag
			}

			if maxIterationsFlag > 0 {
				cfg.MaxIterations = maxIterationsFlag
				if err := config.SaveConfig(cfgPath, cfg); err != nil {
					return err
				}
			}

			workDir := filepath.Dir(cfgPath)
			if !filepath.IsAbs(workDir) {
				workDir = mustGetwd()
			}

			logger := &loop.StdoutLogger{}
			return loop.RunLoop(context.Background(), cfgPath, providerName, registry, workDir, logger)
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
			cfg, err := config.LoadConfig("do-more.json")
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
				case config.StatusDone:
					marker = "✓"
				case config.StatusFailed:
					marker = "✗"
				case config.StatusInProgress:
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

	// --- serve ---
	var portFlag int
	var serveConfigFlag string

	serveCmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the dashboard server",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgPath := serveConfigFlag
			if _, err := os.Stat(cfgPath); err != nil {
				return fmt.Errorf("do-more.json not found. Run 'do-more init' first.")
			}

			workDir := filepath.Dir(cfgPath)
			if !filepath.IsAbs(workDir) {
				workDir = mustGetwd()
			}
			srv := server.NewServer(cfgPath, workDir, registry)

			addr := fmt.Sprintf("localhost:%d", portFlag)
			fmt.Printf("[do-more] Dashboard: http://%s\n", addr)

			ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer cancel()

			errChan := make(chan error, 1)
			go func() {
				errChan <- srv.ListenAndServe(addr)
			}()

			select {
			case err := <-errChan:
				return err
			case <-ctx.Done():
				shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer shutdownCancel()
				return srv.Shutdown(shutdownCtx)
			}
		},
	}
	serveCmd.Flags().IntVar(&portFlag, "port", 8585, "Port to serve on")
	serveCmd.Flags().StringVar(&serveConfigFlag, "config", "do-more.json", "Path to config file")

	rootCmd.AddCommand(initCmd, runCmd, statusCmd, providersCmd, modelsCmd, serveCmd)

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
