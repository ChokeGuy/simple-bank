package user

import (
	"fmt"
	"reflect"

	db "github.com/ChokeGuy/simple-bank/db/sqlc"
	"github.com/ChokeGuy/simple-bank/util/password"
	"github.com/ChokeGuy/simple-bank/worker"
	"github.com/golang/mock/gomock"
)

type eqCreateUserTxParamsMatcher struct {
	arg      db.CreateUserTxParams
	password string
	user     db.User
}

func (e eqCreateUserTxParamsMatcher) Matches(x interface{}) bool {
	actualArg, ok := x.(db.CreateUserTxParams)
	if !ok {
		return false
	}

	err := password.CheckPassword(e.password, actualArg.HashedPassword)
	if err != nil {
		return false
	}

	e.arg.HashedPassword = actualArg.HashedPassword

	if !reflect.DeepEqual(e.arg.CreateUserParams, actualArg.CreateUserParams) {
		return false
	}

	actualArg.AfterCreate(e.user)
	return true
}

func (e eqCreateUserTxParamsMatcher) String() string {
	return fmt.Sprintf("matches arg %v and password %v", e.arg, e.password)
}

func EqCreateUserTxParams(arg db.CreateUserTxParams, password string, user db.User) gomock.Matcher {
	return eqCreateUserTxParamsMatcher{arg, password, user}
}

// Custom matcher for PayloadSendVerifyEmail
type eqPayloadSendVerifyEmailMatcher struct {
	expected *worker.PayloadSendVerifyEmail
}

func (e eqPayloadSendVerifyEmailMatcher) Matches(x interface{}) bool {
	actual, ok := x.(*worker.PayloadSendVerifyEmail)
	if !ok {
		return false
	}
	return actual.UserName == e.expected.UserName
}

func (e eqPayloadSendVerifyEmailMatcher) String() string {
	return "matches payload send verify email"
}

func EqPayloadSendVerifyEmail(expected *worker.PayloadSendVerifyEmail) gomock.Matcher {
	return eqPayloadSendVerifyEmailMatcher{expected}
}
