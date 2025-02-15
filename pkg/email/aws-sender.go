package email

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"

	pkg "github.com/ChokeGuy/simple-bank/pkg/config"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/aws/aws-sdk-go-v2/service/ses/types"
)

// Constants for MIME headers
const (
	contentTypeHTML       = "text/html; charset=UTF-8"
	contentTransferQP     = "quoted-printable"
	contentTypeAttachment = "application/octet-stream"
	contentTransferBase64 = "base64"
	mimeVersionHeader     = "MIME-Version: 1.0"
	multipartMixedHeader  = "multipart/mixed; boundary=%s"
)

// SesEmailSender is an email sender that uses AWS SES
type SesEmailSender struct {
	sesClient        *ses.Client
	fromEmailAddress string
}

// NewSesEmailSender initializes a new SesEmailSender
func NewSesEmailSender() (EmailSender, error) {
	envCfg, err := pkg.LoadConfig("../../")
	if err != nil {
		return nil, fmt.Errorf("failed to load environment variables: %v", err)
	}

	cfg, err := config.LoadDefaultConfig(
		context.TODO(),
		config.WithRegion(envCfg.AWSRegion),
		config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(
				envCfg.AWSAcessKeyID,
				envCfg.AWSSecretKey,
				"")),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %v", err)
	}

	sesClient := ses.NewFromConfig(cfg)

	return &SesEmailSender{
		sesClient:        sesClient,
		fromEmailAddress: envCfg.EmailSenderAddress,
	}, nil
}

// SendEmail sends an email with optional attachments via AWS SES
func (sesSender *SesEmailSender) SendEmail(payload EmailPayload) error {
	var emailRaw bytes.Buffer
	writer := multipart.NewWriter(&emailRaw)

	// Email headers
	boundary := writer.Boundary()

	var headers bytes.Buffer
	headers.WriteString(fmt.Sprintf("From: %s\n", sesSender.fromEmailAddress))
	headers.WriteString(fmt.Sprintf("To: %s\n", strings.Join(payload.To, ",")))

	// Add CC and BCC if provided
	if len(payload.CC) > 0 {
		headers.WriteString(formatHeader("CC", payload.CC))
	}
	if len(payload.BCC) > 0 {
		headers.WriteString(formatHeader("BCC", payload.BCC))
	}

	headers.WriteString(fmt.Sprintf("Subject: %s\n", payload.Subject))
	headers.WriteString(fmt.Sprintf("%s\n", mimeVersionHeader))
	headers.WriteString(fmt.Sprintf("Content-Type: "+multipartMixedHeader+"\n\n", boundary))

	emailRaw.WriteString(headers.String())

	// Email content (HTML)
	if err := writeBody(writer, payload.Content); err != nil {
		return err
	}

	// Attach files if provided
	for _, file := range payload.AttachFiles {
		if err := attachFile(writer, file); err != nil {
			return fmt.Errorf("failed to attach file %s: %v", file, err)
		}
	}

	writer.Close()

	// Send email via SES
	_, err := sesSender.sesClient.SendRawEmail(
		context.TODO(),
		&ses.SendRawEmailInput{
			Source: aws.String(sesSender.fromEmailAddress),
			Destinations: append(payload.To,
				append(payload.CC, payload.BCC...)...),
			RawMessage: &types.RawMessage{Data: emailRaw.Bytes()},
		})

	if err != nil {
		return fmt.Errorf("failed to send email via SES: %v", err)
	}

	return nil
}

func formatHeader(headerName string, addresses []string) string {
	if len(addresses) == 0 {
		return ""
	}
	return fmt.Sprintf("%s: %s\n", headerName, strings.Join(addresses, ","))
}

// writeBody adds the email body (HTML) to the multipart message
func writeBody(writer *multipart.Writer, body string) error {
	part, err := writer.CreatePart(
		textproto.MIMEHeader{
			"Content-Type":              {contentTypeHTML},
			"Content-Transfer-Encoding": {contentTransferQP},
		})

	if err != nil {
		return fmt.Errorf("failed to create body part: %v", err)
	}

	qp := quotedprintable.NewWriter(part)
	qp.Write([]byte(body))
	qp.Close()

	return nil
}

// attachFile adds a file attachment to the multipart email
func attachFile(writer *multipart.Writer, filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %v", err)
	}
	defer file.Close()

	fileName := filepath.Base(filePath)
	mimeType := mime.TypeByExtension(filepath.Ext(fileName))
	if mimeType == "" {
		mimeType = contentTypeAttachment
	}

	partHeader := make(textproto.MIMEHeader)
	partHeader.Set("Content-Type", fmt.Sprintf("%s; name=%q", mimeType, fileName))
	partHeader.Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", fileName))
	partHeader.Set("Content-Transfer-Encoding", contentTransferBase64)

	part, err := writer.CreatePart(partHeader)

	if err != nil {
		return fmt.Errorf("failed to create attachment part: %v", err)
	}

	fileBytes := make([]byte, 4*1024)
	encoder := base64.NewEncoder(base64.StdEncoding, part)
	for {
		n, err := file.Read(fileBytes)
		if n > 0 {
			encoder.Write(fileBytes[:n])
		}
		if err != nil {
			break
		}
	}
	encoder.Close()

	return nil
}
