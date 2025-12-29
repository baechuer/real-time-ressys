package rabbitmq

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/baechuer/real-time-ressys/services/join-service/internal/contracts/event"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type CanceledRepo struct {
	mock.Mock
}

func (m *CanceledRepo) InitCapacityTx(ctx context.Context, tx pgx.Tx, eid uuid.UUID, cap int) error {
	args := m.Called(ctx, tx, eid, cap)
	return args.Error(0)
}

func (m *CanceledRepo) HandleEventCanceledTx(ctx context.Context, tx pgx.Tx, traceID string, eid uuid.UUID, reason string) error {
	args := m.Called(ctx, tx, traceID, eid, reason)
	return args.Error(0)
}

func TestApplySnapshotTx_Updated_UpdatesCapacity(t *testing.T) {
	repo := new(MockRepo)
	ctx := context.Background()
	eid := uuid.New()
	capacity := 77

	payload := event.EventUpdatedPayload{
		EventID:  eid.String(),
		Capacity: &capacity,
		Status:   "published",
	}
	b, _ := json.Marshal(payload)

	repo.On("InitCapacityTx", ctx, mock.Anything, eid, 77).Return(nil).Once()

	err := applySnapshotTx(ctx, repo, nil, "event.updated", b, "trace-2", loggerStub())
	assert.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestApplySnapshotTx_InvalidJSON_IsIgnored(t *testing.T) {
	repo := new(MockRepo)
	ctx := context.Background()

	err := applySnapshotTx(ctx, repo, nil, "event.published", []byte("{not-json"), "trace-x", loggerStub())
	assert.NoError(t, err)

	repo.AssertNotCalled(t, "InitCapacityTx", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestApplySnapshotTx_Canceled_PrefersCanceledHandlerWhenImplemented(t *testing.T) {
	repo := new(CanceledRepo)
	ctx := context.Background()
	eid := uuid.New()

	payload := event.EventCanceledPayload{EventID: eid.String(), Reason: "Rain"}
	b, _ := json.Marshal(payload)

	repo.On("HandleEventCanceledTx", ctx, mock.Anything, "trace-3", eid, "Rain").Return(nil).Once()

	err := applySnapshotTx(ctx, repo, nil, "event.canceled", b, "trace-3", loggerStub())
	assert.NoError(t, err)
	repo.AssertExpectations(t)
}
