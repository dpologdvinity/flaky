package flaky

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestBisect(t *testing.T) {
	dir := fixtureRepo(t)
	script := filepath.Join(dir, "fake.sh")
	for i := 0; i < 3; i++ {
		body := "#!/bin/sh\nprintf '<testsuite><testcase classname=\"pkg\" name=\"x\"/></testsuite>' > \"$FLAKY_JUNIT_XML\"\n"
		if i > 0 {
			body = "#!/bin/sh\nif [ $((FLAKY_RUN_INDEX % 3)) -eq 0 ]; then printf '<testsuite><testcase classname=\"pkg\" name=\"x\"><failure>bad</failure></testcase></testsuite>' > \"$FLAKY_JUNIT_XML\"; else printf '<testsuite><testcase classname=\"pkg\" name=\"x\"/></testsuite>' > \"$FLAKY_JUNIT_XML\"; fi\n"
		}
		body = strings.Replace(body, "#!/bin/sh\n", "#!/bin/sh\ngit rev-parse --is-inside-work-tree >/dev/null || exit 4\n[ ! -e touched ] || exit 5\ntouch touched\n", 1)
		if err := os.WriteFile(script, []byte(body+"\n# revision "+string(rune('0'+i))+"\n"), 0755); err != nil {
			t.Fatal(err)
		}
		env := append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_AUTHOR_NAME=Fixture", "GIT_AUTHOR_EMAIL=fixture@example.test", "GIT_COMMITTER_NAME=Fixture", "GIT_COMMITTER_EMAIL=fixture@example.test", "GIT_AUTHOR_DATE=2020-01-01T00:00:00Z", "GIT_COMMITTER_DATE=2020-01-01T00:00:00Z")
		c := exec.Command("git", "-C", dir, "add", "fake.sh")
		c.Env = env
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatal(string(out), err)
		}
		c = exec.Command("git", "-C", dir, "commit", "-qm", "test script")
		c.Env = env
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatal(string(out), err)
		}
		if i < 2 {
			continue
		}
	}
	hist, err := History(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	r, err := Bisect(context.Background(), dir, "pkg::x", []string{"./fake.sh"}, "report.xml", 30)
	if err != nil {
		t.Fatal(err)
	}
	if r.FirstCommit != hist[4].Hash {
		t.Fatalf("first=%s want=%s", r.FirstCommit, hist[4].Hash)
	}
}

func TestBisectInconclusiveAndMissing(t *testing.T) {
	dir := fixtureRepo(t)
	if _, err := Bisect(context.Background(), dir, "pkg::absent", []string{"./fake.sh"}, "report.xml", 30); err == nil {
		t.Fatal("missing ID accepted")
	} else {
		var missing MissingTestError
		if !errors.As(err, &missing) {
			t.Fatalf("want missing ID: %v", err)
		}
	}
	script := filepath.Join(dir, "fake.sh")
	body := "#!/bin/sh\nif [ \"$FLAKY_RUN_INDEX\" = 1 ]; then printf '<testsuite><testcase classname=\"pkg\" name=\"other\"/></testsuite>' > \"$FLAKY_JUNIT_XML\"; else printf '<testsuite><testcase classname=\"pkg\" name=\"x\"/></testsuite>' > \"$FLAKY_JUNIT_XML\"; fi\n"
	if err := os.WriteFile(script, []byte(body), 0755); err != nil {
		t.Fatal(err)
	}
	env := append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_AUTHOR_NAME=Fixture", "GIT_AUTHOR_EMAIL=fixture@example.test", "GIT_COMMITTER_NAME=Fixture", "GIT_COMMITTER_EMAIL=fixture@example.test", "GIT_AUTHOR_DATE=2020-01-01T00:00:00Z", "GIT_COMMITTER_DATE=2020-01-01T00:00:00Z")
	for _, args := range [][]string{{"-C", dir, "add", "fake.sh"}, {"-C", dir, "commit", "-qm", "rare failure"}} {
		c := exec.Command("git", args...)
		c.Env = env
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatal(string(out), err)
		}
	}
	_, err := Bisect(context.Background(), dir, "pkg::x", []string{"./fake.sh"}, "report.xml", 30)
	var uncertain InconclusiveError
	if !errors.As(err, &uncertain) {
		t.Fatalf("want inconclusive: %v", err)
	}
}

func TestBisectFromSubdirectory(t *testing.T) {
	for _, escape := range []bool{false, true} {
		t.Run(map[bool]string{false: "directory", true: "historical outside symlink"}[escape], func(t *testing.T) {
			dir := t.TempDir()
			for key, value := range map[string]string{
				"GIT_CONFIG_GLOBAL": "/dev/null", "GIT_AUTHOR_NAME": "Fixture", "GIT_AUTHOR_EMAIL": "fixture@example.test",
				"GIT_COMMITTER_NAME": "Fixture", "GIT_COMMITTER_EMAIL": "fixture@example.test",
				"GIT_AUTHOR_DATE": "2020-01-01T00:00:00Z", "GIT_COMMITTER_DATE": "2020-01-01T00:00:00Z",
			} {
				t.Setenv(key, value)
			}
			ctx := context.Background()
			if _, err := git(ctx, dir, "init", "-q"); err != nil {
				t.Fatal(err)
			}
			sub := filepath.Join(dir, "nested package")
			outside := t.TempDir()
			var err error
			if escape {
				err = os.Symlink(outside, sub)
			} else {
				err = os.Mkdir(sub, 0755)
			}
			if err != nil {
				t.Fatal(err)
			}
			for i, body := range []string{
				"printf '<testsuite><testcase classname=\"pkg\" name=\"x\"/></testsuite>'",
				"if [ $((FLAKY_RUN_INDEX % 3)) -eq 0 ]; then printf '<testsuite><testcase classname=\"pkg\" name=\"x\"><failure>bad</failure></testcase></testsuite>'; else printf '<testsuite><testcase classname=\"pkg\" name=\"x\"/></testsuite>'; fi",
			} {
				if escape && i == 1 {
					if err := os.Remove(sub); err != nil {
						t.Fatal(err)
					}
					if err := os.Mkdir(sub, 0755); err != nil {
						t.Fatal(err)
					}
				}
				script := "#!/bin/sh\n" + body + " > \"$FLAKY_JUNIT_XML\"\n"
				if err := os.WriteFile(filepath.Join(sub, "fake.sh"), []byte(script), 0755); err != nil {
					t.Fatal(err)
				}
				if _, err := git(ctx, dir, "add", "."); err != nil {
					t.Fatal(err)
				}
				if _, err := git(ctx, dir, "commit", "-qm", []string{"stable", "flaky"}[i]); err != nil {
					t.Fatal(err)
				}
			}
			head, err := git(ctx, dir, "rev-parse", "HEAD")
			if err != nil {
				t.Fatal(err)
			}
			result, err := Bisect(ctx, sub, "pkg::x", []string{"./fake.sh"}, "reports/junit.xml", 30)
			if escape {
				if err == nil || !strings.Contains(err.Error(), "escapes workspace") {
					t.Fatalf("unsafe historical working directory accepted: %v", err)
				}
				if _, err := os.Stat(filepath.Join(outside, "reports")); !os.IsNotExist(err) {
					t.Fatalf("external directory modified: %v", err)
				}
			} else if err != nil || result.FirstCommit != head {
				t.Fatalf("subdirectory bisect: first=%s, want=%s, err=%v", result.FirstCommit, head, err)
			}
			status, err := git(ctx, dir, "status", "--porcelain")
			if err != nil || status != "" {
				t.Fatalf("source changed: %q %v", status, err)
			}
		})
	}
}
