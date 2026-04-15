// payment_signature_test.go — branch-complete unit tests for
// PaymentService.verifySignature.
//
// verifySignature is the cryptographic gate for every payment callback
// in the system. It must:
//   - reject when the merchant key is empty,
//   - accept the canonical message format with a correct HMAC-SHA256,
//   - reject any tampering with gateway tx, ref, or amount,
//   - reject malformed signatures and empty signatures.
package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"
)

// newPaymentServiceForSigning constructs a PaymentService with only the
// merchantKey populated. verifySignature reads no other fields, so this
// is a sound minimal fixture and avoids any DB dependency.
func newPaymentServiceForSigning(key string) *PaymentService {
	return &PaymentService{merchantKey: []byte(key)}
}

// hmacSign is the reference implementation that the production code must
// agree with. Mirroring the format here lets these tests act as a
// regression fence on any future refactor of the signature payload.
func hmacSign(key, gatewayTxID, merchantRef string, amount int64) string {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(fmt.Sprintf("%s|%s|%d", gatewayTxID, merchantRef, amount)))
	return hex.EncodeToString(mac.Sum(nil))
}

const testKey = "unit-test-merchant-key-256-bits-of-entropy-please"

func TestVerifySignature_EmptyKey_AlwaysRejects(t *testing.T) {
	svc := newPaymentServiceForSigning("")
	sig := hmacSign(testKey, "tx", "ref", 100) // any signature
	if svc.verifySignature("tx", "ref", 100, sig) {
		t.Fatal("must reject all signatures when merchant key is empty")
	}
}

func TestVerifySignature_HappyPath(t *testing.T) {
	svc := newPaymentServiceForSigning(testKey)
	sig := hmacSign(testKey, "tx-1", "ref-A", 12345)
	if !svc.verifySignature("tx-1", "ref-A", 12345, sig) {
		t.Fatal("correct signature must verify true")
	}
}

func TestVerifySignature_TamperedGatewayTxID(t *testing.T) {
	svc := newPaymentServiceForSigning(testKey)
	sig := hmacSign(testKey, "tx-1", "ref-A", 12345)
	if svc.verifySignature("tx-2", "ref-A", 12345, sig) {
		t.Fatal("must reject when gateway tx ID is tampered")
	}
}

func TestVerifySignature_TamperedMerchantRef(t *testing.T) {
	svc := newPaymentServiceForSigning(testKey)
	sig := hmacSign(testKey, "tx-1", "ref-A", 12345)
	if svc.verifySignature("tx-1", "ref-B", 12345, sig) {
		t.Fatal("must reject when merchant ref is tampered")
	}
}

func TestVerifySignature_TamperedAmount(t *testing.T) {
	svc := newPaymentServiceForSigning(testKey)
	sig := hmacSign(testKey, "tx-1", "ref-A", 12345)
	if svc.verifySignature("tx-1", "ref-A", 12346, sig) {
		t.Fatal("must reject when amount is tampered (1-fen change)")
	}
}

func TestVerifySignature_WrongKey(t *testing.T) {
	svc := newPaymentServiceForSigning(testKey)
	sigFromOtherKey := hmacSign("other-key", "tx-1", "ref-A", 12345)
	if svc.verifySignature("tx-1", "ref-A", 12345, sigFromOtherKey) {
		t.Fatal("must reject signature computed with a different key")
	}
}

func TestVerifySignature_EmptySignature(t *testing.T) {
	svc := newPaymentServiceForSigning(testKey)
	if svc.verifySignature("tx-1", "ref-A", 12345, "") {
		t.Fatal("must reject empty signature")
	}
}

func TestVerifySignature_NonHexSignature(t *testing.T) {
	svc := newPaymentServiceForSigning(testKey)
	if svc.verifySignature("tx-1", "ref-A", 12345, "not-a-hex-string!!!") {
		t.Fatal("must reject non-hex signature")
	}
}

func TestVerifySignature_HexButWrongLength(t *testing.T) {
	svc := newPaymentServiceForSigning(testKey)
	// Truncated to 32 hex chars (16 bytes) — HMAC-SHA256 produces 64 hex chars.
	short := hmacSign(testKey, "tx-1", "ref-A", 12345)[:32]
	if svc.verifySignature("tx-1", "ref-A", 12345, short) {
		t.Fatal("must reject signature of wrong length")
	}
}

// Bit-flip resistance: changing a single hex nibble must invalidate.
func TestVerifySignature_OneNibbleFlip(t *testing.T) {
	svc := newPaymentServiceForSigning(testKey)
	good := hmacSign(testKey, "tx-1", "ref-A", 12345)
	// Flip the last nibble.
	last := good[len(good)-1]
	var flipped byte
	if last == '0' {
		flipped = '1'
	} else {
		flipped = '0'
	}
	tampered := good[:len(good)-1] + string(flipped)
	if svc.verifySignature("tx-1", "ref-A", 12345, tampered) {
		t.Fatal("must reject when even a single nibble is flipped")
	}
}

// Negative amount edge case (refunds are not callbacks but the function
// must not panic on negative values).
func TestVerifySignature_NegativeAmount(t *testing.T) {
	svc := newPaymentServiceForSigning(testKey)
	sig := hmacSign(testKey, "tx-1", "ref-A", -1)
	if !svc.verifySignature("tx-1", "ref-A", -1, sig) {
		t.Fatal("must accept legitimate signature even for negative amount")
	}
	// And reject when sign-flipped.
	if svc.verifySignature("tx-1", "ref-A", 1, sig) {
		t.Fatal("must reject when sign of amount is flipped")
	}
}
