package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

// wrapMode measures system energy while a child command runs, following the
// /usr/bin/time precedent: the child inherits stdio, SIGINT/SIGTERM are
// forwarded to it, the report goes to stderr so the child's stdout stays
// pipeable, and raplscope exits with the child's exit code.
func wrapMode(r *Reader, args []string, interval time.Duration) int {
	acc := NewAccumulator(r.Domains)

	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr

	first, err := r.Read()
	if err != nil {
		fmt.Fprintf(os.Stderr, "raplscope: %v\n", err)
		return 1
	}
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "raplscope: %v\n", err)
		return 1
	}
	acc.Add(first)

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigc)

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	var waitErr error
loop:
	for {
		select {
		case sig := <-sigc:
			// Forward to the child and keep waiting: the child decides
			// when to die, we report afterwards.
			cmd.Process.Signal(sig)
		case waitErr = <-done:
			break loop
		case <-ticker.C:
			if _, err := readAndAdd(r, acc); err != nil {
				fmt.Fprintf(os.Stderr, "raplscope: %v\n", err)
				return 1
			}
		}
	}

	final, err := r.Read()
	if err != nil {
		fmt.Fprintf(os.Stderr, "raplscope: %v\n", err)
		return 1
	}
	acc.Finish(final)

	exitCode := exitStatus(waitErr)
	res := acc.Result()
	res.Command = strings.Join(args, " ")
	res.ExitCode = &exitCode

	writeTable(os.Stderr, res)
	return exitCode
}

func readAndAdd(r *Reader, acc *Accumulator) (Interval, error) {
	s, err := r.Read()
	if err != nil {
		return Interval{}, err
	}
	return acc.Add(s), nil
}

// exitStatus maps the child's wait result to raplscope's own exit code:
// the child's code, or 128+N if the child died from signal N.
func exitStatus(err error) int {
	if err == nil {
		return 0
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		if ws, ok := ee.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
			return 128 + int(ws.Signal())
		}
		return ee.ExitCode()
	}
	return 1
}
