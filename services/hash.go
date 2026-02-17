package services

import (
	"crypto/sha256"
	"encoding/hex"
)

func HashString(value string) string {
	hash := sha256.Sum256([]byte(value))
	return "sha256:" + hex.EncodeToString(hash[:])
}
