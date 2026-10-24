package flaky

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
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
		os.WriteFile(script, []byte(body+"\n# revision "+string(rune('0'+i))+"\n"), 0755)
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
