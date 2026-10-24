package flaky

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestScan(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "fake.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nprintf '<testsuite><testcase classname=\"pkg\" name=\"x\"/></testsuite>' > \"$FLAKY_JUNIT_XML\"\n"), 0755); err != nil {
		t.Fatal(err)
	}
	s, err := Scan(context.Background(), dir, []string{script}, "report.xml", 4, 2)
	if err != nil || len(s) != 1 || s[0].Passes != 4 {
		t.Fatal(s, err)
	}
}
