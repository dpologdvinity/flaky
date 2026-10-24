package flaky

import (
	"context"
	"io"
)

func execute(ctx context.Context, o Options, out, errOut io.Writer) int { return 2 }
