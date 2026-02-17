package services

import (
	"crypto/rand"
	"encoding/hex"
)

func GenerateAnalysisID() string {
	buffer := make([]byte, 6)
	_, err := rand.Read(buffer)
	if err != nil {
		return "ana_000000"
	}

	return "ana_" + hex.EncodeToString(buffer)
}
