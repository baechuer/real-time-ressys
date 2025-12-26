package postgres

import (
	"context"
	"database/sql/driver"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/baechuer/real-time-ressys/services/event-service/internal/application/event"
	"github.com/stretchr/testify/assert"
)

func newEventRow(id string) []driver.Value {
	return []driver.Value{
		id, "owner_1", "Title", "Desc", "Sydney", "Tech",
		time.Now().UTC(), time.Now().Add(time.Hour).UTC(), 100, "published",
		nil, nil, time.Now().UTC(), time.Now().UTC(),
	}
}

func TestRepo_ListPublicTimeKeyset(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	repo := New(db)

	t.Run("first_page_without_cursor", func(t *testing.T) {
		f := event.ListFilter{PageSize: 10, City: "Sydney"}
		rows := sqlmock.NewRows([]string{
			"id", "owner_id", "title", "description", "city", "category",
			"start_time", "end_time", "capacity", "status",
			"published_at", "canceled_at", "created_at", "updated_at",
		}).AddRow(newEventRow("e1")...)

		// 修复：使用 sqlmock.QueryMatcherEqual 或更宽松的正则
		// 注意这里匹配的是 buildPublicBaseWhere 生成的 SQL 部分
		mock.ExpectQuery(`SELECT (.+) FROM events WHERE status = 'published' AND city = \$1 ORDER BY start_time ASC, id ASC LIMIT \$2`).
			WithArgs("Sydney", 10).
			WillReturnRows(rows)

		items, err := repo.ListPublicTimeKeyset(context.Background(), f, false, time.Time{}, "")
		assert.NoError(t, err)
		assert.Len(t, items, 1)
	})

	t.Run("second_page_with_cursor", func(t *testing.T) {
		lastTime := time.Now().UTC()
		lastID := "e1"
		f := event.ListFilter{PageSize: 10}

		rows := sqlmock.NewRows([]string{
			"id", "owner_id", "title", "description", "city", "category",
			"start_time", "end_time", "capacity", "status",
			"published_at", "canceled_at", "created_at", "updated_at",
		}).AddRow(newEventRow("e2")...)

		// 修复：Keyset 谓词正则
		mock.ExpectQuery(`WHERE status = 'published' AND \(start_time, id\) > \(\$1, \$2\) ORDER BY start_time ASC, id ASC LIMIT \$3`).
			WithArgs(lastTime, lastID, 10).
			WillReturnRows(rows)

		items, err := repo.ListPublicTimeKeyset(context.Background(), f, true, lastTime, lastID)
		assert.NoError(t, err)
		assert.Len(t, items, 1)
	})
}

func TestRepo_ListPublicRelevanceKeyset(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	repo := New(db)

	t.Run("search_with_rank_mapping", func(t *testing.T) {
		f := event.ListFilter{Query: "Go", PageSize: 5}

		// 修复：Relevance 模式下 Scan 包含 rank 字段
		rows := sqlmock.NewRows([]string{
			"id", "owner_id", "title", "description", "city", "category",
			"start_time", "end_time", "capacity", "status",
			"published_at", "canceled_at", "created_at", "updated_at",
			"rank",
		}).AddRow(append(newEventRow("e1"), 0.95)...)

		// 匹配 ts_rank_cd 部分
		mock.ExpectQuery(`SELECT (.+) ts_rank_cd\(search_vector, plainto_tsquery\('simple', \$1\)\) AS rank FROM events WHERE status = 'published' AND search_vector @@ plainto_tsquery\('simple', \$1\) ORDER BY rank DESC, start_time ASC, id ASC LIMIT \$2`).
			WithArgs("Go", 5).
			WillReturnRows(rows)

		items, ranks, err := repo.ListPublicRelevanceKeyset(context.Background(), f, false, 0, time.Time{}, "")

		// 检查错误，防止 Panic
		if assert.NoError(t, err) && assert.Len(t, items, 1) {
			assert.Equal(t, 0.95, ranks[0])
		}
	})
}
