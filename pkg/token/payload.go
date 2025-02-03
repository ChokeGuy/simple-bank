package token

import (
	"errors"
	"fmt"
	"time"

	"github.com/aead/chacha20poly1305"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	MinSecretKeySize = 32
)

// ErrInvalidToken is returned when the token is invalid
var (
	ErrInvalidToken         = errors.New("invalid token")
	ErrInvalidJWTKeySize    = fmt.Errorf("invalid key size: must be at least %d characters", MinSecretKeySize)
	ErrInvalidPasetoKeySize = fmt.Errorf("invalid key size: must be at least %d characters", chacha20poly1305.KeySize)
)

// Payload is the payload data of the token
type Payload struct {
	UserName string `json:"userName"`
	jwt.RegisteredClaims
}

// NewPayload creates a new Payload instance
func NewPayload(username string, duration time.Duration) (*Payload, error) {
	tokenId, err := uuid.NewRandom()
	if err != nil {
		return nil, err
	}

	payload := &Payload{
		UserName: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(duration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ID:        tokenId.String(),
		},
	}
	return payload, nil
}

func (payload *Payload) Valid() error {
	if time.Now().After(payload.ExpiresAt.Time) {
		return jwt.ErrTokenExpired
	}
	return nil
}
