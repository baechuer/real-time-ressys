package email

import (
	"fmt"
	"html/template"
	"strings"
)

// RenderVerificationTemplate renders the email verification template
func RenderVerificationTemplate(email, url string) (string, error) {
	tmpl := `<!DOCTYPE html>
<html>
<head>
	<meta charset="UTF-8">
	<title>Verify Your Email</title>
</head>
<body style="font-family: Arial, sans-serif; line-height: 1.6; color: #333;">
	<div style="max-width: 600px; margin: 0 auto; padding: 20px;">
		<h1 style="color: #4CAF50;">Verify Your Email Address</h1>
		<p>Hello,</p>
		<p>Thank you for signing up! Please verify your email address by clicking the button below:</p>
		<div style="text-align: center; margin: 30px 0;">
			<a href="{{.URL}}" style="background-color: #4CAF50; color: white; padding: 12px 30px; text-decoration: none; border-radius: 5px; display: inline-block;">Verify Email</a>
		</div>
		<p>Or copy and paste this link into your browser:</p>
		<p style="word-break: break-all; color: #666;">{{.URL}}</p>
		<p>This link will expire in 24 hours.</p>
		<p>If you didn't create an account, please ignore this email.</p>
		<hr style="border: none; border-top: 1px solid #eee; margin: 20px 0;">
		<p style="color: #999; font-size: 12px;">This is an automated message, please do not reply.</p>
	</div>
</body>
</html>`

	t, err := template.New("verification").Parse(tmpl)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	data := struct {
		Email string
		URL   string
	}{
		Email: email,
		URL:   url,
	}

	var buf strings.Builder
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// RenderPasswordResetTemplate renders the password reset template
func RenderPasswordResetTemplate(email, url string) (string, error) {
	tmpl := `<!DOCTYPE html>
<html>
<head>
	<meta charset="UTF-8">
	<title>Reset Your Password</title>
</head>
<body style="font-family: Arial, sans-serif; line-height: 1.6; color: #333;">
	<div style="max-width: 600px; margin: 0 auto; padding: 20px;">
		<h1 style="color: #2196F3;">Reset Your Password</h1>
		<p>Hello,</p>
		<p>We received a request to reset your password. Click the button below to create a new password:</p>
		<div style="text-align: center; margin: 30px 0;">
			<a href="{{.URL}}" style="background-color: #2196F3; color: white; padding: 12px 30px; text-decoration: none; border-radius: 5px; display: inline-block;">Reset Password</a>
		</div>
		<p>Or copy and paste this link into your browser:</p>
		<p style="word-break: break-all; color: #666;">{{.URL}}</p>
		<p>This link will expire in 1 hour.</p>
		<p>If you didn't request a password reset, please ignore this email. Your password will remain unchanged.</p>
		<hr style="border: none; border-top: 1px solid #eee; margin: 20px 0;">
		<p style="color: #999; font-size: 12px;">This is an automated message, please do not reply.</p>
	</div>
</body>
</html>`

	t, err := template.New("password_reset").Parse(tmpl)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	data := struct {
		Email string
		URL   string
	}{
		Email: email,
		URL:   url,
	}

	var buf strings.Builder
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

