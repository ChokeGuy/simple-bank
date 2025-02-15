package email

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAwsSendEmailWithGmail(t *testing.T) {
	if testing.Short() {
		t.Skip("skip test in short mode")
	}

	// Create a new Gmail sender
	sender, err := NewSesEmailSender()
	require.NoError(t, err)

	receivers := []string{"nguyenthang13a32020@gmail.com"}

	// Send an email
	err = sender.SendEmail(EmailPayload{
		Subject: "Simple Bank Account Verification",
		Content: `
			<h1>Welcome to Simple Bank</h1>
			<p>Hi, this is a verify account from Simple Bank.</p>
			<p>Thank you for joining us.</p>
		`,
		To:          receivers,
		CC:          receivers[:1],
		AttachFiles: []string{"../../README.md"},
	})
	require.NoError(t, err)
}
