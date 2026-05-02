package usecase

import (
	"crypto/rand"
	"encoding/base64"
)

const secureTokenBytes = 32

func randomURLToken() (string, error) {
	buf := make([]byte, secureTokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
