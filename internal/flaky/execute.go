package flaky

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
)

func execute(ctx context.Context, o Options, out, errOut io.Writer) int {
	dir, err := os.Getwd()
	if err != nil {
		_, _ = fmt.Fprintln(errOut, "flaky:", err)
		return 2
	}
	if o.Mode == "scan" {
		tests, e := Scan(ctx, dir, o.Command, o.JUnit, o.Runs, o.Parallel)
		if e != nil {
			err = e
		} else {
			err = RenderScan(out, o.Command, o.Runs, tests, o.JSON)
		}
	} else {
		result, e := Bisect(ctx, dir, o.TestID, o.Command, o.JUnit, o.Runs)
		if e != nil {
			var inconclusive InconclusiveError
			if errors.As(e, &inconclusive) {
				_ = RenderInconclusive(errOut, result, inconclusive.Observation)
				return 3
			}
			err = e
		} else {
			err = RenderBisect(out, result, o.JSON)
		}
	}
	if err == nil {
		return 0
	}
	_, _ = fmt.Fprintln(errOut, "flaky:", err)
	if ctx.Err() != nil {
		return 130
	}
	var missing MissingTestError
	if errors.As(err, &missing) {
		return 1
	}
	return 2
}
