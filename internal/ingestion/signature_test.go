package ingestion

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"testing"
)

func computeHMAC(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func TestVerifySignatureGitHubFormat(t *testing.T) {
	os.Setenv("TEST_SECRET", "mysecret")
	defer os.Unsetenv("TEST_SECRET")

	body := []byte(`{"action":"opened"}`)
	sig := "sha256=" + computeHMAC("mysecret", body)

	if err := VerifySignature(sig, "TEST_SECRET", body); err != nil {
		t.Fatalf("VerifySignature: %v", err)
	}
}

func TestVerifySignatureStripeFormat(t *testing.T) {
	os.Setenv("TEST_SECRET", "whsec_test")
	defer os.Unsetenv("TEST_SECRET")

	body := []byte(`{"type":"charge.succeeded"}`)
	sig := "t=1234567890,v1=" + computeHMAC("whsec_test", body)

	if err := VerifySignature(sig, "TEST_SECRET", body); err != nil {
		t.Fatalf("VerifySignature: %v", err)
	}
}

func TestVerifySignaturePagerDutyFormat(t *testing.T) {
	os.Setenv("TEST_SECRET", "pdsecret")
	defer os.Unsetenv("TEST_SECRET")

	body := []byte(`{"event":{"event_type":"incident.triggered"}}`)
	sig := "v1=" + computeHMAC("pdsecret", body)

	if err := VerifySignature(sig, "TEST_SECRET", body); err != nil {
		t.Fatalf("VerifySignature: %v", err)
	}
}

func TestVerifySignatureInvalid(t *testing.T) {
	os.Setenv("TEST_SECRET", "mysecret")
	defer os.Unsetenv("TEST_SECRET")

	body := []byte(`{"action":"opened"}`)
	if err := VerifySignature("sha256=0000000000000000000000000000000000000000000000000000000000000000", "TEST_SECRET", body); err == nil {
		t.Fatal("expected error for invalid signature")
	}
}

func TestVerifySignatureMissingSecret(t *testing.T) {
	os.Unsetenv("NONEXISTENT_SECRET")
	if err := VerifySignature("sha256=abc", "NONEXISTENT_SECRET", []byte("body")); err == nil {
		t.Fatal("expected error for missing secret")
	}
}

func TestVerifySignatureMissingHeader(t *testing.T) {
	os.Setenv("TEST_SECRET", "mysecret")
	defer os.Unsetenv("TEST_SECRET")
	if err := VerifySignature("", "TEST_SECRET", []byte("body")); err == nil {
		t.Fatal("expected error for missing header")
	}
}
