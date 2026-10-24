package flaky

import (
	"strings"
	"testing"
)

func TestParseJUnit(t *testing.T) {
	input := `<testsuites><testsuite><testcase classname=" pkg " name=" x "><failure message="bad">trace</failure></testcase><testsuite><testcase file="a.go" name="y"><skipped/></testcase><testcase name="z"><system-out>good</system-out></testcase></testsuite></testsuite></testsuites>`
	cases, err := ParseJUnit(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(cases) != 3 || cases[0].ID != "pkg::x" || cases[0].Output != "trace" || cases[1].ID != "a.go::y" || cases[1].Status != "skip" || cases[2].Output != "good" {
		t.Fatalf("%+v", cases)
	}
	for _, bad := range []string{`<foo/>`, `<testsuite/>`, `<testsuite><testcase name="x"/><testcase name="x"/></testsuite>`, `<testsuite><testcase/></testsuite>`, `<testsuite>`} {
		if _, err := ParseJUnit(strings.NewReader(bad)); err == nil {
			t.Fatalf("accepted %q", bad)
		}
	}
}
