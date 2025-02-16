package sqlc

import (
	"context"
	"testing"
	"time"

	"github.com/ChokeGuy/simple-bank/util"
	"github.com/stretchr/testify/require"
)

func CreateRandomTransfer(t *testing.T) Transfer {
	account1 := CreateRandomAccount(t)
	account2 := CreateRandomAccount(t)

	arg := CreateTransferParams{
		FromAccountID: account1.ID,
		ToAccountID:   account2.ID,
		Amount:        util.RandomMoney(),
	}

	transfer, err := testStore.CreateTransfer(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, transfer)

	require.Equal(t, arg.Amount, transfer.Amount)
	require.Equal(t, arg.FromAccountID, transfer.FromAccountID)
	require.Equal(t, arg.ToAccountID, transfer.ToAccountID)

	require.NotZero(t, transfer.ID)
	require.NotZero(t, transfer.CreatedAt)

	return transfer
}

func CreateTransfer(t *testing.T, transfer Transfer) Transfer {
	arg := CreateTransferParams{
		FromAccountID: transfer.FromAccountID,
		ToAccountID:   transfer.ToAccountID,
		Amount:        util.RandomMoney(),
	}

	transfer, err := testStore.CreateTransfer(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, transfer)

	require.Equal(t, arg.Amount, transfer.Amount)
	require.Equal(t, arg.FromAccountID, transfer.FromAccountID)
	require.Equal(t, arg.ToAccountID, transfer.ToAccountID)

	require.NotZero(t, transfer.ID)
	require.NotZero(t, transfer.CreatedAt)

	return transfer
}

func TestCreateTransfer(t *testing.T) {
	CreateRandomTransfer(t)
}

func TestGetTransfers(t *testing.T) {
	transfer := CreateRandomTransfer(t)

	for i := 0; i < 10; i++ {
		CreateTransfer(t, transfer)
	}

	arg := GetTransfersParams{
		FromAccountID: transfer.FromAccountID,
		ToAccountID:   transfer.ToAccountID,
	}

	transfers, err := testStore.GetTransfers(context.Background(), arg)
	require.NoError(t, err)
	require.Len(t, transfers, 11)

	for _, transfer := range transfers {
		require.NotEmpty(t, transfer)
		require.Equal(t, arg.FromAccountID, transfer.FromAccountID)
		require.Equal(t, arg.ToAccountID, transfer.ToAccountID)
	}
}

func TestGetTransfer(t *testing.T) {
	transfer1 := CreateRandomTransfer(t)

	transfer2, err := testStore.GetTransfer(context.Background(), transfer1.ID)

	require.NoError(t, err)
	require.NotEmpty(t, transfer2)

	require.Equal(t, transfer1.ID, transfer2.ID)
	require.Equal(t, transfer1.FromAccountID, transfer2.FromAccountID)
	require.Equal(t, transfer1.ToAccountID, transfer2.ToAccountID)
	require.Equal(t, transfer1.Amount, transfer2.Amount)
	require.WithinDuration(t, transfer1.CreatedAt, transfer2.CreatedAt, time.Second)
}

func TestGetTransfersByFromAccountId(t *testing.T) {
	transfer := CreateRandomTransfer(t)

	for i := 0; i < 10; i++ {
		CreateTransfer(t, transfer)
	}

	transfers, err := testStore.GetTransfersByFromAccountId(context.Background(), transfer.FromAccountID)

	require.NoError(t, err)
	require.Len(t, transfers, 11)

	for _, transfer := range transfers {
		require.NotEmpty(t, transfer)
		require.Equal(t, transfer.FromAccountID, transfer.FromAccountID)
	}
}

func TestGetTransfersByToAccountId(t *testing.T) {
	transfer := CreateRandomTransfer(t)

	for i := 0; i < 10; i++ {
		CreateTransfer(t, transfer)
	}

	transfers, err := testStore.GetTransfersByToAccountId(context.Background(), transfer.ToAccountID)

	require.NoError(t, err)
	require.Len(t, transfers, 11)

	for _, transfer := range transfers {
		require.NotEmpty(t, transfer)
		require.Equal(t, transfer.ToAccountID, transfer.ToAccountID)
	}
}
