package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"log"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"os"
	"strings"
	"time"

	"github.com/chess404/realtime/internal/httputil"
	"github.com/chess404/realtime/internal/platform"
)

type accountEmailSender interface {
	Provider() string
	Enabled() bool
	Send(ctx context.Context, delivery platform.AccountEmailDelivery) (string, error)
}

type disabledAccountEmailSender struct{}

func (disabledAccountEmailSender) Provider() string { return "disabled" }
func (disabledAccountEmailSender) Enabled() bool    { return false }
func (disabledAccountEmailSender) Send(context.Context, platform.AccountEmailDelivery) (string, error) {
	return "", fmt.Errorf("account email delivery is disabled")
}

type previewAccountEmailSender struct{}

func (previewAccountEmailSender) Provider() string { return "preview" }
func (previewAccountEmailSender) Enabled() bool    { return true }
func (previewAccountEmailSender) Send(ctx context.Context, delivery platform.AccountEmailDelivery) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}
	log.Printf("account email preview sent kind=%s delivery=%s account=%s email=%s subject=%q action=%s", delivery.Kind, delivery.DeliveryID, delivery.AccountID, delivery.Email, delivery.Subject, delivery.ActionURL)
	return "preview:" + delivery.DeliveryID, nil
}

type smtpAccountEmailSender struct {
	address   string
	host      string
	auth      smtp.Auth
	from      mail.Address
	replyTo   string
	messageID string
	useTLS    bool
}

func (s smtpAccountEmailSender) Provider() string { return "smtp" }
func (s smtpAccountEmailSender) Enabled() bool    { return true }

func (s smtpAccountEmailSender) Send(ctx context.Context, delivery platform.AccountEmailDelivery) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}
	messageID := buildEmailMessageID(s.messageID)
	payload, err := buildSMTPAccountEmailMessage(s.from, s.replyTo, messageID, delivery)
	if err != nil {
		return "", err
	}
	recipients := []string{strings.TrimSpace(delivery.Email)}
	type sendResult struct {
		err error
	}
	ch := make(chan sendResult, 1)
	go func() {
		err := sendSMTPMessage(s.address, s.auth, s.from.Address, recipients, payload, s.useTLS)
		ch <- sendResult{err}
	}()
	select {
	case r := <-ch:
		if r.err != nil {
			return "", r.err
		}
	case <-ctx.Done():
		return "", ctx.Err()
	case <-time.After(30 * time.Second):
		return "", fmt.Errorf("smtp sendmail timed out after 30s")
	}
	return messageID, nil
}

type accountEmailDispatcher struct {
	outbox      platform.AccountEmailOutboxDirectory
	sender      accountEmailSender
	now         func() time.Time
	interval    time.Duration
	batchSize   int
	maxAttempts int
	baseRetry   time.Duration
	maxRetry    time.Duration
}

func newAccountEmailDispatcher(outbox platform.AccountEmailOutboxDirectory, sender accountEmailSender, now func() time.Time) *accountEmailDispatcher {
	return &accountEmailDispatcher{
		outbox:      outbox,
		sender:      sender,
		now:         now,
		interval:    accountEmailDeliveryDispatchInterval(),
		batchSize:   accountEmailDeliveryBatchSize(),
		maxAttempts: accountEmailDeliveryMaxAttempts(),
		baseRetry:   accountEmailDeliveryBaseRetry(),
		maxRetry:    accountEmailDeliveryMaxRetry(),
	}
}

func (d *accountEmailDispatcher) Start(ctx context.Context) {
	if d == nil || d.outbox == nil || d.sender == nil || !d.sender.Enabled() {
		return
	}
	go d.run(ctx)
}

func (d *accountEmailDispatcher) run(ctx context.Context) {
	d.processBatch(ctx)
	ticker := time.NewTicker(d.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.processBatch(ctx)
		}
	}
}

