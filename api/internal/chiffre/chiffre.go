// Package chiffre protège les secrets des comptes (clés Stripe) en base, avec AES-256-GCM.
package chiffre

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
)

type Cle struct{ aead cipher.AEAD }

// Depuis lit une clé de 32 octets écrite en hexadécimal (64 caractères, ex. : openssl rand -hex 32).
func Depuis(hexa string) (*Cle, error) {
	b, err := hex.DecodeString(hexa)
	if err != nil || len(b) != 32 {
		return nil, errors.New("la clé de chiffrement doit faire 64 caractères hexadécimaux (openssl rand -hex 32)")
	}
	bloc, err := aes.NewCipher(b)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(bloc)
	if err != nil {
		return nil, err
	}
	return &Cle{aead: aead}, nil
}

// Chiffrer renvoie le nonce suivi du texte chiffré. contexte lie le résultat à son propriétaire :
// un secret copié sur un autre compte ne se déchiffre pas.
func (c *Cle) Chiffrer(clair []byte, contexte string) ([]byte, error) {
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return c.aead.Seal(nonce, nonce, clair, []byte(contexte)), nil
}

func (c *Cle) Dechiffrer(chiffre []byte, contexte string) ([]byte, error) {
	n := c.aead.NonceSize()
	if len(chiffre) < n {
		return nil, errors.New("donnée chiffrée trop courte")
	}
	clair, err := c.aead.Open(nil, chiffre[:n], chiffre[n:], []byte(contexte))
	if err != nil {
		return nil, fmt.Errorf("déchiffrement impossible (clé de chiffrement changée ?) : %w", err)
	}
	return clair, nil
}
