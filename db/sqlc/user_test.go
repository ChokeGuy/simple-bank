package sqlc

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/ChokeGuy/simple-bank/util"
	pw "github.com/ChokeGuy/simple-bank/util/password"
	"github.com/stretchr/testify/require"
)

func createRandomUser(t *testing.T) User {
	hashedPassword, err := pw.HashPassword(util.RandomString(6))
	require.NoError(t, err)

	arg := CreateUserParams{
		Username:       util.RandomOwner(),
		HashedPassword: hashedPassword,
		FullName:       util.RandomOwner(),
		Email:          util.RandomEmail(),
	}

	user, err := testQueries.CreateUser(context.Background(), arg)

	require.NoError(t, err)
	require.NotEmpty(t, user)

	require.Equal(t, arg.Username, user.Username)
	require.Equal(t, arg.FullName, user.FullName)
	require.Equal(t, arg.HashedPassword, user.HashedPassword)
	require.Equal(t, arg.Email, user.Email)

	require.True(t, user.PasswordChangedAt.IsZero())
	require.NotZero(t, user.CreatedAt)

	return user
}

func TestCreateUser(t *testing.T) {
	createRandomUser(t)
}

func TestGetUser(t *testing.T) {
	//create random user
	user1 := createRandomUser(t)
	user2, err := testQueries.GetUserByUserName(context.Background(), user1.Username)

	require.NoError(t, err)
	require.NotEmpty(t, user2)

	require.Equal(t, user1.Username, user2.Username)
	require.Equal(t, user1.FullName, user2.FullName)
	require.Equal(t, user1.HashedPassword, user2.HashedPassword)
	require.Equal(t, user1.Email, user2.Email)

	require.WithinDuration(t, user1.PasswordChangedAt, user2.PasswordChangedAt, time.Second)
	require.WithinDuration(t, user1.CreatedAt, user2.CreatedAt, time.Second)
}

func TestUpdateUserFullName(t *testing.T) {
	oldUser := createRandomUser(t)
	newFullName := util.RandomOwner()

	arg := UpdateUserParams{
		Username: oldUser.Username,
		FullName: sql.NullString{String: newFullName, Valid: true},
	}

	newUser, err := testQueries.UpdateUser(context.Background(), arg)

	require.NoError(t, err)
	require.NotEqual(t, oldUser.FullName, newUser.FullName)

	require.Equal(t, newFullName, newUser.FullName)
	require.Equal(t, oldUser.HashedPassword, newUser.HashedPassword)
	require.Equal(t, oldUser.Email, newUser.Email)
}

func TestUpdateUserEmail(t *testing.T) {
	oldUser := createRandomUser(t)
	newEmail := util.RandomEmail()

	arg := UpdateUserParams{
		Username: oldUser.Username,
		Email:    sql.NullString{String: newEmail, Valid: true},
	}

	newUser, err := testQueries.UpdateUser(context.Background(), arg)

	require.NoError(t, err)
	require.NotEqual(t, oldUser.Email, newUser.Email)

	require.Equal(t, newEmail, newUser.Email)
	require.Equal(t, oldUser.HashedPassword, newUser.HashedPassword)
	require.Equal(t, oldUser.FullName, newUser.FullName)
}

func TestUpdateUserHashedPassword(t *testing.T) {
	oldUser := createRandomUser(t)
	newPassword := util.RandomPassword()

	newHashedPassword, err := pw.HashPassword(newPassword)
	require.NoError(t, err)

	arg := UpdateUserParams{
		Username:       oldUser.Username,
		HashedPassword: sql.NullString{String: newHashedPassword, Valid: true},
	}

	newUser, err := testQueries.UpdateUser(context.Background(), arg)

	require.NoError(t, err)
	require.NotEqual(t, oldUser.HashedPassword, newUser.HashedPassword)

	require.Equal(t, newHashedPassword, newUser.HashedPassword)
	require.Equal(t, oldUser.Email, newUser.Email)
	require.Equal(t, oldUser.FullName, newUser.FullName)
}

func TestUpdateUserAllFields(t *testing.T) {
	oldUser := createRandomUser(t)

	newFullName := util.RandomOwner()
	newEmail := util.RandomEmail()
	newPassword := util.RandomPassword()

	newHashedPassword, err := pw.HashPassword(newPassword)
	require.NoError(t, err)

	arg := UpdateUserParams{
		Username:       oldUser.Username,
		HashedPassword: sql.NullString{String: newHashedPassword, Valid: true},
		FullName:       sql.NullString{String: newFullName, Valid: true},
		Email:          sql.NullString{String: newEmail, Valid: true},
	}

	newUser, err := testQueries.UpdateUser(context.Background(), arg)

	require.NoError(t, err)
	require.NotEqual(t, oldUser.HashedPassword, newUser.HashedPassword)
	require.NotEqual(t, oldUser.FullName, newUser.FullName)
	require.NotEqual(t, oldUser.Email, newUser.Email)

	require.Equal(t, newHashedPassword, newUser.HashedPassword)
	require.Equal(t, newFullName, newUser.FullName)
	require.Equal(t, newEmail, newUser.Email)
}
