package accounts_test

import (
	"strings"
	"testing"

	"github.com/awang-karisma/cim-warehouse-barcode/pkg/accurate/accounts"
)

func TestSecretBoxBindsCiphertextToAccountAndField(t *testing.T) {
	box, err := accounts.NewSecretBox("000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f")
	if err != nil {
		t.Fatal(err)
	}
	first, err := box.Seal(12, "password", "known secret")
	if err != nil {
		t.Fatal(err)
	}
	second, err := box.Seal(12, "password", "known secret")
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("separate encryption operations must use distinct nonces")
	}
	if strings.Contains(first, "known secret") {
		t.Fatal("ciphertext contains plaintext")
	}
	plain, err := box.Open(12, "password", first)
	if err != nil || plain != "known secret" {
		t.Fatalf("Open() = %q, %v", plain, err)
	}
	for _, tc := range []struct {
		id    int64
		field string
	}{{13, "password"}, {12, "api_token"}} {
		if _, err := box.Open(tc.id, tc.field, first); err == nil {
			t.Fatalf("ciphertext authenticated under mismatched account/field: %+v", tc)
		}
	}
}
