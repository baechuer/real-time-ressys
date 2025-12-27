package event

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

func cacheKeyEventDetails(id string) string {
	return fmt.Sprintf("event:%s", id)
}

// Generate a deterministic key for list filters
func cacheKeyPublicList(f ListFilter) string {
	// Only cache "first page" style logic is handled in the caller,
	// here we just hash the filter params.
	// Key: events:public:list:{hash_of_params}

	// Normalize times to RFC3339 to avoid pointer diffs
	from := ""
	if f.From != nil {
		from = f.From.UTC().Format(time.RFC3339)
	}
	to := ""
	if f.To != nil {
		to = f.To.UTC().Format(time.RFC3339)
	}

	raw := fmt.Sprintf("city=%s|cat=%s|q=%s|sort=%s|ps=%d|from=%s|to=%s",
		f.City, f.Category, f.Query, f.Sort, f.PageSize, from, to)

	hash := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("events:public:list:%s", hex.EncodeToString(hash[:]))
}
