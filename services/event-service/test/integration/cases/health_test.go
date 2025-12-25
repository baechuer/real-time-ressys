//go:build integration
// +build integration

package cases

import (
	"net/http"
	"testing"
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
