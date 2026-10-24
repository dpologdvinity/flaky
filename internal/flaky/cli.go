package flaky

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"
)

const Version = "0.1.0"

type Options struct {
	Mode, TestID, JUnit string
	Runs, Parallel      int
	JSON                bool
	Command             []string
}

func parseArgs(args []string) (Options, error) {
	o := Options{Mode: "scan", Runs: 20, Parallel: 1}
	if len(args) > 0 && args[0] == "bisect" {
		o.Mode = "bisect"
		o.Runs = 30
		args = args[1:]
	}
	split := -1
	for i, arg := range args {
		if arg == "--" {
			split = i
			break
		}
	}
	flags := args
	if split >= 0 {
		flags = args[:split]
		o.Command = args[split+1:]
	}
	fs := flag.NewFlagSet("flaky", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.IntVar(&o.Runs, "n", o.Runs, "number of runs")
	fs.IntVar(&o.Runs, "runs", o.Runs, "number of runs")
	fs.IntVar(&o.Parallel, "parallel", 1, "parallel workers")
	fs.StringVar(&o.JUnit, "junit", "", "relative JUnit report path")
	fs.BoolVar(&o.JSON, "json", false, "JSON output")
	if o.Mode == "bisect" {
		if len(flags) == 0 {
			return o, errors.New("bisect requires TEST-ID")
		}
		o.TestID = flags[0]
		flags = flags[1:]
	}
	if err := fs.Parse(flags); err != nil {
		return o, err
	}
	var modeErr error
	fs.Visit(func(f *flag.Flag) {
		if o.Mode == "scan" && f.Name == "runs" {
			modeErr = errors.New("--runs is only valid for bisect")
		}
		if o.Mode == "bisect" && (f.Name == "n" || f.Name == "parallel") {
			modeErr = fmt.Errorf("--%s is only valid for scan", f.Name)
		}
	})
	if modeErr != nil {
		return o, modeErr
	}
	if len(fs.Args()) > 0 {
		return o, fmt.Errorf("unexpected argument: %s", fs.Args()[0])
	}
	if split < 0 || len(o.Command) == 0 {
		return o, errors.New("command required after --")
	}
	if o.Runs <= 0 || o.Runs > 1000 {
		return o, errors.New("runs must be between 1 and 1000")
	}
	if o.Parallel <= 0 {
		return o, errors.New("parallel must be positive")
	}
	if o.Mode == "bisect" && (o.Runs < 30 || o.Parallel != 1) {
		return o, errors.New("bisect requires --runs >= 30 and does not support --parallel")
	}
	if o.JUnit != "" && !filepath.IsLocal(o.JUnit) {
		return o, errors.New("--junit must stay within the workspace")
	}
	return o, nil
}

func Main(ctx context.Context, args []string, out, errOut io.Writer) int {
	if len(args) == 1 && (args[0] == "--version" || args[0] == "-V") {
		fmt.Fprintln(out, "flaky "+Version)
		return 0
	}
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Fprintln(out, "flaky [-n N] [--parallel P] [--junit RELPATH] [--json] -- COMMAND [ARG...]\nflaky bisect TEST-ID [--runs N] [--junit RELPATH] [--json] -- COMMAND [ARG...]")
		return 0
	}
	o, err := parseArgs(args)
	if err != nil {
		fmt.Fprintln(errOut, "flaky: "+err.Error())
		return 2
	}
	return execute(ctx, o, out, errOut)
}
