package email

import (
	"fmt"
	"net/smtp"

	"github.com/jordan-wright/email"
)

const (
	smtpAuthAddress   = "smtp.gmail.com"
	smtpServerAddress = "smtp.gmail.com:587"
)

// EmailPayload is a struct to send email
type EmailPayload struct {
	Subject     string
	Content     string
	To          []string
	CC          []string
	BCC         []string
	AttachFiles []string
}

// EmailSender is an interface to send email
type EmailSender interface {
	SendEmail(payload EmailPayload) error
}

// GmailSender is a struct to send email using Gmail
type GmailSender struct {
	name              string
	fromEmailAddress  string
	fromEmailPassword string
}

// NewGmailSender creates a new Gmail sender
func NewGmailSender(name string, fromEmailAddress string, fromEmailPassword string) EmailSender {
	return &GmailSender{
		name:              name,
		fromEmailAddress:  fromEmailAddress,
		fromEmailPassword: fromEmailPassword,
	}
}

// SendEmail sends an email using Gmail
func (gmail *GmailSender) SendEmail(payload EmailPayload) error {
	e := email.NewEmail()
	e.From = fmt.Sprintf("%s <%s>", gmail.name, gmail.fromEmailAddress)
	e.Subject = payload.Subject
	e.HTML = []byte(payload.Content)
	e.To = payload.To
	e.Cc = payload.CC
	e.Bcc = payload.BCC

	for _, file := range payload.AttachFiles {
		_, err := e.AttachFile(file)
		if err != nil {
			return fmt.Errorf("failed to attach file %s: %v", file, err)
		}
	}

	smtpAuth := smtp.PlainAuth("", gmail.fromEmailAddress, gmail.fromEmailPassword, smtpAuthAddress)

	err := e.Send(smtpServerAddress, smtpAuth)

	if err != nil {
		return fmt.Errorf("failed to send email: %v", err)
	}

	return nil
}
