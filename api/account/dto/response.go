package account

import db "github.com/ChokeGuy/simple-bank/db/sqlc"

type ListAccountResponse struct {
	Accounts []db.Account `json:"accounts"`
	Length   int          `json:"length"`
}
