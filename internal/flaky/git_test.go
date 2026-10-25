package flaky

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func fixtureRepo(t *testing.T) string {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	dir := t.TempDir()
	env := append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_AUTHOR_NAME=Fixture", "GIT_AUTHOR_EMAIL=fixture@example.test", "GIT_COMMITTER_NAME=Fixture", "GIT_COMMITTER_EMAIL=fixture@example.test", "GIT_AUTHOR_DATE=2020-01-01T00:00:00Z", "GIT_COMMITTER_DATE=2020-01-01T00:00:00Z")
	for _, args := range [][]string{{"init", "-q", dir}, {"-C", dir, "config", "user.name", "Fixture"}, {"-C", dir, "config", "user.email", "fixture@example.test"}} {
		c := exec.Command("git", args...)
		c.Env = env
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("%s: %v", out, err)
		}
	}
	for i := 0; i < 3; i++ {
		if err := os.WriteFile(filepath.Join(dir, "state"), []byte{byte('0' + i)}, 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "fake.sh"), []byte("#!/bin/sh\nprintf '<testsuite><testcase classname=\"pkg\" name=\"x\"/></testsuite>' > \"$FLAKY_JUNIT_XML\"\n"), 0755); err != nil {
			t.Fatal(err)
		}
		for _, args := range [][]string{{"-C", dir, "add", "."}, {"-C", dir, "commit", "-qm", "step"}} {
			c := exec.Command("git", args...)
			c.Env = env
			if out, err := c.CombinedOutput(); err != nil {
				t.Fatalf("%s: %v", out, err)
			}
		}
	}
	return dir
}
func TestHistoryAndWorktree(t *testing.T) {
	dir := fixtureRepo(t)
	hist, err := History(context.Background(), dir)
	if err != nil || len(hist) != 3 {
		t.Fatal(hist, err)
	}
	err = WithWorktree(context.Background(), dir, hist[0].Hash, func(path string) error {
		b, err := os.ReadFile(filepath.Join(path, "state"))
		if err != nil || string(b) != "0" {
			t.Fatal(string(b), err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	out, err := git(context.Background(), dir, "worktree", "list")
	if err != nil || strings.Count(out, "\n") != 0 {
		t.Fatal(out, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "dirty"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := git(context.Background(), dir, "config", "status.showUntrackedFiles", "no"); err != nil {
		t.Fatal(err)
	}
	if _, err := History(context.Background(), dir); err == nil {
		t.Fatal("dirty accepted")
	}
}

func TestWorktreeRejectsTemporaryDirectoryInsideRepository(t *testing.T) {
	dir := fixtureRepo(t)
	parent := filepath.Join(dir, "tmp")
	if err := os.Mkdir(parent, 0700); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"TMPDIR", "TMP", "TEMP"} {
		t.Setenv(key, parent)
	}
	called := false
	err := WithWorktree(context.Background(), dir, "HEAD", func(string) error { called = true; return nil })
	if err == nil || !strings.Contains(err.Error(), "temporary directory") || called {
		t.Fatalf("unsafe worktree ran: callback=%v, error=%v", called, err)
	}
	entries, err := os.ReadDir(parent)
	if err != nil || len(entries) != 0 {
		t.Fatalf("repository was modified: %v %v", entries, err)
	}
}

func TestWorktreeCleansReadOnlyArtifacts(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix directory permissions")
	}
	dir := fixtureRepo(t)
	var work string
	err := WithWorktree(context.Background(), dir, "HEAD", func(path string) error {
		work = path
		locked := filepath.Join(path, "locked")
		if err := os.Mkdir(locked, 0700); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(locked, "file"), []byte("x"), 0444); err != nil {
			return err
		}
		return os.Chmod(locked, 0000)
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(work); !os.IsNotExist(err) {
		t.Fatalf("worktree remains: %v", err)
	}
	listed, err := git(context.Background(), dir, "worktree", "list", "--porcelain")
	if err != nil || strings.Count(listed, "worktree ") != 1 {
		t.Fatalf("stale worktree: %s %v", listed, err)
	}
}

func TestWorktreeCancellationKillsCheckoutHook(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix checkout hook")
	}
	dir := fixtureRepo(t)
	marker := filepath.Join(t.TempDir(), "started")
	t.Setenv("FLAKY_HOOK_MARKER", marker)
	hook := "#!/bin/sh\nprintf started > \"$FLAKY_HOOK_MARKER\"\nsleep 10\n"
	if err := os.WriteFile(filepath.Join(dir, ".git", "hooks", "post-checkout"), []byte(hook), 0755); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- WithWorktree(ctx, dir, "HEAD", func(string) error { return nil }) }()
	started := false
	for i := 0; i < 200; i++ {
		if _, err := os.Stat(marker); err == nil {
			started = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	if !started {
		t.Error("checkout hook did not start")
	}
	select {
	case err := <-done:
		if err == nil {
			t.Error("canceled checkout succeeded")
		}
	case <-time.After(2 * time.Second):
		t.Error("checkout hook survived cancellation")
		<-done
	}
	listed, err := git(context.Background(), dir, "worktree", "list", "--porcelain")
	if err != nil || strings.Count(listed, "worktree ") != 1 {
		t.Fatalf("stale worktree: %s %v", listed, err)
	}
}
