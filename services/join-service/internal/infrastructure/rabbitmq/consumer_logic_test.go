package rabbitmq

import (
	"context"
	"encoding/json"
	"io"
	"testing"

	"github.com/baechuer/real-time-ressys/services/join-service/internal/contracts/event"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockRepo struct {
	mock.Mock
}

func (m *MockRepo) InitCapacityTx(ctx context.Context, tx pgx.Tx, e uuid.UUID, c int) error {
	// ✅ 关键：把 tx 也传进 mock，否则就会变成 3 参数调用
	args := m.Called(ctx, tx, e, c)
	return args.Error(0)
}

// 这里保留旧方法（如果你其他测试/代码会用到），但 applySnapshotTx 用的是 HandleEventCanceledTx（type assertion）
func (m *MockRepo) HandleEventCanceled(ctx context.Context, t string, e uuid.UUID, r string) error {
	return nil
}

func TestApplySnapshotTx_Published(t *testing.T) {
	repo := new(MockRepo)
	ctx := context.Background()

	eid := uuid.New()
	capacity := 100
	payload := event.EventPublishedPayload{
		EventID:  eid.String(),
		Capacity: &capacity,
		Status:   "published",
	}
	payloadBytes, _ := json.Marshal(payload)

	// ✅ 期望也改成 4 参数：ctx, tx(any), eventID, capacity
	repo.On("InitCapacityTx", ctx, mock.Anything, eid, 100).Return(nil).Once()

	err := applySnapshotTx(ctx, repo, nil, "event.published", payloadBytes, "trace-1", loggerStub())
	assert.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestApplySnapshotTx_Canceled(t *testing.T) {
	repo := new(MockRepo)
	ctx := context.Background()
	eid := uuid.New()

	payload := event.EventCanceledPayload{EventID: eid.String(), Reason: "Rain"}
	payloadBytes, _ := json.Marshal(payload)

	// fallback path：repo 没实现 canceledHandler（HandleEventCanceledTx）时，capacity = -1
	repo.On("InitCapacityTx", ctx, mock.Anything, eid, -1).Return(nil).Once()

	err := applySnapshotTx(ctx, repo, nil, "event.canceled", payloadBytes, "trace-1", loggerStub())
	assert.NoError(t, err)
	repo.AssertExpectations(t)
}

func loggerStub() zerolog.Logger {
	return zerolog.New(io.Discard)
}
