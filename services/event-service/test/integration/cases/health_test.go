//go:build integration
// +build integration

package cases

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/baechuer/real-time-ressys/services/event-service/internal/transport/http/dto"
	"github.com/stretchr/testify/assert"
)

func TestHealthz(t *testing.T) {
	e := setup(t)
	resp, err := http.Get(e.BaseURL + "/healthz")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200 got %d", resp.StatusCode)
	}
}
func TestEvents_KeysetPagination(t *testing.T) {
	e := setup(t)

	// 1. 批量创建并发布 5 个活动
	for i := 1; i <= 5; i++ {
		body := map[string]any{
			"title":       fmt.Sprintf("Pagination Event %d", i),
			"description": "Integration test for keyset pagination",
			"city":        "Sydney",
			"category":    "test",
			"start_time":  time.Now().UTC().Add(time.Duration(i) * time.Hour).Format(time.RFC3339),
			"end_time":    time.Now().UTC().Add(time.Duration(i+1) * time.Hour).Format(time.RFC3339),
			"capacity":    100, // 显式添加字段
		}
		code, env := doJSON(t, "POST", e.BaseURL+"/event/v1/events", e.OrganizerToken, body)
		if code != 201 {
			t.Fatalf("setup create failed at index %d: want 201 got %d, err: %+v", i, code, env.Error)
		}

		var created struct{ ID string }
		_ = json.Unmarshal(env.Data, &created)

		pubCode, pubEnv := doJSON(t, "POST", e.BaseURL+"/event/v1/events/"+created.ID+"/publish", e.OrganizerToken, nil)
		if pubCode != 200 {
			t.Fatalf("setup publish failed at index %d: want 200 got %d, err: %+v", i, pubCode, pubEnv.Error)
		}
	}

	// 2. 第一页获取 2 条
	code, env := doJSON(t, "GET", e.BaseURL+"/event/v1/events?page_size=2&sort=time", "", nil)
	assert.Equal(t, 200, code)

	var resp dto.PageResp[dto.EventResp]
	err := json.Unmarshal(env.Data, &resp)
	assert.NoError(t, err)
	assert.Len(t, resp.Items, 2)
	assert.NotEmpty(t, resp.NextCursor, "First page should return a cursor")

	// 3. 使用 NextCursor 获取第二页
	cursor := resp.NextCursor
	code, env = doJSON(t, "GET", e.BaseURL+"/event/v1/events?page_size=2&sort=time&cursor="+cursor, "", nil)
	assert.Equal(t, 200, code)

	var resp2 dto.PageResp[dto.EventResp]
	_ = json.Unmarshal(env.Data, &resp2)
	assert.Len(t, resp2.Items, 2, "Second page should return 2 more items")

	// 验证分页的连续性：第一页的最后一条 ID 不应该出现在第二页
	assert.NotEqual(t, resp.Items[1].ID, resp2.Items[0].ID)
}
func TestEvents_Search(t *testing.T) {
	e := setup(t)

	createAndPublish(t, e, "Golang Workshop", "Learn microservices")
	createAndPublish(t, e, "Python Party", "Data science fun")

	code, env := doJSON(t, "GET", e.BaseURL+"/event/v1/events?q=microservices&sort=relevance", "", nil)
	assert.Equal(t, http.StatusOK, code) //

	var resp dto.PageResp[dto.EventResp]
	_ = json.Unmarshal(env.Data, &resp)

	assert.True(t, len(resp.Items) >= 1, "Should find at least one event with 'microservices'")
	assert.Contains(t, resp.Items[0].Title, "Golang")
}
