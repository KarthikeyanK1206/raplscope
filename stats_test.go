package main

import (
	"math"
	"testing"
	"time"
)

func TestWrapDelta(t *testing.T) {
	const max = uint64(1000)
	tests := []struct {
		name            string
		prev, cur, want uint64
	}{
		{"normal", 100, 250, 150},
		{"equal", 100, 100, 0},
		{"from zero", 0, 42, 42},
		{"wrap", 900, 50, 150},
		{"wrap to zero", 900, 0, 100},
		{"wrap from max", 1000, 5, 5},
	}
	for _, tt := range tests {
		if got := wrapDelta(tt.prev, tt.cur, max); got != tt.want {
			t.Errorf("%s: wrapDelta(%d, %d, %d) = %d, want %d", tt.name, tt.prev, tt.cur, max, got, tt.want)
		}
	}
}

func approx(t *testing.T, name string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("%s = %v, want %v", name, got, want)
	}
}

func TestAccumulator(t *testing.T) {
	domains := []Domain{
		{Name: "package-0", MaxRangeUJ: 1_000_000_000},
		{Name: "package-1", MaxRangeUJ: 1_000_000_000},
	}
	t0 := time.Now()
	snap := func(offset time.Duration, uj ...uint64) Snapshot {
		return Snapshot{At: t0.Add(offset), UJ: uj}
	}

	acc := NewAccumulator(domains)
	acc.Add(snap(0, 0, 0))

	iv := acc.Add(snap(1*time.Second, 10_000_000, 5_000_000)) // 15 J over 1 s
	approx(t, "interval watts", iv.Watts, 15)
	approx(t, "interval joules", iv.Joules, 15)

	acc.Add(snap(2*time.Second, 40_000_000, 5_000_000)) // 30 J over 1 s → peak

	// Final partial interval: 10 J over 0.5 s (20 W) must not become peak.
	acc.Finish(snap(2500*time.Millisecond, 45_000_000, 10_000_000))

	r := acc.Result()
	approx(t, "TotalJoules", r.TotalJoules, 55)
	approx(t, "ElapsedSec", r.ElapsedSec, 2.5)
	approx(t, "AvgWatts", r.AvgWatts, 22)
	approx(t, "PeakWatts", r.PeakWatts, 30)
	if r.Samples != 2 {
		t.Errorf("Samples = %d, want 2", r.Samples)
	}
	approx(t, "package-0 joules", r.Domains[0].Joules, 45)
	approx(t, "package-1 joules", r.Domains[1].Joules, 10)
}

func TestAccumulatorWrap(t *testing.T) {
	domains := []Domain{{Name: "package-0", MaxRangeUJ: 1000}}
	acc := NewAccumulator(domains)
	t0 := time.Now()
	acc.Add(Snapshot{At: t0, UJ: []uint64{900}})
	acc.Finish(Snapshot{At: t0.Add(time.Second), UJ: []uint64{100}})

	r := acc.Result()
	approx(t, "TotalJoules across wrap", r.TotalJoules, 0.0002) // (1000-900)+100 µJ
}

func TestAccumulatorPeakFallsBackToAvg(t *testing.T) {
	domains := []Domain{{Name: "package-0", MaxRangeUJ: 1_000_000_000}}
	t0 := time.Now()

	// Boundary snapshots only: a wrapped command shorter than one tick.
	acc := NewAccumulator(domains)
	acc.Add(Snapshot{At: t0, UJ: []uint64{0}})
	acc.Finish(Snapshot{At: t0.Add(500 * time.Millisecond), UJ: []uint64{5_000_000}})
	r := acc.Result()
	approx(t, "AvgWatts", r.AvgWatts, 10)
	approx(t, "PeakWatts (0 intervals)", r.PeakWatts, r.AvgWatts)

	// A single full interval is still too few for a meaningful peak.
	acc = NewAccumulator(domains)
	acc.Add(Snapshot{At: t0, UJ: []uint64{0}})
	acc.Add(Snapshot{At: t0.Add(time.Second), UJ: []uint64{30_000_000}})
	acc.Finish(Snapshot{At: t0.Add(1200 * time.Millisecond), UJ: []uint64{31_000_000}})
	r = acc.Result()
	approx(t, "PeakWatts (1 interval)", r.PeakWatts, r.AvgWatts)
}
