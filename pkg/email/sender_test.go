package email

import (
	"testing"

	pkg "github.com/ChokeGuy/simple-bank/pkg/config"
	"github.com/stretchr/testify/require"
)

func TestSendEmailWithGmail(t *testing.T) {
	if testing.Short() {
		t.Skip("skip test in short mode")
	}

	cfg, err := pkg.LoadConfig("../../")
	require.NoError(t, err)

	// Create a new Gmail sender
	sender := NewGmailSender(
		cfg.EmailSenderName,
		cfg.EmailSenderAddress,
		cfg.EmailSenderPassword,
	)

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
		AttachFiles: []string{"../../README.md"},
	})
	require.NoError(t, err)
}
