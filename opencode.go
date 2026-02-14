// opencode.go
package main

import (
	"context"
	"fmt"
	"os/exec"
)

type OpenCodeProvider struct{}

func (p *OpenCodeProvider) Name() string {
	return "opencode"
}

func (p *OpenCodeProvider) Run(ctx context.Context, prompt string, workDir string) (string, error) {
	cmd := exec.CommandContext(ctx, "opencode", "-p", prompt, "-q", "-f", "text")
	cmd.Dir = workDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("opencode provider: %w\noutput: %s", err, output)
	}
	return string(output), nil
}
