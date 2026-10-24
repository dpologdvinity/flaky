package flaky

import (
	"bytes"
	"context"
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
