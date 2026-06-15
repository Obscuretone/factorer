package main

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"runtime"
	"testing"
)

func TestFactorFermatParallelSmallSemiprimes(t *testing.T) {
	tests := []struct {
		n string
		p string
		q string
	}{
		{"35", "5", "7"},
		{"77", "7", "11"},
		{"1000036000099", "1000003", "1000033"},
	}

	for _, tt := range tests {
		n, _ := new(big.Int).SetString(tt.n, 10)
		res, err := FactorFermatParallel(context.Background(), n, Options{Workers: 4, Batch: 2, MaxX: nil})
		if err != nil {
			t.Fatalf("FactorFermatParallel(%s): %v", tt.n, err)
		}
		if res.P.String() != tt.p || res.Q.String() != tt.q {
			t.Fatalf("FactorFermatParallel(%s) = (%s, %s), want (%s, %s)", tt.n, res.P, res.Q, tt.p, tt.q)
		}
		if new(big.Int).Mul(res.P, res.Q).Cmp(n) != 0 {
			t.Fatalf("factors do not multiply back to input")
		}
	}
}

func TestMinDiffCanSkipCloseFactors(t *testing.T) {
	n := big.NewInt(77)
	_, err := FactorFermatParallel(context.Background(), n, Options{
		Workers: 2,
		Batch:   1,
		MaxX:    big.NewInt(20),
		MinDiff: big.NewInt(10),
	})
	if !errors.Is(err, errSearchExhausted) {
		t.Fatalf("expected exhausted search, got %v", err)
	}
}

func TestRejectsEvenAndTinyInputs(t *testing.T) {
	for _, n := range []*big.Int{big.NewInt(0), big.NewInt(1), big.NewInt(100)} {
		_, err := FactorFermatParallel(context.Background(), n, Options{})
		if !errors.Is(err, errNotSemiprimeCandidate) {
			t.Fatalf("expected candidate error for %s, got %v", n, err)
		}
	}
}

func TestSquareResidueTableAcceptsSquares(t *testing.T) {
	for i := 0; i < squareResidueModulus*2; i++ {
		residue := (i * i) % squareResidueModulus
		if !squareResidueTable[residue] {
			t.Fatalf("square residue %d was rejected", residue)
		}
	}
}

func BenchmarkFactorFermatParallel(b *testing.B) {
	p := big.NewInt(1_000_000_007)
	q := big.NewInt(1_200_000_007)
	n := new(big.Int).Mul(p, q)

	for _, workers := range []int{1, runtime.NumCPU()} {
		b.Run(fmt.Sprintf("workers=%d", workers), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				res, err := FactorFermatParallel(context.Background(), n, Options{
					Workers: workers,
					Batch:   2048,
				})
				if err != nil {
					b.Fatal(err)
				}
				if new(big.Int).Mul(res.P, res.Q).Cmp(n) != 0 {
					b.Fatal("factors do not multiply back to input")
				}
			}
		})
	}
}
