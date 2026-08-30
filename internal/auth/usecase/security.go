package usecase

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
)

const secureTokenBytes = 32

func randomURLToken() (string, error) {
	buf := make([]byte, secureTokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// hashToken is what gets stored for a refresh token, so a database leak yields
// hashes rather than usable credentials. SHA-256 with no salt or stretching is
// correct here: the input is 256 bits of entropy from randomURLToken, so there is
// nothing to brute force and a slow KDF would only tax every refresh request.
func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// secureEqual compares callback secrets without leaking length or content timing.
func secureEqual(got, want string) bool {
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}
