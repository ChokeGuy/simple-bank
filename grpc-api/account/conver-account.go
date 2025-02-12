package account

import (
	db "github.com/ChokeGuy/simple-bank/db/sqlc"
	"github.com/ChokeGuy/simple-bank/pb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func convertListAccount(account []db.Account) []*pb.Account {
	var accounts []*pb.Account
	for _, v := range account {
		accounts = append(accounts, &pb.Account{
			Id:        v.ID,
			Owner:     v.Owner,
			Balance:   v.Balance,
			Currency:  v.Currency,
			CreatedAt: timestamppb.New(v.CreatedAt),
		})
	}

	return accounts
}
