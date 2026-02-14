// prompt.go
package main

import (
	"fmt"
	"strings"
)

func BuildPrompt(task *Task, gates []string, gateOutput string) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "You are working on the following task:\n\n")
	fmt.Fprintf(&sb, "## Task: %s\n%s\n", task.Title, task.Description)

	if task.Learnings != "" {
		fmt.Fprintf(&sb, "\n## Previous Learnings\n%s\n", task.Learnings)
	}

	if gateOutput != "" {
		fmt.Fprintf(&sb, "\n## Gate Failures (previous attempt)\n%s\n", gateOutput)
	}

	if len(gates) > 0 {
		fmt.Fprintf(&sb, "\n## Instructions\n")
		fmt.Fprintf(&sb, "- Work in the current directory\n")
		fmt.Fprintf(&sb, "- Make the minimal changes needed\n")
		fmt.Fprintf(&sb, "- When done, the following gates will be checked:\n")
		for _, g := range gates {
			fmt.Fprintf(&sb, "  - %s\n", g)
		}
	}

	return sb.String()
}