func (d *accountEmailDispatcher) processBatch(ctx context.Context) {
	if d == nil || d.outbox == nil || d.sender == nil || !d.sender.Enabled() {
		return
	}
	deliveries := d.outbox.ListPendingDeliveries(d.batchSize, d.now())
	for _, delivery := range deliveries {
		d.processDelivery(ctx, delivery)
	}
}

func (d *accountEmailDispatcher) processDelivery(ctx context.Context, delivery platform.AccountEmailDelivery) {
	attemptedAt := d.now().UTC()
	provider := d.sender.Provider()
	providerMessageID, err := d.sender.Send(ctx, delivery)
	if err == nil {
		if _, recordErr := d.outbox.RecordDeliveryResult(platform.AccountEmailDeliveryResultRequest{
			DeliveryID:        delivery.DeliveryID,
			Provider:          provider,
			AttemptedAt:       attemptedAt,
			Delivered:         true,
			ProviderMessageID: providerMessageID,
		}); recordErr != nil {
			log.Printf("failed to record delivered auth email %s: %v", delivery.DeliveryID, recordErr)
		}
		return
	}

	attemptNumber := delivery.AttemptCount + 1
	terminalFailure := attemptNumber >= d.maxAttempts
	var nextAttemptAt *time.Time
	if !terminalFailure {
		next := attemptedAt.Add(d.retryDelay(attemptNumber))
		nextAttemptAt = &next
	}
	if _, recordErr := d.outbox.RecordDeliveryResult(platform.AccountEmailDeliveryResultRequest{
		DeliveryID:      delivery.DeliveryID,
		Provider:        provider,
		AttemptedAt:     attemptedAt,
		FailureReason:   err.Error(),
		NextAttemptAt:   nextAttemptAt,
		TerminalFailure: terminalFailure,
	}); recordErr != nil {
		log.Printf("failed to record auth email delivery error %s: %v", delivery.DeliveryID, recordErr)
		return
	}
	if terminalFailure {
		log.Printf("auth email delivery permanently failed kind=%s delivery=%s account=%s email=%s error=%v", delivery.Kind, delivery.DeliveryID, delivery.AccountID, delivery.Email, err)
		return
	}
	log.Printf("auth email delivery scheduled retry kind=%s delivery=%s account=%s email=%s error=%v", delivery.Kind, delivery.DeliveryID, delivery.AccountID, delivery.Email, err)
}

func (d *accountEmailDispatcher) retryDelay(attemptNumber int) time.Duration {
	if d == nil {
		return 15 * time.Second
	}
	if attemptNumber < 1 {
		attemptNumber = 1
	}
	delay := d.baseRetry
	for step := 1; step < attemptNumber; step++ {
		delay *= 2
		if delay >= d.maxRetry {
			return d.maxRetry
		}
	}
	if delay <= 0 {
		return 15 * time.Second
	}
	if delay > d.maxRetry {
		return d.maxRetry
	}
	return delay
}

func openAccountEmailSender() (accountEmailSender, error) {
	switch configuredAccountEmailDeliveryProvider() {
	case "disabled":
		return disabledAccountEmailSender{}, nil
	case "smtp":
		return newSMTPAccountEmailSender()
	default:
		return previewAccountEmailSender{}, nil
	}
}

func configuredAccountEmailDeliveryProvider() string {
	switch strings.TrimSpace(strings.ToLower(httputil.EnvOrDefault("ACCOUNT_EMAIL_DELIVERY_PROVIDER", "preview"))) {
	case "disabled":
		return "disabled"
	case "smtp":
		return "smtp"
	default:
		return "preview"
	}
}

func accountEmailDeliveryDispatchInterval() time.Duration {
	seconds := platform.ParseListLimit(strings.TrimSpace(os.Getenv("ACCOUNT_EMAIL_DELIVERY_INTERVAL_SECONDS")), 5)
	if seconds <= 0 {
		seconds = 5
	}
	return time.Duration(seconds) * time.Second
}

func accountEmailDeliveryBatchSize() int {
	value := platform.ParseListLimit(strings.TrimSpace(os.Getenv("ACCOUNT_EMAIL_DELIVERY_BATCH_SIZE")), 8)
	if value <= 0 {
		value = 8
	}
	return value
}

