package main

import (
	"fmt"
	"io"
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
