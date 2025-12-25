//go:build integration
// +build integration

package cases

import (
	"encoding/json"
	"testing"
	"time"
)

type EventResp struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

func TestEventsCRUD_PublishCancel(t *testing.T) {
	e := setup(t)

	createBody := map[string]any{
		"title":       "Integration Event",
		"description": "desc",
		"city":        "Sydney",
		"category":    "test",
		"start_time":  time.Now().UTC().Add(2 * time.Hour).Format(time.RFC3339),
		"end_time":    time.Now().UTC().Add(3 * time.Hour).Format(time.RFC3339),
		"capacity":    0,
	}

	code, env := doJSON(t, "POST", e.BaseURL+"/event/v1/events", e.UserToken, createBody)
	if code != 201 {
		t.Fatalf("create want 201 got %d err=%v", code, env.Error)
	}
	var created EventResp
	_ = json.Unmarshal(env.Data, &created)
	if created.ID == "" {
		t.Fatalf("missing id")
	}
	if created.Status != "draft" {
		t.Fatalf("want draft got %s", created.Status)
	}

	code, env = doJSON(t, "POST", e.BaseURL+"/event/v1/events/"+created.ID+"/publish", e.UserToken, nil)
	if code != 200 {
		t.Fatalf("publish want 200 got %d err=%v", code, env.Error)
	}
	var published EventResp
	_ = json.Unmarshal(env.Data, &published)
	if published.Status != "published" {
		t.Fatalf("want published got %s", published.Status)
	}

	code, env = doJSON(t, "POST", e.BaseURL+"/event/v1/events/"+created.ID+"/cancel", e.UserToken, nil)
	if code != 200 {
		t.Fatalf("cancel want 200 got %d err=%v", code, env.Error)
	}
	var canceled EventResp
	_ = json.Unmarshal(env.Data, &canceled)
	if canceled.Status != "canceled" {
		t.Fatalf("want canceled got %s", canceled.Status)
	}

	code, env = doJSON(t, "POST", e.BaseURL+"/event/v1/events/"+created.ID+"/cancel", e.UserToken, nil)
	if code != 409 {
		t.Fatalf("cancel twice want 409 got %d err=%v", code, env.Error)
	}
}
