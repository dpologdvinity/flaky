package flaky

import (
	"runtime/debug"
	"testing"
)

func TestCLI(t *testing.T) {
	for _, tc := range []struct {
		args           []string
		mode           string
		runs, parallel int
		bad            bool
	}{
		{[]string{"-n", "3", "--", "echo"}, "scan", 3, 1, false},
		{[]string{"--parallel", "2", "--", "echo"}, "scan", 20, 2, false},
		{[]string{"bisect", "pkg::x", "--runs", "30", "--", "echo"}, "bisect", 30, 1, false},
		{[]string{"-n", "0", "--", "echo"}, "", 0, 0, true},
		{[]string{"echo"}, "", 0, 0, true},
		{[]string{"--runs", "30", "--", "echo"}, "", 0, 0, true},
		{[]string{"bisect", "pkg::x", "-n", "30", "--", "echo"}, "", 0, 0, true},
		{[]string{"--junit", "../outside.xml", "--", "echo"}, "", 0, 0, true},
	} {
		o, err := parseArgs(tc.args)
		if (err != nil) != tc.bad {
			t.Fatalf("%v: %v", tc.args, err)
		}
		if err == nil && (o.Mode != tc.mode || o.Runs != tc.runs || o.Parallel != tc.parallel) {
			t.Fatalf("%+v", o)
		}
	}
}

func TestModuleVersion(t *testing.T) {
	for _, c := range []struct {
		info *debug.BuildInfo
		ok   bool
		want string
	}{
		{&debug.BuildInfo{Main: debug.Module{Version: "v0.1.3"}}, true, "0.1.3"},
		{&debug.BuildInfo{Main: debug.Module{Version: "(devel)"}}, true, ""},
		{&debug.BuildInfo{}, true, ""},
		{nil, false, ""},
	} {
		if got := moduleVersion(c.info, c.ok); got != c.want {
			t.Fatalf("moduleVersion(%+v) = %q, want %q", c.info, got, c.want)
		}
	}
}
