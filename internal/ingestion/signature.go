package ingestion

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
)

// VerifySignature checks an HMAC-SHA256 webhook signature.
// Supports formats: raw hex, "sha256=hex", "v1=hex" (PagerDuty), "t=...,v1=hex" (Stripe).
func VerifySignature(header, secretEnvVar string, body []byte) error {
	secret := os.Getenv(secretEnvVar)
	if secret == "" {
		return fmt.Errorf("signature secret env %q not set", secretEnvVar)
	}

	if header == "" {
		return fmt.Errorf("missing signature header")
	}

	// Extract the hex signature from various formats
	sig := extractSignature(header)
	if sig == "" {
		return fmt.Errorf("could not parse signature from header")
	}

	expected, err := hex.DecodeString(sig)
	if err != nil {
		return fmt.Errorf("invalid signature hex: %w", err)
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	computed := mac.Sum(nil)

	if !hmac.Equal(computed, expected) {
		return fmt.Errorf("signature mismatch")
	}
	return nil
}

// extractSignature parses the hex signature from common webhook formats.
func extractSignature(header string) string {
	// "sha256=abc123" (GitHub, Shopify)
	if strings.HasPrefix(header, "sha256=") {
		return strings.TrimPrefix(header, "sha256=")
	}
	// "v1=abc123" (PagerDuty)
	if strings.HasPrefix(header, "v1=") {
		return strings.TrimPrefix(header, "v1=")
	}
	// "t=123,v1=abc123" (Stripe) — extract v1 part
	if strings.Contains(header, ",v1=") {
		for _, part := range strings.Split(header, ",") {
			part = strings.TrimSpace(part)
			if strings.HasPrefix(part, "v1=") {
				return strings.TrimPrefix(part, "v1=")
			}
		}
	}
	// Raw hex
	if _, err := hex.DecodeString(header); err == nil && len(header) >= 32 {
		return header
	}
	return ""
}
