package main

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// writeFakeDomain builds one fake powercap domain directory. The tree is
// generated at test time rather than committed under testdata/ because the
// directory names contain ':', which breaks checkouts on Windows.
func writeFakeDomain(t *testing.T, root, id, name string, energyUJ, maxUJ uint64) string {
	t.Helper()
	dir := filepath.Join(root, id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(dir, "name"), name+"\n")
	writeTestFile(t, filepath.Join(dir, "energy_uj"), strconv.FormatUint(energyUJ, 10)+"\n")
	writeTestFile(t, filepath.Join(dir, "max_energy_range_uj"), strconv.FormatUint(maxUJ, 10)+"\n")
	return dir
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDiscoverReader(t *testing.T) {
	root := t.TempDir()
	writeFakeDomain(t, root, "intel-rapl:0", "package-0", 1000, 262143328850)
	writeFakeDomain(t, root, "intel-rapl:1", "package-1", 2000, 262143328850)
	writeFakeDomain(t, root, "intel-rapl:0:0", "core", 500, 262143328850) // subdomain: skip
	writeFakeDomain(t, root, "intel-rapl:2", "psys", 9000, 262143328850)  // psys: skip
	writeFakeDomain(t, root, "intel-rapl-mmio:0", "package-0", 1000, 262143328850) // mmio duplicate: skip
	if err := os.MkdirAll(filepath.Join(root, "dtpm"), 0o755); err != nil { // unrelated entry: skip
		t.Fatal(err)
	}

	r, err := DiscoverReader(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Domains) != 2 {
		t.Fatalf("got %d domains, want 2: %+v", len(r.Domains), r.Domains)
	}
	if r.Domains[0].Name != "package-0" || r.Domains[1].Name != "package-1" {
		t.Errorf("wrong domains: %+v", r.Domains)
	}
	if r.Domains[0].ID != "intel-rapl:0" {
		t.Errorf("ID = %q, want intel-rapl:0", r.Domains[0].ID)
	}
	if r.Domains[0].MaxRangeUJ != 262143328850 {
		t.Errorf("MaxRangeUJ = %d, want 262143328850", r.Domains[0].MaxRangeUJ)
	}
}

func TestDiscoverReaderNoRAPL(t *testing.T) {
	t.Run("empty root", func(t *testing.T) {
		if _, err := DiscoverReader(t.TempDir()); !errors.Is(err, ErrNoRAPL) {
			t.Errorf("err = %v, want ErrNoRAPL", err)
		}
	})
	t.Run("missing root", func(t *testing.T) {
		if _, err := DiscoverReader(filepath.Join(t.TempDir(), "nope")); !errors.Is(err, ErrNoRAPL) {
			t.Errorf("err = %v, want ErrNoRAPL", err)
		}
	})
	t.Run("psys only", func(t *testing.T) {
		root := t.TempDir()
		writeFakeDomain(t, root, "intel-rapl:0", "psys", 1, 1000)
		if _, err := DiscoverReader(root); !errors.Is(err, ErrNoRAPL) {
			t.Errorf("err = %v, want ErrNoRAPL", err)
		}
	})
}

func TestRead(t *testing.T) {
	root := t.TempDir()
	writeFakeDomain(t, root, "intel-rapl:0", "package-0", 123456, 1000000)
	writeFakeDomain(t, root, "intel-rapl:1", "package-1", 789, 1000000)

	r, err := DiscoverReader(root)
	if err != nil {
		t.Fatal(err)
	}
	s, err := r.Read()
	if err != nil {
		t.Fatal(err)
	}
	if s.UJ[0] != 123456 || s.UJ[1] != 789 {
		t.Errorf("UJ = %v, want [123456 789]", s.UJ)
	}
	if s.At.IsZero() {
		t.Error("snapshot timestamp is zero")
	}
}

func TestReadPermissionDenied(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root; file permissions are not enforced")
	}
	root := t.TempDir()
	dir := writeFakeDomain(t, root, "intel-rapl:0", "package-0", 1, 1000)
	if err := os.Chmod(filepath.Join(dir, "energy_uj"), 0); err != nil {
		t.Fatal(err)
	}

	r, err := DiscoverReader(root)
	if err != nil {
		t.Fatal(err)
	}
	_, err = r.Read()
	if err == nil {
		t.Fatal("Read succeeded, want permission error")
	}
	if !strings.Contains(err.Error(), "run with sudo") {
		t.Errorf("error not actionable: %v", err)
	}
}
