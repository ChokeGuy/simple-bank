package jwt

import (
	"strings"
	"time"

	tk "github.com/ChokeGuy/simple-bank/pkg/token"
	"github.com/golang-jwt/jwt/v5"
)

// JWTMaker is a JSON Web Token maker
type JWTMaker struct {
	secretKey string
}

// CreateToken implements Maker.
func (maker *JWTMaker) CreateToken(username string, duration time.Duration) (string, error) {
	payload, err := tk.NewPayload(username, duration)

	if err != nil {
		return "", err
	}

	jwtToken := jwt.NewWithClaims(jwt.SigningMethodHS256, payload)

	return jwtToken.SignedString([]byte(maker.secretKey))
}

// VerifyToken implements Maker.
func (maker *JWTMaker) VerifyToken(token string) (*tk.Payload, error) {
	keyFunc := func(token *jwt.Token) (interface{}, error) {
		_, ok := token.Method.(*jwt.SigningMethodHMAC)
		if !ok {
			return nil, tk.ErrInvalidToken
		}

		return []byte(maker.secretKey), nil
	}

	jwtToken, err := jwt.ParseWithClaims(token, &tk.Payload{}, keyFunc)
	if err != nil {
		if strings.Contains(err.Error(), jwt.ErrTokenExpired.Error()) {
			return nil, jwt.ErrTokenExpired
		}
		return nil, tk.ErrInvalidToken
	}

	payload, ok := jwtToken.Claims.(*tk.Payload)
	if !ok {
		return nil, tk.ErrInvalidToken
	}

	return payload, nil
}

func NewJWTMaker(secretKey string) (tk.Maker, error) {
	if len(secretKey) < tk.MinSecretKeySize {
		return nil, tk.ErrInvalidJWTKeySize
	}

	return &JWTMaker{secretKey}, nil
}
