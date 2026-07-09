// Command raplscope measures system energy consumption on Linux using the
// Intel RAPL counters exposed under /sys/class/powercap.
//
// Two modes:
//
//	raplscope [flags]                    monitor until Ctrl+C or -duration
//	raplscope [flags] [--] cmd [args]    measure while cmd runs
package main

import (
	"flag"
	"fmt"
	"os"
	"time"
)

const version = "0.1.0"

const defaultPowercapPath = "/sys/class/powercap"

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(argv []string) int {
	fs := flag.NewFlagSet("raplscope", flag.ExitOnError)
	interval := fs.Duration("interval", time.Second, "sampling interval (minimum 10ms)")
	duration := fs.Duration("duration", 0, "monitor mode: stop after this long (0 = until Ctrl+C)")
	jsonOut := fs.Bool("json", false, "print the summary as indented JSON instead of a table")
	csvPath := fs.String("csv", "", "also write a per-sample time series to this CSV file")
	list := fs.Bool("list", false, "print discovered RAPL domains and exit")
	powercapPath := fs.String("powercap-path", defaultPowercapPath, "powercap sysfs root (testing seam)")
	showVersion := fs.Bool("version", false, "print version and exit")
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), `raplscope — measure system energy and power via Intel RAPL

usage:
  raplscope [flags]                    monitor until Ctrl+C or -duration
  raplscope [flags] [--] cmd [args]    measure while cmd runs

flags:
`)
		fs.PrintDefaults()
	}
	fs.Parse(argv)

	if *showVersion {
		fmt.Println("raplscope " + version)
		return 0
	}

	args := fs.Args()
	if *interval < 10*time.Millisecond {
		fmt.Fprintln(os.Stderr, "raplscope: -interval must be at least 10ms")
		return 2
	}
	if *duration < 0 {
		fmt.Fprintln(os.Stderr, "raplscope: -duration must not be negative")
		return 2
	}
	if len(args) > 0 && *duration != 0 {
		fmt.Fprintln(os.Stderr, "raplscope: -duration cannot be combined with a command; the command's runtime defines the window")
		return 2
	}

	reader, err := DiscoverReader(*powercapPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "raplscope: %v\n", err)
		return 1
	}

	if *list {
		for _, d := range reader.Domains {
			fmt.Printf("%s\t%s\tmax_energy_range %d µJ\n", d.ID, d.Name, d.MaxRangeUJ)
		}
		return 0
	}

	_ = *jsonOut
	_ = *csvPath
	fmt.Fprintln(os.Stderr, "raplscope: measurement modes arrive in later milestones")
	return 1
}
