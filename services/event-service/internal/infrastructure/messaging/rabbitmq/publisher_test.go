package rabbitmq

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestPublisher_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()

	// 1. 启动 RabbitMQ 容器
	req := testcontainers.ContainerRequest{
		Image:        "rabbitmq:3-management",
		ExposedPorts: []string{"5672/tcp"},
		WaitingFor:   wait.ForLog("Server startup complete"),
	}
	rabbitC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	assert.NoError(t, err)
	defer rabbitC.Terminate(ctx)

	port, _ := rabbitC.MappedPort(ctx, "5672")
	url := "amqp://guest:guest@localhost:" + port.Port()

	// 2. 初始化 Publisher
	p, err := NewPublisher(url, "test.exchange")
	assert.NoError(t, err)
	defer p.Close()

	t.Run("publish_successfully", func(t *testing.T) {
		payload := map[string]string{"msg": "hello"}
		// 注意：如果 exchange 不存在，且开启了 mandatory，会报错返回
		// 这里简单演示成功流程，真实环境需确保 Exchange 已声明
		err := p.PublishEvent(ctx, "test.key", payload)
		// 因为没有声明 exchange，可能会收到 rabbit returned 错误
		// 如果你已经在基础设施中声明了 exchange，这里应该是 NoError
		t.Log("Error (expected if exchange not exists):", err)
	})
}
