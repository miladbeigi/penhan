package crypto

import (
	"bytes"
	"testing"
)

func TestWrapKeyRoundtrip(t *testing.T) {
	master, _ := GenerateMasterKey()
	key := []byte("safe key bytes")
	wrapped, err := WrapKey(master, key)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(wrapped, key) {
		t.Fatal("wrapped key contains the plaintext key")
	}
	got, err := UnwrapKey(master, wrapped)
	if err != nil || !bytes.Equal(got, key) {
		t.Fatalf("UnwrapKey = %q, %v", got, err)
	}

	other, _ := GenerateMasterKey()
	if _, err := UnwrapKey(other, wrapped); err == nil {
		t.Error("unwrapping with another master key must fail")
	}
}
