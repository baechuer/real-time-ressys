package email

import (
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func TestRenderBasicHTML(t *testing.T) {
	title := "Hello & Welcome"
	link := "https://example.com/verify?token=123"

	htmlOutput := renderBasicHTML(title, "intro", "button", link)

	// 验证 HTML 转义，防止 XSS
	assert.Contains(t, htmlOutput, "Hello &amp; Welcome")
	// 验证链接是否正确嵌入
	assert.Contains(t, htmlOutput, "href=\"https://example.com/verify?token=123\"")
	assert.Contains(t, htmlOutput, "background:#111")
}

func TestContainsAny(t *testing.T) {
	msg := "535 Authentication Failed"

	assert.True(t, containsAny(msg, "535", "auth"))
	assert.False(t, containsAny(msg, "404", "missing"))
	assert.False(t, containsAny(msg, ""))
}

func TestSMTPSender_Config(t *testing.T) {
	cfg := SMTPConfig{
		Host:     "smtp.gmail.com",
		Port:     587,
		Username: "user",
		Password: "password",
		From:     "noreply@test.com",
		Timeout:  5 * time.Second,
	}

	sender := NewSMTPSender(cfg, zerolog.Nop())

	assert.Equal(t, "smtp.gmail.com", sender.host)
	assert.Equal(t, 587, sender.port)
	assert.Equal(t, 5*time.Second, sender.timeout)
}

// 注意：由于 go-mail 内部 NewClient 会尝试解析主机名，
// 真正的 send 逻辑建议使用 Integration Test (集成测试) 配合 Docker Mailpit。
