package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"
	"runtime"
	"sync"
	"sync/atomic"
)

var (
	errNotSemiprimeCandidate = errors.New("number must be an odd integer greater than 1")
	errSearchExhausted       = errors.New("no factors found before search limit")
)

const squareResidueModulus = 64 * 9 * 5 * 7 * 11 * 13

var squareResidueTable = buildSquareResidueTable()

type Options struct {
	Workers int
	Batch   uint64
	MaxX    *big.Int
	MinDiff *big.Int
}

type Result struct {
	P       *big.Int
	Q       *big.Int
	X       *big.Int
	N       *big.Int
	Steps   uint64
	Workers int
}

type workChunk struct {
	start uint64
	count uint64
}

func FactorFermatParallel(ctx context.Context, n *big.Int, opts Options) (Result, error) {
	if err := validateCandidate(n); err != nil {
		return Result{}, err
	}

	workers := opts.Workers
	if workers <= 0 {
		workers = runtime.NumCPU()
	}
	batch := opts.Batch
	if batch == 0 {
		batch = 4096
	}

	startX := fermatStart(n, opts.MinDiff)
	limited := opts.MaxX != nil
	var maxOffset uint64 = math.MaxUint64
	if opts.MaxX != nil {
		if opts.MaxX.Cmp(startX) < 0 {
			return Result{}, errSearchExhausted
		}
		delta := new(big.Int).Sub(opts.MaxX, startX)
		if !delta.IsUint64() {
			return Result{}, fmt.Errorf("max x range is too large for this implementation")
		}
		maxOffset = delta.Uint64()
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	chunks := make(chan workChunk, workers*2)
	found := make(chan Result, 1)
	var checked atomic.Uint64
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if res, ok := workerSearch(ctx, n, startX, chunks, &checked, workers); ok {
				select {
				case found <- res:
					cancel()
				default:
				}
			}
		}()
	}

	go func() {
		defer close(chunks)
		if !limited {
			for offset := uint64(0); ; offset += batch {
				select {
				case <-ctx.Done():
					return
				case chunks <- workChunk{start: offset, count: batch}:
				}
				if math.MaxUint64-offset < batch {
					return
				}
			}
		}

		for offset := uint64(0); offset <= maxOffset; {
			count := batch
			remainingAfterStart := maxOffset - offset
			if remainingAfterStart < count {
				count = remainingAfterStart + 1
			}
			select {
			case <-ctx.Done():
				return
			case chunks <- workChunk{start: offset, count: count}:
			}
			if remainingAfterStart < count {
				return
			}
			offset += count
		}
	}()

	go func() {
		wg.Wait()
		close(found)
	}()

	if err := ctx.Err(); err != nil && !errors.Is(err, context.Canceled) {
		return Result{}, err
	}

	res, ok := <-found
	if ok {
		cancel()
		wg.Wait()
		res.Steps = checked.Load()
		return res, nil
	}

	return Result{Steps: checked.Load(), Workers: workers}, errSearchExhausted
}

func workerSearch(ctx context.Context, n, startX *big.Int, chunks <-chan workChunk, checked *atomic.Uint64, workers int) (Result, bool) {
	x := new(big.Int)
	step := new(big.Int)
	y2 := new(big.Int)
	y := new(big.Int)
	yy := new(big.Int)
	p := new(big.Int)
	q := new(big.Int)
	offset := new(big.Int)
	modulus := big.NewInt(squareResidueModulus)
	two := big.NewInt(2)
	one := big.NewInt(1)
	var localChecked uint64

	flushChecked := func() {
		if localChecked > 0 {
			checked.Add(localChecked)
			localChecked = 0
		}
	}
	defer flushChecked()

	for chunk := range chunks {
		offset.SetUint64(chunk.start)
		x.Add(startX, offset)
		y2.Mul(x, x)
		y2.Sub(y2, n)
		step.Lsh(x, 1)
		step.Add(step, one)
		y2Mod := new(big.Int).Mod(y2, modulus).Uint64()
		stepMod := new(big.Int).Mod(step, modulus).Uint64()

		for i := uint64(0); i < chunk.count; i++ {
			select {
			case <-ctx.Done():
				return Result{}, false
			default:
			}

			localChecked++
			if localChecked >= 1024 {
				flushChecked()
			}

			if squareResidueTable[y2Mod] {
				y.Sqrt(y2)
				yy.Mul(y, y)
				if yy.Cmp(y2) == 0 {
					p.Sub(x, y)
					q.Add(x, y)
					if p.Cmp(one) > 0 && q.Cmp(one) > 0 {
						flushChecked()
						return Result{
							P:       new(big.Int).Set(p),
							Q:       new(big.Int).Set(q),
							X:       new(big.Int).Set(x),
							N:       new(big.Int).Set(y),
							Workers: workers,
						}, true
					}
				}
			}
			y2.Add(y2, step)
			step.Add(step, two)
			y2Mod += stepMod
			y2Mod %= squareResidueModulus
			stepMod += 2
			stepMod %= squareResidueModulus
			x.Add(x, one)
		}
	}

	return Result{}, false
}

func buildSquareResidueTable() []bool {
	table := make([]bool, squareResidueModulus)
	for i := 0; i < squareResidueModulus; i++ {
		table[(i*i)%squareResidueModulus] = true
	}
	return table
}

func fermatStart(n, minDiff *big.Int) *big.Int {
	start := ceilSqrt(n)
	if minDiff == nil || minDiff.Sign() <= 0 {
		return start
	}

	minHalfDiff := new(big.Int).Add(minDiff, big.NewInt(1))
	minHalfDiff.Rsh(minHalfDiff, 1)
	x2 := new(big.Int).Mul(minHalfDiff, minHalfDiff)
	x2.Add(x2, n)
	withMinDiff := ceilSqrt(x2)
	if withMinDiff.Cmp(start) > 0 {
		return withMinDiff
	}
	return start
}

func ceilSqrt(n *big.Int) *big.Int {
	root := new(big.Int).Sqrt(n)
	if new(big.Int).Mul(root, root).Cmp(n) == 0 {
		return root
	}
	return root.Add(root, big.NewInt(1))
}

func validateCandidate(n *big.Int) error {
	if n == nil || n.Cmp(big.NewInt(1)) <= 0 || n.Bit(0) == 0 {
		return errNotSemiprimeCandidate
	}
	return nil
}
