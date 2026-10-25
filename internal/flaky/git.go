package flaky

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type Commit struct {
	Hash    string `json:"commit"`
	Subject string `json:"subject"`
}

func git(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	configureProcess(cmd)
	out, err := cmd.CombinedOutput()
	finishProcess(cmd)
	if errors.Is(err, exec.ErrWaitDelay) {
		err = nil
	}
	if err != nil {
		return "", fmt.Errorf("git %s: %s: %w", strings.Join(args, " "), strings.TrimSpace(string(out)), err)
	}
	return strings.TrimSuffix(string(out), "\n"), nil
}
func History(ctx context.Context, dir string) ([]Commit, error) {
	status, err := git(ctx, dir, "status", "--porcelain", "--untracked-files=all")
	if err != nil {
		return nil, err
	}
	if status != "" {
		return nil, fmt.Errorf("bisect requires a clean git working tree")
	}
	out, err := git(ctx, dir, "log", "--first-parent", "--reverse", "--format=%H%x00%s", "HEAD")
	if err != nil {
		return nil, err
	}
	var commits []Commit
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\x00", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid git log output")
		}
		commits = append(commits, Commit{parts[0], parts[1]})
	}
	return commits, nil
}
func WithWorktree(ctx context.Context, dir, hash string, fn func(string) error) (resultErr error) {
	root, err := git(ctx, dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return err
	}
	parent, err := workspaceTemp(root, "flaky-bisect-")
	if err != nil {
		return err
	}
	defer func() {
		resultErr = errors.Join(resultErr, removeWorkspace(parent))
	}()
	path := filepath.Join(parent, "work")
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		resultErr = errors.Join(resultErr, writableWorkspace(path))
		if _, err := git(cleanupCtx, dir, "worktree", "remove", "--force", path); err != nil && resultErr == nil {
			resultErr = err
		}
	}()
	if _, err := git(ctx, dir, "worktree", "add", "--detach", path, hash); err != nil {
		return err
	}
	return fn(path)
}
