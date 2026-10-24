package flaky

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func fixtureRepo(t *testing.T) string {
	t.Helper()
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
