package token

import "time"

// Maker is an interface that defines the methods a token maker type must provide
type Maker interface {
	// CreateToken generates a new token for a specific username and duration
	CreateToken(username string, duration time.Duration) (string, *Payload, error)
	// VerifyToken checks if the token is valid or not
	VerifyToken(token string) (*Payload, error)
}
