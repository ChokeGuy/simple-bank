package password

import (
	"testing"

	"github.com/ChokeGuy/simple-bank/util"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func TestPassword(t *testing.T) {
	password := util.RandomPassword()

	hashedPassword1, err := HashPassword(password)

	require.NoError(t, err)

	err = CheckPassword(password, hashedPassword1)
	require.NoError(t, err)

	wrongPassword := util.RandomPassword()

	err = CheckPassword(wrongPassword, hashedPassword1)
	require.EqualError(t, err, bcrypt.ErrMismatchedHashAndPassword.Error())

	hashedPassword2, err := HashPassword(password)
	require.NoError(t, err)
	require.NotEmpty(t, hashedPassword2)

	require.NotEqual(t, hashedPassword1, hashedPassword2)
}
