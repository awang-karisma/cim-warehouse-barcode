package accurate

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

// GenerateDeviceID generates a cryptographically random device ID matching Accurate client conventions.
func GenerateDeviceID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("A%d%s", time.Now().UnixMilli(), hex.EncodeToString(b))
}

// GenerateChallengeID generates a cryptographically random challenge ID for MFA challenges.
func GenerateChallengeID() string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

type preLoginPayload struct {
	V int    `json:"v"`
	P string `json:"p"`
	D string `json:"d"`
}

type postLoginPayload struct {
	V int     `json:"v"`
	P string  `json:"p"`
	C string  `json:"c"`
	T *string `json:"t"`
	D string  `json:"d"`
}

// NucauthPre encodes password for pre-login.do:
// "up" + base64(JSON {v: 1, p: password, d: deviceId})
func NucauthPre(password, deviceID string) (string, error) {
	payload := preLoginPayload{
		V: 1,
		P: password,
		D: deviceID,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return "up" + base64.StdEncoding.EncodeToString(b), nil
}

// NucauthPost encodes password and permit (and optional OTP) for auth.do:
// "ua" + base64(JSON {v: 1, p: password, c: permit, t: otpOrNull, d: deviceId})
func NucauthPost(password, permit string, otp *string, deviceID string) (string, error) {
	payload := postLoginPayload{
		V: 1,
		P: password,
		C: permit,
		T: otp,
		D: deviceID,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return "ua" + base64.StdEncoding.EncodeToString(b), nil
}
