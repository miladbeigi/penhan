package crypto

import (
	"bytes"
	"crypto"
	"fmt"
	"os"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
)

type GPGProvider struct {
	entity *openpgp.Entity
}

func NewGPGProvider() *GPGProvider {
	return &GPGProvider{}
}

func (p *GPGProvider) Setup(keyPath, passphrase string) error {
	cfg := &packet.Config{DefaultHash: crypto.SHA256}

	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		entity, err := openpgp.NewEntity("penhan", "", "penhan@secret", cfg)
		if err != nil {
			return err
		}
		p.entity = entity

		f, err := os.Create(keyPath)
		if err != nil {
			return err
		}
		defer f.Close()

		return entity.SerializePrivate(f, nil)
	}

	f, err := os.Open(keyPath)
	if err != nil {
		return err
	}
	defer f.Close()

	entities, err := openpgp.ReadKeyRing(f)
	if err != nil {
		return err
	}

	if len(entities) == 0 {
		return fmt.Errorf("no keys found in key file")
	}

	p.entity = entities[0]
	return nil
}

func (p *GPGProvider) IsInitialized() bool {
	return p.entity != nil
}

func (p *GPGProvider) Encrypt(plaintext []byte) ([]byte, error) {
	var buf bytes.Buffer
	w, err := armor.Encode(&buf, "PGP MESSAGE", nil)
	if err != nil {
		return nil, err
	}

	encrypter, err := openpgp.Encrypt(w, []*openpgp.Entity{p.entity}, nil, nil, &packet.Config{DefaultHash: crypto.SHA256})
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
		return nil, err
	}

	entity, err := openpgp.ReadMessage(block.Body, openpgp.EntityList{p.entity}, nil, nil)
	if err != nil {
		return nil, err
	}

	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(entity.UnverifiedBody); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
