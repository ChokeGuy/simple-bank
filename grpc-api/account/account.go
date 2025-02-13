package account

import (
	"context"
	"net/http"

	db "github.com/ChokeGuy/simple-bank/db/sqlc"
	"github.com/ChokeGuy/simple-bank/pb"
	sv "github.com/ChokeGuy/simple-bank/server/grpc"
	"google.golang.org/grpc/status"
)

type AccountHandler struct {
	*sv.Server
}

func NewAccountHandler(server *sv.Server) *AccountHandler {
	return &AccountHandler{Server: server}
}

func (h *AccountHandler) GetListAccount(ctx context.Context, req *pb.ListAccountRequest) (*pb.ListAccountResponse, error) {
	//authPayload := ctx.MustGet(auth.AuthPayloadKey).(*token.Payload)

	arg := db.ListAccountsParams{
		Owner:  req.GetOwner(),
		Limit:  req.GetSize(),
		Offset: (req.GetPage() - 1) * req.GetSize(),
	}

	accounts, err := h.Store.ListAccounts(ctx, arg)

	if err != nil {
		return nil, status.Errorf(http.StatusInternalServerError, "Failed to retrieve accounts: %v", err)
	}

	response := &pb.ListAccountResponse{
		Accounts: convertListAccount(accounts),
		Length:   int32(len(accounts)),
	}

	return response, nil
}
