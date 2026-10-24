package flaky

import (
	"context"
	"fmt"
)

type Observation struct {
	Commit         string  `json:"commit"`
	Subject        string  `json:"subject"`
	Passes         int     `json:"passes"`
	Failures       int     `json:"failures"`
	Lower          float64 `json:"lower"`
	Upper          float64 `json:"upper"`
	Classification string  `json:"classification"`
}
type BisectResult struct {
	Version       string        `json:"version"`
	TestID        string        `json:"test_id"`
	RunsPerCommit int           `json:"runs_per_commit"`
	Threshold     float64       `json:"threshold"`
	Confidence    float64       `json:"confidence"`
	FirstCommit   string        `json:"first_commit"`
	ParentCommit  *string       `json:"parent_commit"`
	Observations  []Observation `json:"observations"`
}
type InconclusiveError struct{ Observation Observation }

func (e InconclusiveError) Error() string {
	return fmt.Sprintf("inconclusive at %s (%d failures, %.1f%%–%.1f%%)", e.Observation.Commit, e.Observation.Failures, e.Observation.Lower*100, e.Observation.Upper*100)
}

type MissingTestError struct{ ID string }

func (e MissingTestError) Error() string { return "test ID not found: " + e.ID }

func Bisect(ctx context.Context, dir, id string, command []string, junit string, runs int) (BisectResult, error) {
	result := BisectResult{Version: Version, TestID: id, RunsPerCommit: runs, Threshold: .10, Confidence: .90, Observations: []Observation{}}
	history, err := History(ctx, dir)
	if err != nil {
		return result, err
	}
	if len(history) < 2 {
		return result, fmt.Errorf("bisect needs at least two commits")
	}
	cache := map[int]Observation{}
	sample := func(index int) (Observation, error) {
		if o, ok := cache[index]; ok {
			return o, nil
		}
		commit := history[index]
		o := Observation{Commit: commit.Hash, Subject: commit.Subject}
		seen := 0
		for i := 0; i < runs; i++ {
			err := WithWorktree(ctx, dir, commit.Hash, func(work string) error {
				cases, e := runInWork(ctx, work, command, junit, i+1)
				if e != nil {
					return e
				}
				for _, c := range cases {
					if c.ID == id {
						seen++
						switch c.Status {
						case "pass":
							o.Passes++
						case "fail":
							o.Failures++
						}
						break
					}
				}
				return nil
			})
			if err != nil {
				return o, err
			}
		}
		if seen == 0 {
			return o, MissingTestError{id}
		}
		o.Lower, o.Upper = Wilson(o.Failures, runs)
		o.Classification = Classify(o.Failures, o.Passes, runs)
		cache[index] = o
		result.Observations = append(result.Observations, o)
		if o.Classification == "inconclusive" {
			return o, InconclusiveError{o}
		}
		return o, nil
	}
	first, err := sample(0)
	if err != nil {
		return result, err
	}
	if first.Classification != "stable" {
		return result, fmt.Errorf("oldest commit is not stable")
	}
	last, err := sample(len(history) - 1)
	if err != nil {
		return result, err
	}
	if last.Classification != "flaky" {
		return result, fmt.Errorf("HEAD is not flaky")
	}
	lo, hi := 0, len(history)-1
	for hi-lo > 1 {
		mid := (lo + hi) / 2
		o, e := sample(mid)
		if e != nil {
			return result, e
		}
		if o.Classification == "stable" {
			lo = mid
		} else {
			hi = mid
		}
	}
	result.FirstCommit = history[hi].Hash
	parent := history[hi-1].Hash
	result.ParentCommit = &parent
	return result, nil
}
