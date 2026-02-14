// gate.go
package main

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type GateResult struct {
	Command string
	Passed  bool
	Output  string
}

func RunGates(ctx context.Context, gates []string, workDir string) ([]GateResult, error) {
	results := make([]GateResult, 0, len(gates))
	for _, gate := range gates {
		cmd := exec.CommandContext(ctx, "sh", "-c", gate)
		cmd.Dir = workDir
		output, err := cmd.CombinedOutput()
		result := GateResult{
			Command: gate,
			Passed:  err == nil,
			Output:  string(output),
		}
		results = append(results, result)
	}
	return results, nil
}

func AllGatesPassed(results []GateResult) bool {
	for _, r := range results {
		if !r.Passed {
			return false
		}
	}
	return true
}

func GateFailureSummary(results []GateResult) string {
	var sb strings.Builder
	for _, r := range results {
		if !r.Passed {
			fmt.Fprintf(&sb, "FAIL: %s\n%s\n", r.Command, r.Output)
		}
	}
	return sb.String()
}
