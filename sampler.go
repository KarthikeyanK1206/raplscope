package main

import (
	"context"
	"fmt"
	"io"
	"time"
)

// sampleLoop takes an initial boundary snapshot, samples on a ticker until
// ctx is cancelled or duration elapses, then takes a final boundary snapshot.
// Totals come from the boundary snapshots; ticker samples exist for peak
// power, live lines and CSV rows.
func sampleLoop(ctx context.Context, r *Reader, acc *Accumulator, interval, duration time.Duration, live io.Writer, csvL *csvLogger) error {
	first, err := r.Read()
	if err != nil {
		return err
	}
	acc.Add(first)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	var deadline <-chan time.Time
	if duration > 0 {
		timer := time.NewTimer(duration)
		defer timer.Stop()
		deadline = timer.C
	}

	for {
		select {
		case <-ctx.Done():
			return finishSample(r, acc)
		case <-deadline:
			return finishSample(r, acc)
		case <-ticker.C:
			s, err := r.Read()
			if err != nil {
				return err
			}
			iv := acc.Add(s)
			if live != nil {
				fmt.Fprintf(live, "%7.1fs  %10.2f J  %8.2f W\n", iv.ElapsedSec, iv.Joules, iv.Watts)
			}
			if csvL != nil {
				if err := csvL.row(iv); err != nil {
					return err
				}
			}
		}
	}
}

func finishSample(r *Reader, acc *Accumulator) error {
	s, err := r.Read()
	if err != nil {
		return err
	}
	acc.Finish(s)
	return nil
}
