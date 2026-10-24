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
	testWord := "tests"
	if len(tests) == 1 {
		testWord = "test"
	}
	if _, err := fmt.Fprintf(w, "FLAKY  %d runs · %d %s · %d flaky\n", runs, len(tests), testWord, count); err != nil {
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

func RenderBisect(w io.Writer, result BisectResult, jsonMode bool) error {
	if jsonMode {
		return json.NewEncoder(w).Encode(result)
	}
	if _, err := fmt.Fprintf(w, "BISECT  %s · %d runs/commit · threshold 10%% (Wilson 90%%)\n", result.TestID, result.RunsPerCommit); err != nil {
		return err
	}
	var first, parent Observation
	for _, o := range result.Observations {
		if o.Commit == result.FirstCommit {
			first = o
		}
		if result.ParentCommit != nil && o.Commit == *result.ParentCommit {
			parent = o
		}
	}
	if _, err := fmt.Fprintf(w, "FIRST   %.7s  %s\n", result.FirstCommit, first.Subject); err != nil {
		return err
	}
	if result.ParentCommit != nil {
		if _, err := fmt.Fprintf(w, "PARENT  %.7s  stable (%d/%d, %.1f%%–%.1f%%)\n", parent.Commit, parent.Failures, result.RunsPerCommit, parent.Lower*100, parent.Upper*100); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintf(w, "FIRST   %.7s  flaky (%d/%d, %.1f%%–%.1f%%)\n", first.Commit, first.Failures, result.RunsPerCommit, first.Lower*100, first.Upper*100)
	return err
}
