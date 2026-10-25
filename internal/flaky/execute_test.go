package flaky

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMainUsage(t *testing.T) {
	var out, err bytes.Buffer
	if code := Main(context.Background(), []string{"--help"}, &out, &err); code != 0 || !strings.Contains(out.String(), "flaky [-n N]") {
		t.Fatal(code, out.String())
	}
	out.Reset()
	if code := Main(context.Background(), []string{"-n", "0", "--", "fake"}, &out, &err); code != 2 {
		t.Fatal(code, err.String())
	}
}

func chdir(t *testing.T, dir string) {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(old)
	})
}

func TestMainExecuteScan(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	script := filepath.Join(dir, "test.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nprintf '<testsuite><testcase classname=\"pkg\" name=\"x\"/></testsuite>' > report.xml\n"), 0755); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	code := Main(context.Background(), []string{"-n", "2", "--junit", "report.xml", "--", script}, &out, &errOut)
	if code != 0 {
		t.Fatalf("expected code 0, got %d: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "FLAKY  2 runs · 1 test · 0 flaky") {
		t.Fatalf("unexpected output: %s", out.String())
	}

	out.Reset()
	errOut.Reset()
	code = Main(context.Background(), []string{"-n", "2", "--junit", "report.xml", "--json", "--", script}, &out, &errOut)
	if code != 0 {
		t.Fatalf("expected code 0 for json, got %d: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), `"version":"`+Version+`"`) || !strings.Contains(out.String(), `"id":"pkg::x"`) {
		t.Fatalf("unexpected json output: %s", out.String())
	}
}

func TestMainExecuteMissingReport(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	script := filepath.Join(dir, "fail.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nexit 1\n"), 0755); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	code := Main(context.Background(), []string{"-n", "1", "--junit", "report.xml", "--", script}, &out, &errOut)
	if code != 2 {
		t.Fatalf("expected code 2, got %d: %s", code, errOut.String())
	}
	if !strings.Contains(errOut.String(), "JUnit report missing") {
		t.Fatalf("unexpected err output: %s", errOut.String())
	}
}

func TestMainExecuteCancellation(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	script := filepath.Join(dir, "slow.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nsleep 10\n"), 0755); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var out, errOut bytes.Buffer
	code := Main(ctx, []string{"-n", "1", "--junit", "report.xml", "--", script}, &out, &errOut)
	if code != 130 {
		t.Fatalf("expected code 130, got %d: %s", code, errOut.String())
	}
}
func TestRenderBisect(t *testing.T) {
	parent := "abcdef012345"
	r := BisectResult{TestID: "pkg::x", RunsPerCommit: 30, FirstCommit: "123456789abcd", ParentCommit: &parent, Observations: []Observation{{Commit: parent, Failures: 0, Lower: 0, Upper: .083, Classification: "stable"}, {Commit: "123456789abcd", Subject: "introduce", Failures: 8, Lower: .14, Upper: .42, Classification: "flaky"}}}
	var b bytes.Buffer
	if err := RenderBisect(&b, r, false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), "FIRST   1234567  introduce") || !strings.Contains(b.String(), "PARENT  abcdef0  stable") {
		t.Fatal(b.String())
	}
}

func TestRenderInconclusive(t *testing.T) {
	var b bytes.Buffer
	r := BisectResult{TestID: "pkg::x", RunsPerCommit: 30}
	o := Observation{Commit: "abcdef012345", Failures: 1, Lower: .006, Upper: .17}
	if err := RenderInconclusive(&b, r, o); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), "BISECT  pkg::x · 30 runs/commit") || !strings.Contains(b.String(), "INCONCLUSIVE abcdef0 (1/30, 0.6%–17.0%)") {
		t.Fatal(b.String())
	}
}
