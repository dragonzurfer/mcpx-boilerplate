package services

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"os"
	"strings"
	"time"
)

var ErrReceiptKeyMissing = errors.New("receipt signing key missing")
var ErrReceiptPublicKeyMissing = errors.New("receipt verification key missing")

const receiptKeyEnv = "JUDGE_RECEIPT_PRIVATE_KEY"
const receiptPublicKeyEnv = "JUDGE_RECEIPT_PUBLIC_KEY"

type Receipt struct {
	SubmissionID uint   `json:"submission_id"`
	ResultHash   string `json:"result_hash"`
	IssuedAt     string `json:"issued_at"`
	Signature    string `json:"signature"`
}

type receiptInput struct {
	SubmissionID uint
	ResultJSON   string
}

type receiptVerifyInput struct {
	Receipt    Receipt
	ResultJSON string
}

func BuildReceipt(input receiptInput) (Receipt, error) {
	privateKey, err := receiptPrivateKey()
	if err != nil {
		return Receipt{}, err
	}

	hash := sha256.Sum256([]byte(input.ResultJSON))
	signature := ed25519.Sign(privateKey, hash[:])

	return Receipt{
		SubmissionID: input.SubmissionID,
		ResultHash:   "sha256:" + hex.EncodeToString(hash[:]),
		IssuedAt:     time.Now().UTC().Format(time.RFC3339),
		Signature:    "ed25519:" + base64.StdEncoding.EncodeToString(signature),
	}, nil
}

func VerifyReceipt(input receiptVerifyInput) error {
	if input.Receipt.SubmissionID == 0 {
		return errors.New("receipt submission missing")
	}

	resultJSON := strings.TrimSpace(input.ResultJSON)
	if resultJSON == "" {
		return errors.New("result payload missing")
	}

	expectedHash := HashString(resultJSON)
	if strings.TrimSpace(input.Receipt.ResultHash) != expectedHash {
		return errors.New("receipt hash mismatch")
	}

	publicKey, err := receiptPublicKey()
	if err != nil {
		return err
	}

	signature, err := parseReceiptSignature(input.Receipt.Signature)
	if err != nil {
		return err
	}

	hash := sha256.Sum256([]byte(resultJSON))
	if !ed25519.Verify(publicKey, hash[:], signature) {
		return errors.New("receipt signature invalid")
	}

	return nil
}

func receiptPrivateKey() (ed25519.PrivateKey, error) {
	encoded := strings.TrimSpace(os.Getenv(receiptKeyEnv))
	if encoded == "" {
		return nil, ErrReceiptKeyMissing
	}

	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, err
	}

	if len(decoded) != ed25519.PrivateKeySize {
		return nil, errors.New("invalid receipt key length")
	}

	return ed25519.PrivateKey(decoded), nil
}

func receiptPublicKey() (ed25519.PublicKey, error) {
	encoded := strings.TrimSpace(os.Getenv(receiptPublicKeyEnv))
	if encoded == "" {
		return nil, ErrReceiptPublicKeyMissing
	}

	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, err
	}

	if len(decoded) != ed25519.PublicKeySize {
		return nil, errors.New("invalid receipt public key length")
	}

	return ed25519.PublicKey(decoded), nil
}

func parseReceiptSignature(value string) ([]byte, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, errors.New("receipt signature missing")
	}

	parts := strings.SplitN(trimmed, ":", 2)
	if len(parts) != 2 || parts[0] != "ed25519" {
		return nil, errors.New("invalid receipt signature")
	}

	decoded, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, err
	}

	if len(decoded) != ed25519.SignatureSize {
		return nil, errors.New("invalid receipt signature length")
	}

	return decoded, nil
}
