package grpcapi

import (
	"context"

	gAccount "github.com/ChokeGuy/simple-bank/grpc-api/account"
	gUser "github.com/ChokeGuy/simple-bank/grpc-api/user"
	"github.com/ChokeGuy/simple-bank/pb"
	sv "github.com/ChokeGuy/simple-bank/server/grpc"
)

type ServiceHandler struct {
	pb.UnimplementedSimpleBankServer
	*gUser.UserHandler
	*gAccount.AccountHandler
}

func NewServiceHandler(server *sv.Server) *ServiceHandler {
	userHandler := gUser.NewUserHandler(server)
	accountHandler := gAccount.NewAccountHandler(server)

	return &ServiceHandler{UserHandler: userHandler, AccountHandler: accountHandler}
}

func (h *ServiceHandler) CreateUser(ctx context.Context, req *pb.CreateUserRequest) (*pb.CreateUserResponse, error) {
	return h.UserHandler.CreateUser(ctx, req)
}

func (h *ServiceHandler) LoginUser(ctx context.Context, req *pb.LoginUserRequest) (*pb.LoginUserResponse, error) {
	return h.UserHandler.LoginUser(ctx, req)
}

func (h *ServiceHandler) GetListAccount(ctx context.Context, req *pb.ListAccountRequest) (*pb.ListAccountResponse, error) {
	return h.AccountHandler.GetListAccount(ctx, req)
}

func (h *ServiceHandler) UpdateUser(ctx context.Context, req *pb.UpdateUserRequest) (*pb.UpdateUserResponse, error) {
	return h.UserHandler.UpdateUser(ctx, req)
}

func (h *ServiceHandler) VerifyUserEmail(ctx context.Context, req *pb.VerifyUserEmailRequest) (*pb.VerifyUserEmailResponse, error) {
	return h.UserHandler.VerifyUserEmail(ctx, req)
}
