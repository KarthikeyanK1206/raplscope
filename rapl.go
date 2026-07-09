package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ErrNoRAPL is returned when the powercap tree has no usable package domains.
var ErrNoRAPL = errors.New("no Intel RAPL domains found — raplscope needs bare-metal Linux on Intel (Sandy Bridge or newer) or modern AMD (Zen 2 or newer); VMs, containers and ARM are unsupported")

// topLevel matches top-level RAPL domains like "intel-rapl:0". Subdomains
// such as "intel-rapl:0:0" (core, uncore, dram) carry a second colon and the
// mmio duplicates live under "intel-rapl-mmio:*"; both are excluded so no
// energy is counted twice.
var topLevel = regexp.MustCompile(`^intel-rapl:[0-9]+$`)

// Domain is one top-level RAPL package domain.
type Domain struct {
	Name       string // from the domain's name file, e.g. "package-0"
	ID         string // sysfs directory name, e.g. "intel-rapl:0"
	energyPath string
	MaxRangeUJ uint64 // counter ceiling from max_energy_range_uj; the wrap point
}

// Reader reads raw cumulative energy counters for the discovered domains.
// It does no math: wrap correction and deltas live in stats.go.
type Reader struct {
	Domains []Domain
}

// Snapshot is one raw reading of every domain at a single instant.
type Snapshot struct {
	At time.Time // carries Go's monotonic clock reading
	UJ []uint64  // cumulative microjoules, parallel to Reader.Domains
}

// DiscoverReader scans root (normally /sys/class/powercap) for top-level
// package-* RAPL domains. psys is skipped: it already includes the packages,
// so summing both would double count.
func DiscoverReader(root string) (*Reader, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrNoRAPL
		}
		return nil, fmt.Errorf("reading %s: %w", root, err)
	}
	var domains []Domain
	for _, e := range entries {
		if !topLevel.MatchString(e.Name()) {
			continue
		}
		dir := filepath.Join(root, e.Name())
		name, err := readStringFile(filepath.Join(dir, "name"))
		if err != nil {
			return nil, fmt.Errorf("reading domain name: %w", err)
		}
		if !strings.HasPrefix(name, "package") {
			continue
		}
		maxRange, err := readUintFile(filepath.Join(dir, "max_energy_range_uj"))
		if err != nil {
			return nil, fmt.Errorf("reading max energy range for %s: %w", name, err)
		}
		domains = append(domains, Domain{
			Name:       name,
			ID:         e.Name(),
			energyPath: filepath.Join(dir, "energy_uj"),
			MaxRangeUJ: maxRange,
		})
	}
	if len(domains) == 0 {
		return nil, ErrNoRAPL
	}
	return &Reader{Domains: domains}, nil
}

// Read takes one raw reading of all domains.
func (r *Reader) Read() (Snapshot, error) {
	s := Snapshot{At: time.Now(), UJ: make([]uint64, len(r.Domains))}
	for i, d := range r.Domains {
		v, err := readUintFile(d.energyPath)
		if err != nil {
			if errors.Is(err, os.ErrPermission) {
				return Snapshot{}, errors.New("permission denied reading energy counters — run with sudo (the kernel restricts RAPL to root since the PLATYPUS fix, CVE-2020-8694)")
			}
			return Snapshot{}, err
		}
		s.UJ[i] = v
	}
	return s, nil
}

func readStringFile(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

func readUintFile(path string) (uint64, error) {
	str, err := readStringFile(path)
	if err != nil {
		return 0, err
	}
	v, err := strconv.ParseUint(str, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parsing %s: %w", path, err)
	}
	return v, nil
}
