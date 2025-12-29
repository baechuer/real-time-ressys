package domain

// Waitlist policy (Option B):
// Derive waitlist_max from capacity, without storing per-event config.
//
// Semantics:
// - capacity <= 0 (unlimited or closed sentinel handled elsewhere): return 0 (means "no waitlist cap from policy")
// - otherwise: waitlist_max = min(absCap, max(minCap, capacity*mult))
//
// Default parameters chosen to match your desire:
// - hard cap: 100
// - small events still allow some waitlist: at least 20
// - multiplier: 1x capacity
func WaitlistMax(capacity int) int {
	// For unlimited (0) or invalid/closed (<0), the repo should short-circuit earlier.
	if capacity <= 0 {
		return 0
	}

	const (
		absCap = 100
		minCap = 20
		mult   = 1
	)

	max := capacity * mult
	if max < minCap {
		max = minCap
	}
	if max > absCap {
		max = absCap
	}
	return max
}
