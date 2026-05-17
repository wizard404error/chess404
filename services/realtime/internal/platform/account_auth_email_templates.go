package platform

import (
	"fmt"
	"net/url"
	"path"
	"strings"
)

func BuildAccountEmailVerificationDelivery(account AccountProfile, challenge AccountEmailVerificationChallenge, publicBaseURL string) AccountEmailDeliveryRequest {
	handle := strings.TrimSpace(account.Handle)
	if handle == "" {
		handle = "player"
	}
	actionURL := buildAccountAuthActionURL(publicBaseURL, "verify-email", challenge.AccountID, challenge.Token)
	subject := "Verify your Chess404 email"
	textBody := strings.TrimSpace(fmt.Sprintf(
		"Hi @%s,\n\nUse the link below to verify your Chess404 email for %s.\n\nVerify: %s\n\nVerification token: %s\n\nIf you did not request this, you can ignore this email.\n",
		handle,
		challenge.Email,
		actionURL,
		challenge.Token,
	))
	htmlBody := strings.TrimSpace(fmt.Sprintf(
		`<p>Hi <strong>@%s</strong>,</p><p>Use the link below to verify your Chess404 email for <strong>%s</strong>.</p><p><a href="%s">Verify your email</a></p><p>If the button does not work, use this verification token:</p><pre>%s</pre><p>If you did not request this, you can ignore this email.</p>`,
		htmlEscape(handle),
		htmlEscape(challenge.Email),
		htmlEscape(actionURL),
		htmlEscape(challenge.Token),
	))
	return AccountEmailDeliveryRequest{
		AccountID: challenge.AccountID,
		Email:     challenge.Email,
		Kind:      AccountEmailDeliveryKindEmailVerification,
		Subject:   subject,
		TextBody:  textBody,
		HTMLBody:  htmlBody,
		ActionURL: actionURL,
	}
}

func BuildAccountPasswordResetDelivery(account AccountProfile, challenge AccountPasswordResetChallenge, publicBaseURL string) AccountEmailDeliveryRequest {
	handle := strings.TrimSpace(account.Handle)
	if handle == "" {
		handle = "player"
	}
	actionURL := buildAccountAuthActionURL(publicBaseURL, "reset-password", challenge.AccountID, challenge.Token)
	subject := "Reset your Chess404 password"
	textBody := strings.TrimSpace(fmt.Sprintf(
		"Hi @%s,\n\nUse the link below to reset the Chess404 password for %s.\n\nReset password: %s\n\nReset token: %s\n\nIf you did not request this, you can ignore this email.\n",
		handle,
		challenge.Email,
		actionURL,
		challenge.Token,
	))
	htmlBody := strings.TrimSpace(fmt.Sprintf(
		`<p>Hi <strong>@%s</strong>,</p><p>Use the link below to reset the Chess404 password for <strong>%s</strong>.</p><p><a href="%s">Reset your password</a></p><p>If the button does not work, use this reset token:</p><pre>%s</pre><p>If you did not request this, you can ignore this email.</p>`,
		htmlEscape(handle),
		htmlEscape(challenge.Email),
		htmlEscape(actionURL),
		htmlEscape(challenge.Token),
	))
	return AccountEmailDeliveryRequest{
		AccountID: challenge.AccountID,
		Email:     challenge.Email,
		Kind:      AccountEmailDeliveryKindPasswordReset,
		Subject:   subject,
		TextBody:  textBody,
		HTMLBody:  htmlBody,
		ActionURL: actionURL,
	}
}

func buildAccountAuthActionURL(publicBaseURL, action, accountID, token string) string {
	base := strings.TrimSpace(publicBaseURL)
	if base == "" {
		base = "http://127.0.0.1:3000"
	}
	parsed, err := url.Parse(base)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		parsed, _ = url.Parse("http://127.0.0.1:3000")
	}
	parsed.Path = path.Clean(strings.TrimRight(parsed.Path, "/") + "/")
	query := parsed.Query()
	query.Set("auth", strings.TrimSpace(action))
	query.Set("account", strings.TrimSpace(accountID))
	query.Set("token", strings.TrimSpace(token))
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func htmlEscape(value string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&#39;",
	)
	return replacer.Replace(value)
}
