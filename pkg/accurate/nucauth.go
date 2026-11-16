package accurate

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"
	"unicode/utf8"
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
// "up" + btoa(JSON.stringify({v: 1, p: password, d: deviceId})).
func NucauthPre(password, deviceID string) (string, error) {
	payload := preLoginPayload{
		V: 1,
		P: password,
		D: deviceID,
	}
	return encodeNucauth("up", payload, password, deviceID)
}

// NucauthPost encodes password and permit (and optional OTP) for auth.do:
// "ua" + btoa(JSON.stringify({v: 1, p: password, c: permit, t: otpOrNull, d: deviceId})).
func NucauthPost(password, permit string, otp *string, deviceID string) (string, error) {
	payload := postLoginPayload{
		V: 1,
		P: password,
		C: permit,
		T: otp,
		D: deviceID,
	}
	if otp != nil {
		return encodeNucauth("ua", payload, password, permit, *otp, deviceID)
	}
	return encodeNucauth("ua", payload, password, permit, deviceID)
}

// Browser btoa consumes one byte per Latin-1 code point, not UTF-8. Reject
// unsupported input instead of silently sending different credential bytes.
func encodeNucauth(prefix string, payload any, values ...string) (string, error) {
	for _, value := range values {
		if !utf8.ValidString(value) {
			return "", errors.New("Accurate browser credential encoding requires valid Latin-1 text")
		}
		for _, r := range value {
			if r > 255 {
				return "", errors.New("Accurate browser credential encoding requires valid Latin-1 text")
			}
		}
	}
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(payload); err != nil {
		return "", err
	}
	// Encode appends a newline that JSON.stringify does not. The payload is
	// already validated, so compact its UTF-8 representation to Latin-1 in place.
	b := buf.Bytes()
	n := 0
	for i := 0; i < len(b)-1; {
		r, size := utf8.DecodeRune(b[i:])
		b[n] = byte(r)
		n++
		i += size
	}
	return prefix + base64.StdEncoding.EncodeToString(b[:n]), nil
}
