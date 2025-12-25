package postgres

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/baechuer/real-time-ressys/services/event-service/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestRepo_Create(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	repo := New(db)
	now := time.Now().UTC()
	e := &domain.Event{
		ID: "evt_1", OwnerID: "user_1", Title: "Sydney Meetup",
		StartTime: now, EndTime: now.Add(time.Hour), Status: domain.StatusDraft,
		CreatedAt: now, UpdatedAt: now,
	}

	// 验证 SQL 执行和参数绑定
	// 注意：ExpectExec 里的正则匹配需要处理 SQL 中的换行和空格
	mock.ExpectExec("INSERT INTO events").
		WithArgs(
			e.ID, e.OwnerID, e.Title, e.Description, e.City, e.Category,
			e.StartTime, e.EndTime, e.Capacity, string(e.Status),
			e.PublishedAt, e.CanceledAt, e.CreatedAt, e.UpdatedAt,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = repo.Create(context.Background(), e)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepo_GetByID(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	repo := New(db)
	eventID := "evt_123"

	t.Run("success_mapping", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{
			"id", "owner_id", "title", "description", "city", "category",
			"start_time", "end_time", "capacity", "status",
			"published_at", "canceled_at", "created_at", "updated_at",
		}).AddRow(
			eventID, "owner_1", "Title", "Desc", "Syd", "Cat",
			time.Now(), time.Now(), 0, "published",
			nil, nil, time.Now(), time.Now(),
		)

		mock.ExpectQuery("SELECT (.+) FROM events WHERE id =").
			WithArgs(eventID).
			WillReturnRows(rows)

		ev, err := repo.GetByID(context.Background(), eventID)
		assert.NoError(t, err)
		assert.Equal(t, eventID, ev.ID)
		assert.Equal(t, domain.StatusPublished, ev.Status)
	})

	t.Run("not_found_mapping", func(t *testing.T) {
		mock.ExpectQuery("SELECT").WithArgs("none").WillReturnError(sql.ErrNoRows)

		ev, err := repo.GetByID(context.Background(), "none")
		assert.Error(t, err)
		assert.Nil(t, ev)
		// 验证是否正确转换为了 domain 层的错误
		assert.Contains(t, err.Error(), "event not found")
	})
}
