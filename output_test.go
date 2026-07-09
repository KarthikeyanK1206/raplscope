package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func sampleResult() Result {
	code := 0
	return Result{
		Domains:     []DomainResult{{Name: "package-0", Joules: 12.5}},
		TotalJoules: 12.5,
		AvgWatts:    2.5,
		PeakWatts:   3.1,
		ElapsedSec:  5,
		Samples:     5,
		Command:     "sleep 5",
		ExitCode:    &code,
	}
}

func TestWriteJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := writeJSON(&buf, sampleResult()); err != nil {
		t.Fatal(err)
	}
	want := `{
  "domains": [
    {
      "name": "package-0",
      "joules": 12.5
    }
  ],
  "total_joules": 12.5,
  "average_watts": 2.5,
  "peak_watts": 3.1,
  "elapsed_seconds": 5,
  "samples": 5,
  "command": "sleep 5",
  "exit_code": 0
}
`
	if buf.String() != want {
		t.Errorf("JSON mismatch:\ngot:\n%s\nwant:\n%s", buf.String(), want)
	}
}

func TestWriteJSONOmitsWrapFields(t *testing.T) {
	r := sampleResult()
	r.Command = ""
	r.ExitCode = nil
	var buf bytes.Buffer
	if err := writeJSON(&buf, r); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "command") || strings.Contains(buf.String(), "exit_code") {
		t.Errorf("monitor-mode JSON leaked wrap-mode fields:\n%s", buf.String())
	}
}

func TestWriteTable(t *testing.T) {
	var buf bytes.Buffer
	writeTable(&buf, sampleResult())
	out := buf.String()
	for _, want := range []string{
		"command", "sleep 5", "exit code",
		"package-0", "12.50 J",
		"total energy", "average power", "2.50 W",
		"peak power", "3.10 W", "elapsed", "5.000 s", "samples",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("table missing %q:\n%s", want, out)
		}
	}
}

func TestCSVLogger(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.csv")
	l, err := newCSVLogger(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := l.row(Interval{ElapsedSec: 1, Seconds: 1, Watts: 15.5, CumulativeJ: 15.5}); err != nil {
		t.Fatal(err)
	}
	if err := l.row(Interval{ElapsedSec: 2.001, Seconds: 1.001, Watts: 20, CumulativeJ: 35.52}); err != nil {
		t.Fatal(err)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "elapsed_s,interval_s,power_w,cumulative_j\n" +
		"1.000,1.000,15.50,15.500\n" +
		"2.001,1.001,20.00,35.520\n"
	if string(b) != want {
		t.Errorf("CSV mismatch:\ngot:\n%s\nwant:\n%s", b, want)
	}
}
