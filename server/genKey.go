package server

import (
	"crypto/rand"
	"math/big"
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
// Python server's key format. range over an integer is a Go 1.22
// feature.
func randomDigits(n int) (string, error) {
	digits := make([]byte, n)
	max := big.NewInt(10)
	for i := range n {
		num, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		digits[i] = '0' + byte(num.Int64())
	}
	return string(digits), nil
}

// channelKeyTaken reports whether any existing channel uses key as its
// name or its password, which would make key an unsafe channel key.
func channelKeyTaken(key string) bool {
	sl.Lock()
	defer sl.Unlock()
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
