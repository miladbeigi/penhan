package crypto

import (
	"bytes"
	"crypto"
	"fmt"
	"io"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
)

var gpgConfig = &packet.Config{DefaultHash: crypto.SHA256}

// GPGProvider encrypts to an OpenPGP keypair generated for the safe and
// writes ASCII-armored messages.
type GPGProvider struct {
	entity *openpgp.Entity
}

func generateGPGKey() ([]byte, error) {
	entity, err := openpgp.NewEntity("penhan", "", "penhan@secret", gpgConfig)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := entity.SerializePrivate(&buf, nil); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func newGPG(key []byte) (*GPGProvider, error) {
	entities, err := openpgp.ReadKeyRing(bytes.NewReader(key))
	if err != nil {
		return nil, fmt.Errorf("parse gpg key: %w", err)
	}
	if len(entities) == 0 {
		return nil, fmt.Errorf("no keys found in gpg key file")
	}
	return &GPGProvider{entity: entities[0]}, nil
}

func (p *GPGProvider) Encrypt(plaintext []byte) ([]byte, error) {
	var buf bytes.Buffer
	w, err := armor.Encode(&buf, "PGP MESSAGE", nil)
	if err != nil {
		return nil, err
	}

	encrypter, err := openpgp.Encrypt(w, []*openpgp.Entity{p.entity}, nil, nil, gpgConfig)
	if err != nil {
		return nil, err
	}
	if _, err := encrypter.Write(plaintext); err != nil {
		return nil, err
	}
	if err := encrypter.Close(); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (p *GPGProvider) Decrypt(ciphertext []byte) ([]byte, error) {
	block, err := armor.Decode(bytes.NewReader(ciphertext))
	if err != nil {
		return nil, fmt.Errorf("gpg decrypt: %w", err)
	}

	msg, err := openpgp.ReadMessage(block.Body, openpgp.EntityList{p.entity}, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("gpg decrypt (wrong key?): %w", err)
	}

	return io.ReadAll(msg.UnverifiedBody)
}
