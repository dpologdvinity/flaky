package flaky

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

func Run(ctx context.Context, source string, command []string, junit string) ([]Case, error) {
	return run(ctx, source, command, junit, 1)
}

func run(ctx context.Context, source string, command []string, junit string, index int) ([]Case, error) {
	work, clean, err := CopyWorkspace(source)
	if err != nil {
		return nil, err
	}
	defer clean()
	return runInWork(ctx, work, command, junit, index)
}

func reportPath(work, junit string) (string, error) {
	if junit == "" {
		junit = "flaky-junit.xml"
	}
	if !filepath.IsLocal(junit) {
		return "", fmt.Errorf("JUnit path must stay within workspace")
	}
	part := work
	for _, component := range strings.Split(filepath.Clean(junit), string(os.PathSeparator)) {
		part = filepath.Join(part, component)
		info, err := os.Lstat(part)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return "", err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("JUnit path contains symlink: %s", junit)
		}
	}
	return filepath.Join(work, junit), nil
}

func runInWork(ctx context.Context, work string, command []string, junit string, index int) ([]Case, error) {
	report, err := reportPath(work, junit)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(report), 0755); err != nil {
		return nil, err
	}
	if err := os.Remove(report); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	args := append([]string(nil), command[1:]...)
	env := append(os.Environ(), "FLAKY_JUNIT_XML="+report, "FLAKY_RUN_INDEX="+strconv.Itoa(index))
	base := strings.ToLower(filepath.Base(command[0]))
	base = strings.TrimSuffix(strings.TrimSuffix(base, ".exe"), ".cmd")
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
	configureProcess(cmd)
	output, runErr := cmd.CombinedOutput()
	finishProcess(cmd)
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if _, err := reportPath(work, junit); err != nil {
		return nil, err
	}
	data, err := os.Open(report)
	if err != nil {
		return nil, fmt.Errorf("JUnit report missing at %s: %w; command: %s", report, err, strings.TrimSpace(string(output)))
	}
	defer func() {
		_ = data.Close()
	}()
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
