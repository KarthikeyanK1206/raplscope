package main

// Pure measurement math. Nothing in this file does I/O.

// wrapDelta returns the energy consumed between two cumulative counter
// readings, correcting for a single counter wraparound at maxRange. RAPL
// counters wrap roughly every 45 minutes at a busy ~100 W package, so this
// happens in real use.
func wrapDelta(prev, cur, maxRange uint64) uint64 {
	if cur >= prev {
		return cur - prev
	}
	return (maxRange - prev) + cur
}

// Interval describes what happened between two consecutive snapshots.
type Interval struct {
	ElapsedSec  float64 // seconds since the first snapshot
	Seconds     float64 // length of this interval (actual, not nominal)
	Joules      float64 // energy consumed in this interval, all domains
	Watts       float64 // average power over this interval
	CumulativeJ float64 // total energy since the first snapshot
}

// Accumulator folds snapshots into wrap-corrected totals and interval stats.
// Totals are defined by the boundary snapshots (first and last); ticker
// samples in between exist for peak power, live output and CSV rows.
type Accumulator struct {
	domains   []Domain
	first     Snapshot
	last      Snapshot
	haveFirst bool
	totalUJ   []uint64 // per-domain, wrap-corrected
	peakW     float64
	samples   int // full ticker intervals folded in via Add
}

func NewAccumulator(domains []Domain) *Accumulator {
	return &Accumulator{domains: domains, totalUJ: make([]uint64, len(domains))}
}

// Add folds in a snapshot from the ticker loop. The first call records the
// starting boundary; later calls accumulate a wrap-corrected delta and update
// peak power.
func (a *Accumulator) Add(s Snapshot) Interval {
	if !a.haveFirst {
		a.first, a.last, a.haveFirst = s, s, true
		return Interval{}
	}
	iv := a.fold(s)
	a.samples++
	if iv.Watts > a.peakW {
		a.peakW = iv.Watts
	}
	return iv
}

// Finish folds in the final boundary snapshot. It completes the totals but is
// excluded from peak power: the closing interval is a partial tick whose tiny
// Δt would make its power reading noise rather than signal.
func (a *Accumulator) Finish(s Snapshot) {
	if !a.haveFirst {
		a.first, a.last, a.haveFirst = s, s, true
		return
	}
	a.fold(s)
}

func (a *Accumulator) fold(s Snapshot) Interval {
	var deltaUJ uint64
	for i := range a.domains {
		d := wrapDelta(a.last.UJ[i], s.UJ[i], a.domains[i].MaxRangeUJ)
		a.totalUJ[i] += d
		deltaUJ += d
	}
	// Actual elapsed time from the monotonic clock, never the nominal
	// ticker interval: tickers jitter.
	dt := s.At.Sub(a.last.At).Seconds()
	iv := Interval{
		ElapsedSec: s.At.Sub(a.first.At).Seconds(),
		Seconds:    dt,
		Joules:     float64(deltaUJ) / 1e6,
	}
	if dt > 0 {
		iv.Watts = iv.Joules / dt
	}
	a.last = s
	var total uint64
	for _, uj := range a.totalUJ {
		total += uj
	}
	iv.CumulativeJ = float64(total) / 1e6
	return iv
}

// DomainResult is the per-domain slice of a Result.
type DomainResult struct {
	Name   string  `json:"name"`
	Joules float64 `json:"joules"`
}

// Result is the finished measurement summary rendered by output.go.
type Result struct {
	Domains     []DomainResult `json:"domains"`
	TotalJoules float64        `json:"total_joules"`
	AvgWatts    float64        `json:"average_watts"`
	PeakWatts   float64        `json:"peak_watts"` // interval-averaged, not instantaneous
	ElapsedSec  float64        `json:"elapsed_seconds"`
	Samples     int            `json:"samples"`
	Command     string         `json:"command,omitempty"`   // wrap mode only
	ExitCode    *int           `json:"exit_code,omitempty"` // wrap mode only
}

func (a *Accumulator) Result() Result {
	r := Result{Samples: a.samples}
	var totalUJ uint64
	for i, d := range a.domains {
		totalUJ += a.totalUJ[i]
		r.Domains = append(r.Domains, DomainResult{Name: d.Name, Joules: float64(a.totalUJ[i]) / 1e6})
	}
	r.TotalJoules = float64(totalUJ) / 1e6
	if a.haveFirst {
		r.ElapsedSec = a.last.At.Sub(a.first.At).Seconds()
	}
	if r.ElapsedSec > 0 {
		r.AvgWatts = r.TotalJoules / r.ElapsedSec
	}
	r.PeakWatts = a.peakW
	if a.samples < 2 {
		// Too few full intervals for a meaningful peak (e.g. a wrapped
		// command shorter than one tick); fall back to the average.
		r.PeakWatts = r.AvgWatts
	}
	return r
}
