package flaky

import (
	"math"
	"sort"
)

type Summary struct {
	ID            string   `json:"id"`
	Passes        int      `json:"passes"`
	Failures      int      `json:"failures"`
	Skips         int      `json:"skips"`
	Missing       int      `json:"missing"`
	FailureRate   *float64 `json:"failure_rate"`
	Flaky         bool     `json:"flaky"`
	FailureOutput string   `json:"failure_output"`
	PassOutput    string   `json:"pass_output"`
}

func Summarize(runs [][]Case) []Summary {
	byID := map[string]*Summary{}
	for _, run := range runs {
		for _, c := range run {
			s := byID[c.ID]
			if s == nil {
				s = &Summary{ID: c.ID}
				byID[c.ID] = s
			}
			switch c.Status {
			case "pass":
				s.Passes++
				if s.Passes == 1 {
					s.PassOutput = c.Output
				}
			case "fail":
				s.Failures++
				if s.Failures == 1 {
					s.FailureOutput = c.Output
				}
			case "skip":
				s.Skips++
			}
		}
	}
	out := make([]Summary, 0, len(byID))
	for _, s := range byID {
		s.Missing = len(runs) - s.Passes - s.Failures - s.Skips
		n := s.Passes + s.Failures
		if n > 0 {
			r := float64(s.Failures) / float64(n)
			s.FailureRate = &r
		}
		s.Flaky = s.Passes > 0 && s.Failures > 0
		out = append(out, *s)
	}
	sort.Slice(out, func(i, j int) bool {
		rank := func(s Summary) int {
			if s.Flaky {
				return 0
			}
			if s.Failures > 0 {
				return 1
			}
			if s.Passes > 0 {
				return 2
			}
			return 3
		}
		a, b := rank(out[i]), rank(out[j])
		if a != b {
			return a < b
		}
		if a == 0 && *out[i].FailureRate != *out[j].FailureRate {
			return *out[i].FailureRate > *out[j].FailureRate
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func Wilson(f, n int) (float64, float64) {
	if n <= 0 {
		return 0, 1
	}
	z := 1.6448536269514722
	q := float64(f) / float64(n)
	d := 1 + z*z/float64(n)
	c := (q + z*z/(2*float64(n))) / d
	h := z * math.Sqrt(q*(1-q)/float64(n)+z*z/(4*float64(n*n))) / d
	if f == 0 {
		return 0, math.Min(1, c+h)
	}
	if f == n {
		return math.Max(0, c-h), 1
	}
	return math.Max(0, c-h), math.Min(1, c+h)
}
func Classify(f, p, n int) string {
	if f+p != n {
		return "inconclusive"
	}
	low, high := Wilson(f, n)
	if high < 0.10 {
		return "stable"
	}
	if p > 0 && f > 0 && low > 0.10 {
		return "flaky"
	}
	return "inconclusive"
}
