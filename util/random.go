package util

import (
	"fmt"
	"math/rand"
	"strings"
	"time"
)

const alphabet = "abcdefghijklmnopqrstuvwxyz"

func init() {
	rand.Seed(time.Now().UnixNano())
}

func RandomInt(min, max int64) int64 {
	return min + rand.Int63n(max-min+1)
}

func RandomString(n int) string {
	var sb strings.Builder

	k := len(alphabet)

	for i := 0; i < n; i++ {
		c := alphabet[rand.Intn(k)]
		sb.WriteByte(c)
	}

	return sb.String()
}

func RandomPassword() string {
	const (
		upperLetters = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
		lowerLetters = "abcdefghijklmnopqrstuvwxyz"
		numbers      = "0123456789"
		specialChars = "!@#~$%^&*()_+|<>?:{}"
		allChars     = upperLetters + lowerLetters + numbers + specialChars
	)

	rand.Seed(time.Now().UnixNano())

	password := make([]byte, 6)
	password[0] = upperLetters[rand.Intn(len(upperLetters))]
	password[1] = lowerLetters[rand.Intn(len(lowerLetters))]
	password[2] = numbers[rand.Intn(len(numbers))]
	password[3] = specialChars[rand.Intn(len(specialChars))]
	for i := 4; i < 6; i++ {
		password[i] = allChars[rand.Intn(len(allChars))]
	}

	// Shuffle the password to ensure randomness
	rand.Shuffle(len(password), func(i, j int) {
		password[i], password[j] = password[j], password[i]
	})

	return string(password)
}

func RandomMoney() int64 {
	return RandomInt(0, 1000)
}

func RandomCurrency() string {
	currencies := []string{USD, EUR, CAD, VND}
	n := len(currencies)
	return currencies[rand.Intn(n)]
}

func RandomOwner() string {
	return RandomString(6)
}

func RandomEmail() string {
	return fmt.Sprintf("%s@gmail.com", RandomString(6))
}
