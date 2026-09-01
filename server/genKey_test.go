package server

import (
	"crypto/rand"
	"math/big"
	"testing"
)

// BenchmarkRandomDigits measures the optimized single-read approach.
func BenchmarkRandomDigits(b *testing.B) {
	for b.Loop() {
		key, err := randomDigits(9)
		if err != nil {
			b.Fatal(err)
		}
		_ = key
	}
}

// BenchmarkRandomDigitsOld measures the original 9-call approach for comparison.
func BenchmarkRandomDigitsOld(b *testing.B) {
	maxDigit := big.NewInt(10)
	for b.Loop() {
		digits := make([]byte, 9)
		for i := range 9 {
			num, err := rand.Int(rand.Reader, maxDigit)
			if err != nil {
				b.Fatal(err)
			}
			digits[i] = '0' + byte(num.Int64())
		}
		_ = string(digits)
	}
}
