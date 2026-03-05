package utils

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net/smtp"
	"strings"
)

type SMTPConfig struct {
	Server        string
	Port          int
	UseTLS        bool
	Username      string
	Password      string
	DefaultSender string
}

type EmailAttachment struct {
	FileName    string
	ContentType string
	Data        []byte
}

func SendEmail(cfg SMTPConfig, to []string, subject, textBody, htmlBody string, attachments []EmailAttachment) error {
	recipients := make([]string, 0, len(to))
	for _, addr := range to {
		value := strings.TrimSpace(addr)
		if value != "" {
			recipients = append(recipients, value)
		}
	}
	if len(recipients) == 0 {
		return fmt.Errorf("empty recipients")
	}

	from := strings.TrimSpace(cfg.DefaultSender)
	if from == "" {
		from = strings.TrimSpace(cfg.Username)
	}
	if from == "" {
		return fmt.Errorf("missing sender")
	}
	if strings.TrimSpace(cfg.Server) == "" || cfg.Port <= 0 {
		return fmt.Errorf("invalid smtp server config")
	}

	message, err := buildMIMEMessage(from, recipients, subject, textBody, htmlBody, attachments)
	if err != nil {
		return err
	}

	auth := smtp.Auth(nil)
	if strings.TrimSpace(cfg.Username) != "" && strings.TrimSpace(cfg.Password) != "" {
		auth = smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Server)
	}
	addr := fmt.Sprintf("%s:%d", cfg.Server, cfg.Port)

	if !cfg.UseTLS {
		if auth == nil {
			return smtp.SendMail(addr, nil, from, recipients, message)
		}
		return smtp.SendMail(addr, auth, from, recipients, message)
	}

	conn, err := tls.Dial("tcp", addr, &tls.Config{
		ServerName: cfg.Server,
		MinVersion: tls.VersionTLS12,
	})
	if err != nil {
		return err
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, cfg.Server)
	if err != nil {
		return err
	}
	defer client.Quit()

	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return err
		}
	}
	if err := client.Mail(from); err != nil {
		return err
	}
	for _, recipient := range recipients {
		if err := client.Rcpt(recipient); err != nil {
			return err
		}
	}

	writer, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := writer.Write(message); err != nil {
		_ = writer.Close()
		return err
	}
	return writer.Close()
}

func buildMIMEMessage(from string, to []string, subject, textBody, htmlBody string, attachments []EmailAttachment) ([]byte, error) {
	mixedBoundary := "mixedBoundaryDutyLog"
	altBoundary := "altBoundaryDutyLog"

	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("From: %s\r\n", from))
	buf.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(to, ", ")))
	buf.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	buf.WriteString("MIME-Version: 1.0\r\n")

	hasAttachments := len(attachments) > 0
	hasHTML := strings.TrimSpace(htmlBody) != ""
	if hasAttachments || hasHTML {
		buf.WriteString(fmt.Sprintf("Content-Type: multipart/mixed; boundary=%q\r\n\r\n", mixedBoundary))

		buf.WriteString(fmt.Sprintf("--%s\r\n", mixedBoundary))
		buf.WriteString(fmt.Sprintf("Content-Type: multipart/alternative; boundary=%q\r\n\r\n", altBoundary))

		buf.WriteString(fmt.Sprintf("--%s\r\n", altBoundary))
		buf.WriteString("Content-Type: text/plain; charset=UTF-8\r\n\r\n")
		buf.WriteString(textBody + "\r\n")

		if hasHTML {
			buf.WriteString(fmt.Sprintf("--%s\r\n", altBoundary))
			buf.WriteString("Content-Type: text/html; charset=UTF-8\r\n\r\n")
			buf.WriteString(htmlBody + "\r\n")
		}
		buf.WriteString(fmt.Sprintf("--%s--\r\n", altBoundary))

		for _, attachment := range attachments {
			if len(attachment.Data) == 0 || strings.TrimSpace(attachment.FileName) == "" {
				continue
			}
			contentType := strings.TrimSpace(attachment.ContentType)
			if contentType == "" {
				contentType = "application/octet-stream"
			}
			buf.WriteString(fmt.Sprintf("--%s\r\n", mixedBoundary))
			buf.WriteString(fmt.Sprintf("Content-Type: %s; name=%q\r\n", contentType, attachment.FileName))
			buf.WriteString("Content-Transfer-Encoding: base64\r\n")
			buf.WriteString(fmt.Sprintf("Content-Disposition: attachment; filename=%q\r\n\r\n", attachment.FileName))

			encoded := make([]byte, base64.StdEncoding.EncodedLen(len(attachment.Data)))
			base64.StdEncoding.Encode(encoded, attachment.Data)
			writeBase64WithWrap(&buf, encoded, 76)
			buf.WriteString("\r\n")
		}
		buf.WriteString(fmt.Sprintf("--%s--\r\n", mixedBoundary))
		return buf.Bytes(), nil
	}

	buf.WriteString("Content-Type: text/plain; charset=UTF-8\r\n\r\n")
	buf.WriteString(textBody + "\r\n")
	return buf.Bytes(), nil
}

func writeBase64WithWrap(buf *bytes.Buffer, encoded []byte, lineSize int) {
	if lineSize <= 0 {
		lineSize = 76
	}
	for len(encoded) > lineSize {
		buf.Write(encoded[:lineSize])
		buf.WriteString("\r\n")
		encoded = encoded[lineSize:]
	}
	if len(encoded) > 0 {
		buf.Write(encoded)
	}
}

func NewAttachmentFromFile(fileName string, data []byte) EmailAttachment {
	contentType := "application/octet-stream"
	switch {
	case strings.HasSuffix(strings.ToLower(fileName), ".pdf"):
		contentType = "application/pdf"
	case strings.HasSuffix(strings.ToLower(fileName), ".zip"):
		contentType = "application/zip"
	case strings.HasSuffix(strings.ToLower(fileName), ".enc"):
		contentType = "application/octet-stream"
	case strings.HasSuffix(strings.ToLower(fileName), ".txt"):
		contentType = "text/plain; charset=UTF-8"
	}
	return EmailAttachment{
		FileName:    fileName,
		ContentType: contentType,
		Data:        data,
	}
}
