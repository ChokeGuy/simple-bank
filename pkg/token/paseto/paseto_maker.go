package paseto

import (
	"time"

	tk "github.com/ChokeGuy/simple-bank/pkg/token"
	"github.com/aead/chacha20poly1305"
	"github.com/o1egl/paseto"
)

// PasetoMaker is a PASETO Token maker
type PasetoMaker struct {
	paseto       *paseto.V2
	symmetricKey []byte
}

// NewPasetoMaker create a new PasetoMaker
func NewPasetoMaker(symmetricKey string) (tk.Maker, error) {
	if len(symmetricKey) != chacha20poly1305.KeySize {
		return nil, tk.ErrInvalidPasetoKeySize
	}

	maker := &PasetoMaker{
		paseto:       paseto.NewV2(),
		symmetricKey: []byte(symmetricKey),
	}
	return maker, nil
}

// CreateToken creates a new token for a specific username and duration.
func (maker *PasetoMaker) CreateToken(username string, role string, duration time.Duration) (string, *tk.Payload, error) {
	payload, err := tk.NewPayload(username, role, duration)

	if err != nil {
		return "", payload, err
	}

	token, err := maker.paseto.Encrypt(maker.symmetricKey, payload, nil)
	return token, payload, err
}

// VerifyToken check if the token is valid or not.
func (maker *PasetoMaker) VerifyToken(token string) (*tk.Payload, error) {
	payload := &tk.Payload{}

	err := maker.paseto.Decrypt(token, maker.symmetricKey, payload, nil)
	if err != nil {
		return nil, tk.ErrInvalidToken
	}

	err = payload.Valid()
	if err != nil {
		return nil, err
	}

	return payload, nil
}
