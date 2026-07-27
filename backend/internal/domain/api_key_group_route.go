package domain

// APIKeyGroupRoute configures a candidate group for one API key.
// Lower priority values win; equal-priority routes use Weight.
type APIKeyGroupRoute struct {
	GroupID         int64 `json:"group_id"`
	Priority        int   `json:"priority"`
	Weight          int   `json:"weight"`
	Enabled         bool  `json:"enabled"`
	CooldownSeconds int   `json:"cooldown_seconds"`
}
