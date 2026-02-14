// claude.go
package main

import (
	"context"
	"fmt"
	"os/exec"
)

type ClaudeProvider struct{}

func (p *ClaudeProvider) Name() string {
	return "claude"
}

func (p *ClaudeProvider) Run(ctx context.Context, prompt string, workDir string) (string, error) {
	cmd := exec.CommandContext(ctx, "claude", "-p", prompt, "--output-format", "text")
	cmd.Dir = workDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("claude provider: %w\noutput: %s", err, output)
	}
	return string(output), nil
}
