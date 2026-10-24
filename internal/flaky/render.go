package flaky

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
)

func RenderScan(w io.Writer, command []string, runs int, tests []Summary, jsonMode bool) error {
	count := 0
	for _, s := range tests {
		if s.Flaky {
			count++
		}
	}
	if jsonMode {
		return json.NewEncoder(w).Encode(struct {
			Version string    `json:"version"`
			Command []string  `json:"command"`
			Runs    int       `json:"runs"`
			Tests   []Summary `json:"tests"`
			Flaky   int       `json:"flaky"`
		}{Version, command, runs, tests, count})
	}
	if _, err := fmt.Fprintf(w, "FLAKY  %d runs · %d tests · %d flaky\n", runs, len(tests), count); err != nil {
		return err
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "RATE\tFAIL/PASS\tMISS\tTEST")
	for _, s := range tests {
		rate := "—"
		if s.FailureRate != nil {
			rate = fmt.Sprintf("%.1f%%", *s.FailureRate*100)
		}
		fmt.Fprintf(tw, "%s\t%d/%d\t%d\t%s\n", rate, s.Failures, s.Passes, s.Missing, s.ID)
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	for _, s := range tests {
		if !s.Flaky {
			continue
		}
		fail := s.FailureOutput
		pass := s.PassOutput
		if fail == "" {
			fail = "(no output)"
		}
		if pass == "" {
			pass = "(no output)"
		}
		if _, err := fmt.Fprintf(w, "\n%s\n  fail: %s\n  pass: %s\n", s.ID, strings.ReplaceAll(fail, "\n", "\n  "), strings.ReplaceAll(pass, "\n", "\n  ")); err != nil {
			return err
		}
	}
	return nil
}
