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

func TestParseJUnitPrefersErrorBody(t *testing.T) {
	cases, err := ParseJUnit(strings.NewReader(`<testsuite><testcase name="x"><failure message="failure message"/><error>error trace</error></testcase></testsuite>`))
	if err != nil || cases[0].Output != "error trace" {
		t.Fatalf("%+v: %v", cases, err)
	}
}

func TestParseJUnitRejectsTrailingAndFailureWins(t *testing.T) {
	if _, err := ParseJUnit(strings.NewReader(`<testsuite><testcase name="x"/></testsuite><broken>`)); err == nil {
		t.Fatal("accepted trailing XML")
	}
	cases, err := ParseJUnit(strings.NewReader(`<testsuite><testcase name="x"><skipped/><system-out>pass text</system-out><failure>failure trace</failure></testcase></testsuite>`))
	if err != nil || cases[0].Status != "fail" || cases[0].Output != "failure trace" {
		t.Fatalf("%+v: %v", cases, err)
	}
}

func TestParseJUnitFirstFailureBody(t *testing.T) {
	cases, err := ParseJUnit(strings.NewReader(`<testsuite><testcase name="x"><error>first trace</error><failure>second trace</failure></testcase></testsuite><!-- okay -->`))
	if err != nil || cases[0].Output != "first trace" {
		t.Fatalf("%+v: %v", cases, err)
	}
}
