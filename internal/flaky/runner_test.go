package flaky

import (
	"context"
	"os"
	"path/filepath"
	"testing"
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
