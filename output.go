package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"text/tabwriter"
)

// writeTable renders the human-readable summary.
func writeTable(w io.Writer, r Result) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw)
	if r.Command != "" {
		fmt.Fprintf(tw, "command\t%s\n", r.Command)
		if r.ExitCode != nil {
			fmt.Fprintf(tw, "exit code\t%d\n", *r.ExitCode)
		}
	}
	for _, d := range r.Domains {
		fmt.Fprintf(tw, "%s\t%.2f J\n", d.Name, d.Joules)
	}
	fmt.Fprintf(tw, "total energy\t%.2f J\n", r.TotalJoules)
	fmt.Fprintf(tw, "average power\t%.2f W\n", r.AvgWatts)
	fmt.Fprintf(tw, "peak power\t%.2f W\n", r.PeakWatts)
	fmt.Fprintf(tw, "elapsed\t%.3f s\n", r.ElapsedSec)
	fmt.Fprintf(tw, "samples\t%d\n", r.Samples)
	tw.Flush()
}

// writeJSON renders the summary as indented JSON.
func writeJSON(w io.Writer, r Result) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

// csvLogger writes the per-sample time series: one row per ticker interval,
// flushed per row so the file is usable even if raplscope is killed.
type csvLogger struct {
	f *os.File
	w *csv.Writer
}

func newCSVLogger(path string) (*csvLogger, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	l := &csvLogger{f: f, w: csv.NewWriter(f)}
	if err := l.writeRow("elapsed_s", "interval_s", "power_w", "cumulative_j"); err != nil {
		f.Close()
		return nil, err
	}
	return l, nil
}

func (l *csvLogger) row(iv Interval) error {
	return l.writeRow(
		strconv.FormatFloat(iv.ElapsedSec, 'f', 3, 64),
		strconv.FormatFloat(iv.Seconds, 'f', 3, 64),
		strconv.FormatFloat(iv.Watts, 'f', 2, 64),
		strconv.FormatFloat(iv.CumulativeJ, 'f', 3, 64),
	)
}

func (l *csvLogger) writeRow(fields ...string) error {
	if err := l.w.Write(fields); err != nil {
		return err
	}
	l.w.Flush()
	return l.w.Error()
}

func (l *csvLogger) Close() error {
	err := l.w.Error()
	if cerr := l.f.Close(); err == nil {
		err = cerr
	}
	return err
}
