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
