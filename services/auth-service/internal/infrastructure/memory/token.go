package memory

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
)

func domainInternalOpaqueToken(bytesLen int) (string, error) {
	if bytesLen <= 0 {
		return "", errors.New("invalid token length")
	}
	b := make([]byte, bytesLen)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
