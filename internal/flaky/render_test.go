package flaky

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestRenderScan(t *testing.T) {
	s := Summarize([][]Case{{{ID: "pkg::x", Status: "fail", Output: "bad"}}, {{ID: "pkg::x", Status: "pass", Output: "good"}}})
	var b bytes.Buffer
	if err := RenderScan(&b, []string{"fake"}, 2, s, false); err != nil {
		t.Fatal(err)
	}
	for _, part := range []string{"FLAKY  2 runs · 1 test · 1 flaky", "50.0%", "1/1", "pkg::x", "fail: bad", "pass: good"} {
		if !strings.Contains(b.String(), part) {
			t.Fatalf("missing %q: %s", part, b.String())
		}
	}
	b.Reset()
	if err := RenderScan(&b, []string{"fake"}, 2, s, true); err != nil {
		t.Fatal(err)
	}
	var v map[string]any
	if err := json.Unmarshal(b.Bytes(), &v); err != nil {
		t.Fatal(err)
	}
	if v["flaky"].(float64) != 1 {
		t.Fatal(v)
	}
}
