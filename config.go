// config.go
package main

import (
	"encoding/json"
	"fmt"
	"os"
)

const (
	StatusPending    = "pending"
	StatusInProgress = "in_progress"
	StatusDone       = "done"
	StatusFailed     = "failed"
)

type Task struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"`
	Learnings   string `json:"learnings"`
}

type Config struct {
	Name          string   `json:"name"`
	Provider      string   `json:"provider"`
	Branch        string   `json:"branch"`
	Gates         []string `json:"gates"`
	MaxIterations int      `json:"maxIterations"`
	Tasks         []Task   `json:"tasks"`
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	return &cfg, nil
}

func SaveConfig(path string, cfg *Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}
	return nil
}

func (c *Config) NextPendingTask() *Task {
	for i := range c.Tasks {
		if c.Tasks[i].Status == StatusPending {
			return &c.Tasks[i]
		}
	}
	return nil
}
