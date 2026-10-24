package flaky

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRun(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "fake.sh")
	os.WriteFile(script, []byte("#!/bin/sh\nprintf '<testsuite><testcase name=\"x\"><failure>bad</failure></testcase></testsuite>' > \"$FLAKY_JUNIT_XML\"\nexit 1\n"), 0755)
	cases, err := Run(context.Background(), dir, []string{script}, "report.xml")
	if err != nil || len(cases) != 1 || cases[0].Status != "fail" {
		t.Fatal(cases, err)
	}
	os.WriteFile(script, []byte("#!/bin/sh\nexit 0\n"), 0755)
	if _, err := Run(context.Background(), dir, []string{script}, "report.xml"); err == nil {
		t.Fatal("missing report accepted")
	}
}
func TestRunPytestAdapter(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "pytest")
	os.WriteFile(script, []byte("#!/bin/sh\ncase \"$1\" in --junitxml=*) p=${1#--junitxml=};; esac\nprintf '<testsuite><testcase name=\"x\"/></testsuite>' > \"$p\"\n"), 0755)
	if _, err := Run(context.Background(), dir, []string{script}, ""); err != nil {
		t.Fatal(err)
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
	for i := 0; i < 100; i++ {
		b, err := os.ReadFile(marker)
		if err == nil {
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
