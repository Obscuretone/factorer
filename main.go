package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"math/big"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"time"
)

func main() {
	workers := flag.Int("workers", runtime.NumCPU(), "number of parallel workers")
	batch := flag.Uint64("batch", 4096, "x values assigned to a worker at a time")
	maxSteps := flag.Uint64("max-steps", 1_000_000, "maximum x values to test from the starting point; 0 means unlimited")
	minDiff := flag.String("min-diff", "", "optional known lower bound for |p-q|; decimal or 0x-prefixed")
	flag.Parse()

	if flag.NArg() != 1 {
		fmt.Fprintf(os.Stderr, "usage: %s [flags] ODD_SEMIPRIME\n", os.Args[0])
		flag.PrintDefaults()
		os.Exit(2)
	}

	n, ok := parseBig(flag.Arg(0))
	if !ok {
		exitf("invalid semiprime: %q", flag.Arg(0))
	}

	var minDiffN *big.Int
	if strings.TrimSpace(*minDiff) != "" {
		var ok bool
		minDiffN, ok = parseBig(*minDiff)
		if !ok {
			exitf("invalid -min-diff: %q", *minDiff)
		}
	}

	opts := Options{
		Workers: *workers,
		Batch:   *batch,
		MinDiff: minDiffN,
	}
	if *maxSteps > 0 {
		start := fermatStart(n, minDiffN)
		opts.MaxX = new(big.Int).Add(start, new(big.Int).SetUint64(*maxSteps-1))
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	started := time.Now()
	res, err := FactorFermatParallel(ctx, n, opts)
	elapsed := time.Since(started)
	if err != nil {
		if errors.Is(err, errSearchExhausted) {
			fmt.Fprintf(os.Stderr, "not found after %d tested x values in %s\n", res.Steps, elapsed.Round(time.Microsecond))
			os.Exit(1)
		}
		exitf("%v", err)
	}

	fmt.Printf("p: %s\n", res.P.String())
	fmt.Printf("q: %s\n", res.Q.String())
	fmt.Printf("x: %s\n", res.X.String())
	fmt.Printf("n: %s\n", res.N.String())
	fmt.Printf("checked: %d\n", res.Steps)
	fmt.Printf("workers: %d\n", res.Workers)
	fmt.Printf("elapsed: %s\n", elapsed.Round(time.Microsecond))
}

func parseBig(s string) (*big.Int, bool) {
	n := new(big.Int)
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		_, ok := n.SetString(s[2:], 16)
		return n, ok
	}
	_, ok := n.SetString(s, 10)
	return n, ok
}

func exitf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
