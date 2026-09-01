package server

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"math/big"
	"strings"
)

// gen_key generates a cryptographically secure random 9-digit channel
// key. Unlike the original Go implementation (which used math/rand and
// a 7-digit key), the key mirrors the Python server's 9-digit format,
// but the digits are drawn from crypto/rand instead of the
// non-cryptographic PRNGs used by both original implementations.
//
// The key is rejected if any existing channel uses it as its name or
// its password, mirroring the Python server's check_key, which
// compared against channel passwords (in the Python server the channel
// name and password are the same value). The loop is unbounded, like
// the Python server's while loop; with a billion possible keys and
// cryptographically random draws, a collision is vanishingly unlikely.
func gen_key() (string, error) {
	for {
		key, err := randomDigits(9)
		if err != nil {
			return "", err
		}
		if !channelKeyTaken(key) {
			return key, nil
		}
	}
}

// randomDigits returns a string of n uniformly random decimal digits
// drawn from crypto/rand. Leading zeros are allowed, matching the
// Python server's key format.
//
// For small n (≤18), we read a single 8-byte (64-bit) value from
// crypto/rand and extract digits via modular arithmetic. This avoids
// n separate syscalls to the OS CSPRNG (e.g. /dev/urandom), reducing
// the cost from ~9×rand.Int to ~1×rand.Read. The bias from modular
// reduction is negligible for n ≤ 18 (10^18 < 2^60).
func randomDigits(n int) (string, error) {
	if n <= 0 {
		return "", nil
	}
	if n > 18 {
		// Fallback for large n: use big.Int per digit.
		return randomDigitsBig(n)
	}

	// Read 8 bytes (64 bits) in one syscall.
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", fmt.Errorf("reading random bytes: %w", err)
	}

	// Convert to uint64. crypto/rand guarantees uniform distribution,
	// so modular reduction gives (nearly) uniform digits.
	val := binary.BigEndian.Uint64(buf[:])
	modulus := uint64(1)
	for range n {
		modulus *= 10
	}
	val %= modulus

	// Format as zero-padded string.
	var sb strings.Builder
	sb.Grow(n)
	for range n {
		sb.WriteByte(byte('0' + val%10))
		val /= 10
	}
	// Reverse: we extracted digits LSB-first.
	b := []byte(sb.String())
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}
	return string(b), nil
}

// randomDigitsBig is the fallback for n > 18 digits, using big.Int
// per digit. Only called for unreasonably large key sizes.
func randomDigitsBig(n int) (string, error) {
	digits := make([]byte, n)
	maxDigit := big.NewInt(10)
	for i := range n {
		num, err := rand.Int(rand.Reader, maxDigit)
		if err != nil {
			return "", fmt.Errorf("generating random digit: %w", err)
		}
		digits[i] = '0' + byte(num.Int64())
	}
	return string(digits), nil
}

// channelKeyTaken reports whether any existing channel uses key as its
// name or its password, which would make key an unsafe channel key.
func channelKeyTaken(key string) bool {
	mu.RLock()
	defer mu.RUnlock()
	if channels == nil {
		return false
	}
	for name, c := range channels {
		if name == key || c.password == key {
			return true
		}
	}
	return false
}