func accountEmailDeliveryMaxAttempts() int {
	value := platform.ParseListLimit(strings.TrimSpace(os.Getenv("ACCOUNT_EMAIL_DELIVERY_MAX_ATTEMPTS")), 5)
	if value <= 0 {
		value = 5
	}
	return value
}

func accountEmailDeliveryBaseRetry() time.Duration {
	seconds := platform.ParseListLimit(strings.TrimSpace(os.Getenv("ACCOUNT_EMAIL_DELIVERY_RETRY_BASE_SECONDS")), 15)
	if seconds <= 0 {
		seconds = 15
	}
	return time.Duration(seconds) * time.Second
}

func accountEmailDeliveryMaxRetry() time.Duration {
	seconds := platform.ParseListLimit(strings.TrimSpace(os.Getenv("ACCOUNT_EMAIL_DELIVERY_RETRY_MAX_SECONDS")), 900)
	if seconds <= 0 {
		seconds = 900
	}
	return time.Duration(seconds) * time.Second
}

func newSMTPAccountEmailSender() (accountEmailSender, error) {
	address := strings.TrimSpace(os.Getenv("ACCOUNT_EMAIL_SMTP_ADDRESS"))
	fromAddress := strings.TrimSpace(os.Getenv("ACCOUNT_EMAIL_SMTP_FROM"))
	if address == "" || fromAddress == "" {
		return nil, fmt.Errorf("smtp email delivery requires ACCOUNT_EMAIL_SMTP_ADDRESS and ACCOUNT_EMAIL_SMTP_FROM")
	}
	host := address
	if parsedHost, _, err := net.SplitHostPort(address); err == nil {
		host = parsedHost
	}
	host = strings.TrimSpace(host)
	if host == "" {
		return nil, fmt.Errorf("smtp email delivery requires a valid ACCOUNT_EMAIL_SMTP_ADDRESS")
	}
	username := strings.TrimSpace(os.Getenv("ACCOUNT_EMAIL_SMTP_USERNAME"))
	password := os.Getenv("ACCOUNT_EMAIL_SMTP_PASSWORD")
	var auth smtp.Auth
	if username != "" {
		auth = smtp.PlainAuth("", username, password, host)
	}
	// SECURITY: SMTP credentials must not be sent in cleartext. Require
	// ACCOUNT_EMAIL_SMTP_TLS=true for any non-loopback host, or reject
	// delivery outright. This is a hard fail-on-misconfig: the audit
	// flagged this as a launch blocker.
	tlsRequired := strings.EqualFold(strings.TrimSpace(os.Getenv("ACCOUNT_EMAIL_SMTP_TLS")), "true")
	isLoopback := host == "localhost" || host == "127.0.0.1" || host == "::1"
	if !tlsRequired && !isLoopback {
		return nil, fmt.Errorf("ACCOUNT_EMAIL_SMTP_TLS=true is required for non-loopback SMTP host %q (cleartext credentials are not allowed)", host)
	}
	from := mail.Address{
		Name:    strings.TrimSpace(os.Getenv("ACCOUNT_EMAIL_SMTP_FROM_NAME")),
		Address: fromAddress,
	}
	if !strings.Contains(from.Address, "@") {
		return nil, fmt.Errorf("smtp email delivery requires a valid ACCOUNT_EMAIL_SMTP_FROM")
	}
	return smtpAccountEmailSender{
		address:   address,
		host:      host,
		auth:      auth,
		from:      from,
		replyTo:   strings.TrimSpace(os.Getenv("ACCOUNT_EMAIL_SMTP_REPLY_TO")),
		messageID: accountEmailSMTPMessageDomain(host),
		useTLS:    tlsRequired,
	}, nil
}

