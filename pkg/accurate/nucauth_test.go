package accurate_test

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/awang-karisma/cim-warehouse-barcode/pkg/accurate"
)

func TestNucauthPre(t *testing.T) {
	password := "SecretPass123!"
	deviceID := "A1234567890"

	encoded, err := accurate.NucauthPre(password, deviceID)
	if err != nil {
		t.Fatalf("NucauthPre failed: %v", err)
	}

	if !strings.HasPrefix(encoded, "up") {
		t.Fatalf("Expected 'up' prefix, got: %s", encoded)
	}

	b64Part := encoded[2:]
	rawJSON, err := base64.StdEncoding.DecodeString(b64Part)
	if err != nil {
		t.Fatalf("Failed to decode base64: %v", err)
	}

	var payload struct {
		V int    `json:"v"`
		P string `json:"p"`
		D string `json:"d"`
	}
	if err := json.Unmarshal(rawJSON, &payload); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	if payload.V != 1 || payload.P != password || payload.D != deviceID {
		t.Fatalf("Payload mismatch: %+v", payload)
	}
}

func TestNucauthPost(t *testing.T) {
	password := "SecretPass123!"
	permit := "PERMIT_TOKEN_XYZ"
	otp := "123456"
	deviceID := "A1234567890"

	// 1. With OTP
	encodedWithOTP, err := accurate.NucauthPost(password, permit, &otp, deviceID)
	if err != nil {
		t.Fatalf("NucauthPost failed: %v", err)
	}

	if !strings.HasPrefix(encodedWithOTP, "ua") {
		t.Fatalf("Expected 'ua' prefix, got: %s", encodedWithOTP)
	}

	rawWithOTP, _ := base64.StdEncoding.DecodeString(encodedWithOTP[2:])
	var pWithOTP struct {
		V int     `json:"v"`
		P string  `json:"p"`
		C string  `json:"c"`
		T *string `json:"t"`
		D string  `json:"d"`
	}
	if err := json.Unmarshal(rawWithOTP, &pWithOTP); err != nil {
		t.Fatalf("Failed to unmarshal JSON with OTP: %v", err)
	}
	if pWithOTP.V != 1 || pWithOTP.P != password || pWithOTP.C != permit || pWithOTP.T == nil || *pWithOTP.T != otp || pWithOTP.D != deviceID {
		t.Fatalf("Payload mismatch with OTP: %+v", pWithOTP)
	}

	// 2. Without OTP (direct login)
	encodedNoOTP, err := accurate.NucauthPost(password, permit, nil, deviceID)
	if err != nil {
		t.Fatalf("NucauthPost without OTP failed: %v", err)
	}

	rawNoOTP, _ := base64.StdEncoding.DecodeString(encodedNoOTP[2:])
	var pNoOTP struct {
		V int     `json:"v"`
		P string  `json:"p"`
		C string  `json:"c"`
		T *string `json:"t"`
		D string  `json:"d"`
	}
	if err := json.Unmarshal(rawNoOTP, &pNoOTP); err != nil {
		t.Fatalf("Failed to unmarshal JSON without OTP: %v", err)
	}
	if pNoOTP.T != nil {
		t.Fatalf("Expected null OTP in payload, got: %v", pNoOTP.T)
	}
}
