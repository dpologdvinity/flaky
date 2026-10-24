package flaky

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func Run(ctx context.Context, source string, command []string, junit string) ([]Case, error) {
	work, clean, err := CopyWorkspace(source)
	if err != nil {
		return nil, err
	}
	defer clean()
	report := filepath.Join(work, "flaky-junit.xml")
	if junit != "" {
		report = filepath.Join(work, junit)
	}
	if err := os.MkdirAll(filepath.Dir(report), 0755); err != nil {
		return nil, err
	}
	_ = os.Remove(report)
	args := append([]string(nil), command[1:]...)
	env := append(os.Environ(), "FLAKY_JUNIT_XML="+report)
	base := filepath.Base(command[0])
	adapter := base
	if base == "python" || base == "python3" {
		if len(args) >= 2 && args[0] == "-m" && args[1] == "pytest" {
			adapter = "pytest"
		}
	}
	if base == "npx" {
		if len(args) > 0 && args[0] == "jest" {
			adapter = "jest"
		}
	}
	if junit == "" {
		switch adapter {
		case "pytest":
			args = append(args, "--junitxml="+report)
		case "gotestsum":
			at := len(args)
			for i, v := range args {
				if v == "--" {
					at = i
					break
				}
			}
			args = append(args[:at], append([]string{"--junitfile=" + report}, args[at:]...)...)
		case "jest":
			env = append(env, "JEST_JUNIT_OUTPUT_DIR="+filepath.Dir(report), "JEST_JUNIT_OUTPUT_NAME="+filepath.Base(report))
		default:
			return nil, fmt.Errorf("unsupported runner %q: pass --junit", base)
		}
	}
	cmd := exec.CommandContext(ctx, command[0], args...)
	cmd.Dir = work
	cmd.Env = env
	output, runErr := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	data, err := os.Open(report)
	if err != nil {
		return nil, fmt.Errorf("JUnit report missing at %s: %w; command: %s", junit, err, strings.TrimSpace(string(output)))
	}
	defer data.Close()
	cases, err := ParseJUnit(data)
	if err != nil {
		return nil, fmt.Errorf("invalid JUnit report: %w", err)
	}
	if runErr != nil {
		if _, ok := runErr.(*exec.ExitError); !ok {
			return nil, runErr
		}
	}
	return cases, nil
}
