# factorer

`factorer` is a parallel Fermat factorization implementation inspired by the
method described in
[Factoring Large Semiprimes](https://obscuretone.com/posts/en/hack_the_planet).

That article came from independently arriving at the same core idea: write an
odd semiprime as a difference of squares and search outward from
`ceil(sqrt(N))`. It felt like a breakthrough, and it is a real mathematical
insight, but it is also the classic Fermat factorization method.

It starts at `ceil(sqrt(N))` and searches for an `x` where `x^2 - N` is a perfect
square. When found:

```text
p = x - n
q = x + n
```

This is very fast when the two prime factors are close together. It is not a
general RSA-breaking algorithm; if the factors are far apart, the search remains
large.

RSA key generation intentionally chooses primes of roughly the same size so both
factors sit near `sqrt(N)`, but standards also require them not to be too close
together because that is exactly where Fermat factorization is strongest. See
[Fermat's factoring trick and cryptography](https://www.johndcook.com/blog/2018/10/28/fermat-factoring/)
for an approachable explanation, and NIST FIPS 186-5 for the corresponding RSA
prime-generation checks.

## Build

```sh
go build -o factorer .
```

## Run

```sh
./factorer 1000036000099
```

Useful flags:

```text
-workers N       goroutines to search with, defaults to CPU count
-batch N         candidate x values assigned to each worker at a time
-max-steps N     maximum x values to test from the starting point; 0 is unlimited
-min-diff N      optional lower bound for |p-q|, decimal or 0x-prefixed
```

Examples:

```sh
./factorer -workers 8 -batch 8192 1000036000099
./factorer -min-diff 1000 -max-steps 10000000 152415789666209426002111556165263283035677489
```

## Benchmark CPU scaling

```sh
go test -bench BenchmarkFactorFermatParallel -benchtime=1x -count=1
```

On an 8-core Apple Silicon machine, the included benchmark went from about
`0.20s` with one worker to about `0.036s` with eight workers.
