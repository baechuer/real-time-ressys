//go:build integration
// +build integration

package cases

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/baechuer/real-time-ressys/services/event-service/test/integration/infra"
)

func TestOrganizerACL_OwnerOtherAdmin(t *testing.T) {
	e := setup(t)

	otherToken, err := infra.MakeToken(e.JWTSecret, e.JWTIssuer, "33333333-3333-3333-3333-333333333333", "user", 0, 15*time.Minute)
	if err != nil {
		t.Fatalf("make other token: %v", err)
	}

	createBody := map[string]any{
		"title":       "ACL Event",
		"description": "desc",
		"city":        "Sydney",
		"category":    "test",
		"start_time":  time.Now().UTC().Add(2 * time.Hour).Format(time.RFC3339),
		"end_time":    time.Now().UTC().Add(3 * time.Hour).Format(time.RFC3339),
		"capacity":    0,
	}

	code, env := doJSON(t, "POST", e.BaseURL+"/event/v1/events", e.OrganizerToken, createBody)
	if code != 201 {
		t.Fatalf("create want 201 got %d err=%v", code, env.Error)
	}
	var created struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(env.Data, &created)

	// Other user publish -> 403
	code, env = doJSON(t, "POST", e.BaseURL+"/event/v1/events/"+created.ID+"/publish", otherToken, nil)
	if code != 403 {
		t.Fatalf("other publish want 403 got %d err=%v", code, env.Error)
	}

	// Admin publish -> 200
	code, env = doJSON(t, "POST", e.BaseURL+"/event/v1/events/"+created.ID+"/publish", e.AdminToken, nil)
	if code != 200 {
		t.Fatalf("admin publish want 200 got %d err=%v", code, env.Error)
	}
}
