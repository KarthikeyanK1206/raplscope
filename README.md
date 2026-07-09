# raplscope

A Linux system energy profiler. Reads Intel RAPL counters from sysfs and
reports energy (joules) and power (watts) for a time window — or for exactly
as long as a wrapped command runs, like `/usr/bin/time` but for energy.

Single static binary. One Go package. Zero dependencies.

```
$ sudo raplscope -- gzip -9 big.iso

command        gzip -9 big.iso
exit code      0
package-0      412.33 J
total energy   412.33 J
average power  28.71 W
peak power     34.02 W
elapsed        14.363 s
samples        14
```

## Install

```
go install github.com/karthikeyan/raplscope@latest
```

Or clone and `go build`.

## Usage

```
raplscope [flags]                    monitor until Ctrl+C or -duration
raplscope [flags] [--] cmd [args]    measure while cmd runs
```

| Flag | Default | Meaning |
|---|---|---|
| `-interval` | `1s` | Sampling interval. Minimum 10 ms; below ~100 ms adds noise (RAPL updates about every 1 ms). |
| `-duration` | `0` | Monitor mode: stop after this long. `0` = until Ctrl+C. Cannot be combined with a command. |
| `-json` | off | Summary as indented JSON instead of a table. |
| `-csv` | `""` | Also write a per-sample time series (`elapsed_s,interval_s,power_w,cumulative_j`) to this file. |
| `-list` | off | Print discovered RAPL domains and exit. |
| `-powercap-path` | `/sys/class/powercap` | Override the sysfs root (testing/demo seam). |
| `-version` | off | Print version and exit. |

Exit codes: `0` success · `1` runtime error · `2` usage error · wrap mode
exits with the child's code, or `128+N` if the child died from signal `N`.

Monitor mode reports to **stdout** (live per-interval lines go to stderr, so
`raplscope -json | jq` stays clean). Wrap mode reports to **stderr**, so the
child owns stdout — same convention as `/usr/bin/time`.

## Example experiment: gzip vs zstd

Energy, not just time, of two compressors on the same input:

```
$ sudo raplscope -- gzip  -k -9 corpus.tar     # total energy 412 J, 14.4 s
$ sudo raplscope -- zstd  -k -19 corpus.tar    # total energy 305 J, 9.8 s
```

Same file, roughly comparable ratios — and a concrete joule count for the
difference. `-csv power.csv` gives you the per-second power curve to plot.

## How it works

Modern Intel (Sandy Bridge+) and AMD (Zen 2+) CPUs expose **RAPL** (Running
Average Power Limit) energy counters. The Linux `powercap` driver publishes
them in sysfs:

```
/sys/class/powercap/intel-rapl:0/
├── name                  "package-0"  (the whole CPU socket)
├── energy_uj             cumulative microjoules since boot, modulo wrap
└── max_energy_range_uj   the counter's ceiling
```

raplscope reads `energy_uj` at the start and end of the measurement window;
the difference is the energy consumed. Details it gets right:

- **Wraparound.** Counters wrap to zero at `max_energy_range_uj` — at a busy
  ~100 W package that's roughly every 45 minutes. Every delta is
  wrap-corrected.
- **Boundary snapshots.** Totals come from snapshots at the window edges, not
  from summing ticker samples, so a 50 ms wrapped command measures correctly
  even at a 1 s interval.
- **Monotonic time.** Power = ΔE/Δt uses Go's monotonic clock readings, never
  the nominal ticker interval.
- **No double counting.** Only top-level `package-*` domains are summed.
  `psys` (which already contains the packages), core/uncore/dram subdomains,
  and the `intel-rapl-mmio` duplicates are all excluded.

## Limitations (read this)

- **System-wide, not per-process.** RAPL counts everything on the socket.
  Wrapping a command measures system energy *while it ran* — background load
  included. RAPL physically cannot attribute energy to one process.
- **Root required.** Since the PLATYPUS side-channel fix (CVE-2020-8694),
  `energy_uj` is readable only by root.
- **Bare metal only.** VMs, most containers, WSL2 and ARM machines don't
  expose RAPL.
- **Peak power is interval-averaged.** With `-interval 1s`, "peak" means the
  hungriest one-second window, not a microsecond spike. With fewer than two
  full intervals, peak falls back to the average.
- **Package domains only.** GPU, DRAM and platform (psys) energy are out of
  scope.

## Development

All logic is testable without hardware or root: tests generate a fake
powercap tree in a temp directory and point the reader at it via
`-powercap-path` — that one injectable path is the entire testing seam.

```
go build ./... && go vet ./... && go test ./...
```

Real-hardware smoke test: `sudo ./raplscope -list`, then
`sudo ./raplscope -duration 5s`, then `sudo ./raplscope -- sleep 3`
(elapsed should be ~3 s and the exit code should propagate:
`sudo ./raplscope -- sh -c 'exit 7'; echo $?` prints 7).

See `docs/architecture.html` for the full design walkthrough.

## License

MIT
