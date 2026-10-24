package flaky

import (
	"context"
	"fmt"
	"sync"
)

func Scan(ctx context.Context, source string, command []string, junit string, n, parallel int) ([]Summary, error) {
	if n <= 0 || parallel <= 0 {
		return nil, fmt.Errorf("runs and parallel must be positive")
	}
	type result struct {
		index int
		cases []Case
		err   error
	}
	jobs := make(chan int, n)
	results := make(chan result, n)
	for i := 0; i < n; i++ {
		jobs <- i
	}
	close(jobs)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var wg sync.WaitGroup
	for w := 0; w < parallel && w < n; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				if ctx.Err() != nil {
					results <- result{index: i, err: ctx.Err()}
					continue
				}
				cases, err := Run(ctx, source, command, junit)
				results <- result{index: i, cases: cases, err: err}
			}
		}()
	}
	go func() { wg.Wait(); close(results) }()
	runs := make([][]Case, n)
	var first error
	for r := range results {
		if r.err != nil && first == nil {
			first = fmt.Errorf("run %d: %w", r.index+1, r.err)
			cancel()
		}
		runs[r.index] = r.cases
	}
	if first != nil {
		return nil, first
	}
	return Summarize(runs), nil
}
