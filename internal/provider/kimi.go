package provider

import (
	"context"
	"fmt"
	"os/exec"
)

type KimiProvider struct{}

func (p *KimiProvider) Name() string {
	return "kimi"
}

func (p *KimiProvider) Run(ctx context.Context, prompt string, workDir string) (string, error) {
	cmd := exec.CommandContext(ctx, "kimi", "--print", "-p", prompt, "--final-message-only")
	cmd.Dir = workDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("kimi provider: %w\noutput: %s", err, output)
	}
	return string(output), nil
}
