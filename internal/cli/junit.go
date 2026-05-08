package cli

import (
	"encoding/xml"
	"fmt"
	"io"
	"time"
)

// junitSuite is the minimal JUnit XML schema accepted by every CI ingestor.
type junitSuite struct {
	XMLName  xml.Name    `xml:"testsuite"`
	Name     string      `xml:"name,attr"`
	Tests    int         `xml:"tests,attr"`
	Failures int         `xml:"failures,attr"`
	Errors   int         `xml:"errors,attr"`
	Time     float64     `xml:"time,attr"`
	Cases    []junitCase `xml:"testcase"`
}

type junitCase struct {
	XMLName   xml.Name      `xml:"testcase"`
	Classname string        `xml:"classname,attr"`
	Name      string        `xml:"name,attr"`
	Time      float64       `xml:"time,attr"`
	Failure   *junitFailure `xml:"failure,omitempty"`
	SystemOut string        `xml:"system-out,omitempty"`
}

type junitFailure struct {
	Message string `xml:",chardata"`
}

// TestResult is the per-test datum the test runner records and the JUnit
// writer consumes.
type TestResult struct {
	Name     string
	Passed   bool
	Duration time.Duration
	Logs     string
	Error    string
}

func writeJUnit(w io.Writer, releaseName string, results []TestResult) error {
	suite := junitSuite{Name: "keramos-test/" + releaseName}
	var total time.Duration
	for _, r := range results {
		c := junitCase{
			Classname: "keramos",
			Name:      r.Name,
			Time:      r.Duration.Seconds(),
			SystemOut: r.Logs,
		}
		if !r.Passed {
			suite.Failures++
			c.Failure = &junitFailure{Message: r.Error}
			if "" == c.Failure.Message {
				c.Failure.Message = "test did not pass"
			}
		}
		suite.Cases = append(suite.Cases, c)
		total += r.Duration
	}
	suite.Tests = len(results)
	suite.Time = total.Seconds()

	if _, err := fmt.Fprint(w, xml.Header); nil != err {
		return err
	}
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	if err := enc.Encode(suite); nil != err {
		return err
	}
	_, _ = fmt.Fprintln(w)
	return nil
}
