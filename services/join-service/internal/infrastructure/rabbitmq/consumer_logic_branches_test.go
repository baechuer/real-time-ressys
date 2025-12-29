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

type RepoCancelMock struct {
	mock.Mock
}

func (m *RepoCancelMock) InitCapacityTx(ctx context.Context, tx pgx.Tx, eid uuid.UUID, cap int) error {
	args := m.Called(ctx, tx, eid, cap)
	return args.Error(0)
}

func (m *RepoCancelMock) HandleEventCanceledTx(ctx context.Context, tx pgx.Tx, traceID string, eid uuid.UUID, reason string) error {
	args := m.Called(ctx, tx, traceID, eid, reason)
	return args.Error(0)
}

func TestApplySnapshotTx_Published_MissingCapacity_IsIgnored(t *testing.T) {
	repo := new(MockRepo)
	ctx := context.Background()
	eid := uuid.New()

	payload := event.EventPublishedPayload{
		EventID:  eid.String(),
		Capacity: nil, // ✅ missing
		Status:   "published",
	}
	b, _ := json.Marshal(payload)

	err := applySnapshotTx(ctx, repo, nil, "event.published", b, "trace-miss-cap", loggerStub())
	assert.NoError(t, err)
	repo.AssertNotCalled(t, "InitCapacityTx", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestApplySnapshotTx_Published_InvalidEventID_IsIgnored(t *testing.T) {
	repo := new(MockRepo)
	ctx := context.Background()
	capacity := 10

	payload := event.EventPublishedPayload{
		EventID:  "not-a-uuid",
		Capacity: &capacity,
		Status:   "published",
	}
	b, _ := json.Marshal(payload)

	err := applySnapshotTx(ctx, repo, nil, "event.published", b, "trace-bad-uuid", loggerStub())
	assert.NoError(t, err)
	repo.AssertNotCalled(t, "InitCapacityTx", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestApplySnapshotTx_Canceled_EmptyReason_Defaults(t *testing.T) {
	repo := new(RepoCancelMock)
	ctx := context.Background()
	eid := uuid.New()

	payload := event.EventCanceledPayload{
		EventID: eid.String(),
		Reason:  "", // ✅ should default to "event_canceled" in your code
	}
	b, _ := json.Marshal(payload)

	repo.On("HandleEventCanceledTx", ctx, mock.Anything, "trace-def", eid, "event_canceled").Return(nil).Once()

	err := applySnapshotTx(ctx, repo, nil, "event.canceled", b, "trace-def", loggerStub())
	assert.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestApplySnapshotTx_Canceled_LegacyIDField_StillWorks(t *testing.T) {
	repo := new(RepoCancelMock)
	ctx := context.Background()
	eid := uuid.New()

	// ✅ legacy: payload has `id` not `event_id`
	legacy := map[string]any{
		"id":     eid.String(),
		"reason": "legacy-cancel",
	}
	b, _ := json.Marshal(legacy)

	repo.On("HandleEventCanceledTx", ctx, mock.Anything, "trace-legacy", eid, "legacy-cancel").Return(nil).Once()

	err := applySnapshotTx(ctx, repo, nil, "event.canceled", b, "trace-legacy", loggerStub())
	assert.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestApplySnapshotTx_UnknownRoutingKey_IsIgnored(t *testing.T) {
	repo := new(MockRepo)
	ctx := context.Background()

	err := applySnapshotTx(ctx, repo, nil, "event.unknown", []byte(`{"x":1}`), "trace-unk", loggerStub())
	assert.NoError(t, err)
	repo.AssertNotCalled(t, "InitCapacityTx", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}
