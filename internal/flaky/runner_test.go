package flaky

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestRunReportsLaunchError(t *testing.T) {
	for _, tc := range []struct {
		name string
		run  func(context.Context, string, []string, string) ([]Case, error)
	}{
		{"scan", Run},
		{"bisect", func(ctx context.Context, work string, command []string, junit string) ([]Case, error) {
			return runInWork(ctx, work, command, junit, 1)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			command := filepath.Join(dir, "missing-command")
			_, err := tc.run(context.Background(), dir, []string{command}, "report.xml")
			if !errors.Is(err, os.ErrNotExist) || !strings.Contains(err.Error(), command) || strings.Contains(err.Error(), "JUnit report") {
				t.Fatalf("expected command launch error, got %v", err)
			}
		})
	}
}

func TestRunBoundsCommandOutput(t *testing.T) {
	for _, report := range []bool{false, true} {
		name := "missing report"
		if report {
			name = "valid report"
		}
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			script := filepath.Join(dir, "noisy.sh")
			body := "#!/bin/sh\nprintf 'old diagnostic\\n'\ni=0\nwhile [ \"$i\" -lt 10000 ]; do\nprintf '🌍'\ni=$((i+1))\ndone\nprintf 'final stdout\\n'\nprintf 'final stderr\\n' >&2\n"
			if report {
				body += "printf '<testsuite><testcase name=\"x\"/></testsuite>' > \"$FLAKY_JUNIT_XML\"\n"
			}
			body += "exit 1\n"
			if err := os.WriteFile(script, []byte(body), 0755); err != nil {
				t.Fatal(err)
			}
			cases, err := Run(context.Background(), dir, []string{script}, "report.xml")
			if report {
				if err != nil || len(cases) != 1 || cases[0].Status != "pass" {
					t.Fatalf("%+v: %v", cases, err)
				}
				return
			}
			if err == nil {
				t.Fatal("missing report accepted")
			}
			message := err.Error()
			if !utf8.ValidString(message) || len(message) > 9*1024 || strings.Contains(message, "old diagnostic") || !strings.Contains(message, "final stdout") || !strings.Contains(message, "final stderr") {
				t.Fatalf("expected bounded diagnostic tail with both streams; got %d bytes", len(message))
			}
		})
	}
}

func TestRun(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "fake.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nprintf '<testsuite><testcase name=\"x\"><failure>bad</failure></testcase></testsuite>' > \"$FLAKY_JUNIT_XML\"\nexit 1\n"), 0755); err != nil {
		t.Fatal(err)
	}
	cases, err := Run(context.Background(), dir, []string{script}, "report.xml")
	if err != nil || len(cases) != 1 || cases[0].Status != "fail" {
		t.Fatal(cases, err)
	}
	if err := os.WriteFile(script, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := Run(context.Background(), dir, []string{script}, "report.xml"); err == nil {
		t.Fatal("missing report accepted")
	}
}
func TestRunPytestAdapter(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"pytest", "pytest.exe"} {
		script := filepath.Join(dir, name)
		if err := os.WriteFile(script, []byte("#!/bin/sh\ncase \"$1\" in --junitxml=*) p=${1#--junitxml=};; esac\nprintf '<testsuite><testcase name=\"x\"/></testsuite>' > \"$p\"\n"), 0755); err != nil {
			t.Fatal(err)
		}
		if _, err := Run(context.Background(), dir, []string{script}, ""); err != nil {
			t.Fatal(name, err)
		}
	}
}

func TestRunCancellationCleansWorkspace(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "marker")
	t.Setenv("FLAKY_MARKER", marker)
	script := filepath.Join(dir, "wait.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho \"$PWD\" > \"$FLAKY_MARKER\"\nsleep 10\n"), 0755); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { _, err := Run(ctx, dir, []string{script}, "report.xml"); done <- err }()
	var path string
	for i := 0; i < 200; i++ {
		b, err := os.ReadFile(marker)
		if err == nil && len(strings.TrimSpace(string(b))) > 0 {
			path = strings.TrimSpace(string(b))
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if path == "" {
		cancel()
		t.Fatal("command did not start")
	}
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected cancellation")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("cancellation hung")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("workspace remains: %s: %v", path, err)
	}
}

func TestRunRejectsSymlinkedReport(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(dir, "reports")); err != nil {
		t.Fatal(err)
	}
	protected := filepath.Join(outside, "report.xml")
	if err := os.WriteFile(protected, []byte("protected"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Run(context.Background(), dir, []string{"true"}, "reports/report.xml"); err == nil {
		t.Fatal("accepted symlinked report")
	}
	b, err := os.ReadFile(protected)
	if err != nil || string(b) != "protected" {
		t.Fatalf("external report changed: %q %v", b, err)
	}
}

func TestRunKillsBackgroundChild(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(t.TempDir(), "survived")
	t.Setenv("FLAKY_MARKER", marker)
	script := filepath.Join(dir, "spawn.sh")
	body := "#!/bin/sh\n(sleep 0.3; echo survived > \"$FLAKY_MARKER\") >/dev/null 2>&1 &\nprintf '<testsuite><testcase name=\"x\"/></testsuite>' > \"$FLAKY_JUNIT_XML\"\n"
	if err := os.WriteFile(script, []byte(body), 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := Run(context.Background(), dir, []string{script}, "report.xml"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(500 * time.Millisecond)
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("background child survived: %v", err)
	}
}
