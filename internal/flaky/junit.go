package flaky

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"
)

type Case struct{ ID, Status, Message, Output string }
type xmlEvent struct {
	XMLName xml.Name
	Message string `xml:"message,attr"`
	Body    string `xml:",chardata"`
}
type xmlCase struct {
	Name    string     `xml:"name,attr"`
	Class   string     `xml:"classname,attr"`
	File    string     `xml:"file,attr"`
	Skipped *xmlEvent  `xml:"skipped"`
	Out     string     `xml:"system-out"`
	Events  []xmlEvent `xml:",any"`
}
type xmlSuite struct {
	XMLName xml.Name
	Cases   []xmlCase  `xml:"testcase"`
	Suites  []xmlSuite `xml:"testsuite"`
}

func ParseJUnit(r io.Reader) ([]Case, error) {
	decoder := xml.NewDecoder(r)
	var suite xmlSuite
	if err := decoder.Decode(&suite); err != nil {
		return nil, err
	}
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if text, ok := token.(xml.CharData); ok && strings.TrimSpace(string(text)) == "" {
			continue
		}
		if _, ok := token.(xml.Comment); ok {
			continue
		}
		if _, ok := token.(xml.ProcInst); ok {
			continue
		}
		return nil, errors.New("trailing content after JUnit report")
	}
	if suite.XMLName.Local != "testsuite" && suite.XMLName.Local != "testsuites" {
		return nil, errors.New("JUnit root must be testsuite or testsuites")
	}
	seen := map[string]bool{}
	var out []Case
	var walk func(xmlSuite) error
	walk = func(s xmlSuite) error {
		for _, x := range s.Cases {
			name := strings.TrimSpace(x.Name)
			if name == "" {
				return errors.New("JUnit testcase has empty name")
			}
			prefix := strings.TrimSpace(x.Class)
			if prefix == "" {
				prefix = strings.TrimSpace(x.File)
			}
			id := name
			if prefix != "" {
				id = prefix + "::" + name
			}
			if seen[id] {
				return fmt.Errorf("duplicate JUnit testcase: %s", id)
			}
			seen[id] = true
			c := Case{ID: id, Status: "pass"}
			for _, event := range x.Events {
				if event.XMLName.Local != "failure" && event.XMLName.Local != "error" {
					continue
				}
				c.Status = "fail"
				if c.Message == "" {
					c.Message = strings.TrimSpace(event.Message)
				}
				if c.Output == "" {
					c.Output = strings.TrimSpace(event.Body)
				}
			}
			if c.Status == "fail" {
				if c.Output == "" {
					c.Output = c.Message
				}
				c.Output = capOutput(c.Output)
			} else if x.Skipped != nil {
				c.Status = "skip"
			} else {
				c.Output = capOutput(strings.TrimSpace(x.Out))
			}
			out = append(out, c)
		}
		for _, child := range s.Suites {
			if err := walk(child); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(suite); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, errors.New("JUnit report contains no testcases")
	}
	return out, nil
}
func capOutput(s string) string {
	if len(s) > 8192 {
		return s[:8192]
	}
	return s
}
