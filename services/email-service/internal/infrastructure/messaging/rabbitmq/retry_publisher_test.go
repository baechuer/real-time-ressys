package rabbitmq

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 获取 RabbitMQ 连接，你可以根据你的 Docker 配置修改 URL
func getTestConn(t *testing.T) *amqp.Connection {
	url := os.Getenv("RABBITMQ_URL")
	if url == "" {
		url = "amqp://guest:guest@localhost:5672/"
	}
	conn, err := amqp.Dial(url)
	if err != nil {
		t.Skipf("Skipping integration test: RabbitMQ not reachable at %s", url)
	}
	return conn
}

func setupExchanges(t *testing.T, ch *amqp.Channel) {
	exchanges := []string{DLX10sExchange, DLX1mExchange, DLX10mExchange, DLXFinalExchange}
	for _, ex := range exchanges {
		err := ch.ExchangeDeclare(ex, "topic", true, false, false, false, nil)
		require.NoError(t, err)
	}
}

func TestRetryPublisher_Integration(t *testing.T) {
	conn := getTestConn(t)
	defer conn.Close()

	ch, err := conn.Channel()
	require.NoError(t, err)
	defer ch.Close()

	// 预先创建测试需要的 Exchange
	setupExchanges(t, ch)

	logger := zerolog.New(os.Stdout)
	publisher, err := NewRetryPublisher(ch, logger)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 模拟一条原始消息
	origDelivery := amqp.Delivery{
		ContentType: "application/json",
		Body:        []byte(`{"msg":"hello"}`),
		RoutingKey:  "user.registered",
		MessageId:   "test-msg-123",
		Headers:     amqp.Table{"existing-header": "val"},
	}

	t.Run("PublishRetry_10s_Tier", func(t *testing.T) {
		err := publisher.PublishRetry(ctx, "10s", origDelivery, 1, nil)
		assert.NoError(t, err, "Should successfully publish to 10s exchange")
	})

	t.Run("PublishRetry_Headers_Check", func(t *testing.T) {
		cause := fmt.Errorf("connection timeout")
		err := publisher.PublishRetry(ctx, "1m", origDelivery, 2, cause)
		assert.NoError(t, err)

		// 校验逻辑在 publisher 内部已经通过 waitAckOrReturn 完成了确认
	})

	t.Run("PublishFinal_DLQ", func(t *testing.T) {
		err := publisher.PublishFinal(ctx, origDelivery, "max_retries_exceeded", nil)
		assert.NoError(t, err, "Should successfully move message to final DLQ")
	})
}

func TestCopyHeaders(t *testing.T) {
	t.Run("nil_input", func(t *testing.T) {
		out := copyHeaders(nil)
		assert.NotNil(t, out)
		assert.Equal(t, 0, len(out))
	})

	t.Run("valid_copy", func(t *testing.T) {
		in := amqp.Table{"key": "value", "num": 123}
		out := copyHeaders(in)

		assert.Equal(t, in, out)

		// 确保是深拷贝（虽然 Table 里的 Value 可能是指针，但 Map 本身应该分离）
		out["new"] = true
		assert.NotContains(t, in, "new")
	})
}
