package engine

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Simplify runs the official egglog CLI and returns its extracted expression.
func Simplify(ctx context.Context, executable, ruleSource, expression string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("egglog: %w", err)
	}
	file, err := os.CreateTemp("", "egglog-helper-*.egg")
	if err != nil {
		return "", fmt.Errorf("egglog: create program: %w", err)
	}
	defer os.Remove(file.Name())
	program := ruleSource + "\n(let target " + expression + ")\n(run 8)\n(extract target)\n"
	if _, err := file.WriteString(program); err != nil {
		file.Close()
		return "", fmt.Errorf("egglog: write program: %w", err)
	}
	if err := file.Close(); err != nil {
		return "", fmt.Errorf("egglog: close program: %w", err)
	}
	cmd := exec.CommandContext(ctx, executable, file.Name())
	output, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() != nil {
			return "", fmt.Errorf("egglog: %w", ctx.Err())
		}
		return "", fmt.Errorf("egglog: %w: %s", err, strings.TrimSpace(string(output)))
	}
	result := strings.TrimSpace(string(output))
	if result == "" || len(result) > 32768 || strings.ContainsAny(result, "\r\n") {
		return "", fmt.Errorf("egglog: unexpected extraction output %q", result)
	}
	return result, nil
}