func accountEmailSMTPMessageDomain(host string) string {
	domain := strings.TrimSpace(os.Getenv("ACCOUNT_EMAIL_SMTP_MESSAGE_DOMAIN"))
	if domain != "" {
		return domain
	}
	if host != "" {
		return host
	}
	return "chess404.local"
}

// sendSMTPMessage sends a single SMTP message. If useTLS is true, it
// dials the server over implicit TLS on port 465 (the smtps scheme),
// which keeps the auth credentials off the wire in cleartext.
func sendSMTPMessage(address string, auth smtp.Auth, from string, to []string, msg []byte, useTLS bool) error {
	if !useTLS {
		return smtp.SendMail(address, auth, from, to, msg)
	}
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("parse smtp address: %w", err)
	}
	conn, err := tls.Dial("tcp", address, &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12})
	if err != nil {
		return fmt.Errorf("smtp tls dial: %w", err)
	}
	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return fmt.Errorf("smtp new client: %w", err)
	}
	defer func() { _ = client.Quit() }()
	if auth != nil {
		if ok, _ := client.Extension("AUTH"); ok {
			if err := client.Auth(auth); err != nil {
				return fmt.Errorf("smtp auth: %w", err)
			}
		}
	}
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("smtp MAIL FROM: %w", err)
	}
	for _, recipient := range to {
		if err := client.Rcpt(recipient); err != nil {
			return fmt.Errorf("smtp RCPT TO %q: %w", recipient, err)
		}
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp DATA: %w", err)
	}
	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("smtp DATA write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("smtp DATA close: %w", err)
	}
	return nil
}

func buildSMTPAccountEmailMessage(from mail.Address, replyTo, messageID string, delivery platform.AccountEmailDelivery) ([]byte, error) {
	boundary := "chess404-mail-" + randomHex(12)
	var body bytes.Buffer
	writer := quotedprintable.NewWriter(&body)
	if _, err := writer.Write([]byte(strings.TrimSpace(delivery.TextBody))); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	var htmlBody bytes.Buffer
	htmlWriter := quotedprintable.NewWriter(&htmlBody)
	if _, err := htmlWriter.Write([]byte(strings.TrimSpace(delivery.HTMLBody))); err != nil {
		return nil, err
	}
	if err := htmlWriter.Close(); err != nil {
		return nil, err
	}

	var message bytes.Buffer
	message.WriteString("From: " + from.String() + "\r\n")
	message.WriteString("To: " + (&mail.Address{Address: delivery.Email}).String() + "\r\n")
	if strings.TrimSpace(replyTo) != "" {
		message.WriteString("Reply-To: " + strings.TrimSpace(replyTo) + "\r\n")
	}
	message.WriteString("Subject: " + mime.QEncoding.Encode("utf-8", delivery.Subject) + "\r\n")
	message.WriteString("Date: " + time.Now().UTC().Format(time.RFC1123Z) + "\r\n")
	message.WriteString("Message-ID: " + messageID + "\r\n")
	message.WriteString("MIME-Version: 1.0\r\n")
	message.WriteString("Content-Type: multipart/alternative; boundary=\"" + boundary + "\"\r\n")
	message.WriteString("\r\n")
	message.WriteString("--" + boundary + "\r\n")
	message.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	message.WriteString("Content-Transfer-Encoding: quoted-printable\r\n\r\n")
	message.Write(body.Bytes())
	message.WriteString("\r\n--" + boundary + "\r\n")
	message.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	message.WriteString("Content-Transfer-Encoding: quoted-printable\r\n\r\n")
	message.Write(htmlBody.Bytes())
	message.WriteString("\r\n--" + boundary + "--\r\n")
	return message.Bytes(), nil
}

func buildEmailMessageID(domain string) string {
	resolvedDomain := strings.TrimSpace(domain)
	if resolvedDomain == "" {
		resolvedDomain = "chess404.local"
	}
	return fmt.Sprintf("<%s@%s>", randomHex(12), resolvedDomain)
}

func randomHex(byteCount int) string {
	if byteCount <= 0 {
		byteCount = 8
	}
	buf := make([]byte, byteCount)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf)
}
