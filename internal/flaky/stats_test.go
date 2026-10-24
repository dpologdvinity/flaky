package flaky

import "testing"

func TestSummarize(t *testing.T) {
	runs := [][]Case{{{ID: "a", Status: "pass", Output: "ok"}, {ID: "b", Status: "fail", Output: "bad"}}, {{ID: "a", Status: "fail", Output: "oops"}, {ID: "b", Status: "fail", Output: "bad"}, {ID: "c", Status: "skip"}}, {{ID: "a", Status: "pass"}}}
	s := Summarize(runs)
	if len(s) != 3 || s[0].ID != "a" || s[0].Failures != 1 || s[0].Passes != 2 || s[0].FailureOutput != "oops" || s[1].ID != "b" || s[1].Missing != 1 || s[2].FailureRate != nil {
		t.Fatalf("%+v", s)
	}
}
func TestWilson(t *testing.T) {
	lo, hi := Wilson(0, 30)
	if lo != 0 || hi >= .10 {
		t.Fatalf("%f %f", lo, hi)
	}
	if Classify(0, 30, 30) != "stable" || Classify(8, 22, 30) != "flaky" || Classify(1, 29, 30) != "inconclusive" || Classify(0, 29, 30) != "inconclusive" {
		t.Fatal("classification")
	}
}
