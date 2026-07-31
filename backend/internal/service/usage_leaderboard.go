package service

import (
	"context"
	"strconv"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
)

const (
	leaderboardCacheTTL = 60 * time.Second
	leaderboardMaxTop   = 20
)

// UserLeaderboardRow is the raw DB row returned by the repo layer.
// The handler is responsible for desensitizing DisplayName before sending to clients.
type UserLeaderboardRow struct {
	Rank                int64
	UserID              int64
	Username            string
	Email               string
	InputTokens         int64
	OutputTokens        int64
	CacheCreationTokens int64
	CacheReadTokens     int64
	ImageOutputTokens   int64
	TotalTokens         int64
}

// leaderboardRepository is an optional duck-typed extension of UsageLogRepository.
// Only the concrete repo that implements GetUserLeaderboard will satisfy it.
type leaderboardRepository interface {
	GetUserLeaderboard(ctx context.Context, startTime, endTime time.Time, limit int, currentUserID int64) ([]UserLeaderboardRow, error)
}

// leaderboardCacheEntry holds a cached leaderboard result.
type leaderboardCacheEntry struct {
	rows      []UserLeaderboardRow
	expiresAt time.Time
}

// leaderboardCache is a simple in-memory per-period cache (no Redis required).
type leaderboardCache struct {
	mu      sync.Mutex
	entries map[string]*leaderboardCacheEntry
}

func newLeaderboardCache() *leaderboardCache {
	return &leaderboardCache{entries: make(map[string]*leaderboardCacheEntry)}
}

func (c *leaderboardCache) get(key string, now time.Time) ([]UserLeaderboardRow, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok || now.After(e.expiresAt) {
		return nil, false
	}
	out := make([]UserLeaderboardRow, len(e.rows))
	copy(out, e.rows)
	return out, true
}

func (c *leaderboardCache) set(key string, rows []UserLeaderboardRow, now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	cp := make([]UserLeaderboardRow, len(rows))
	copy(cp, rows)
	c.entries[key] = &leaderboardCacheEntry{rows: cp, expiresAt: now.Add(leaderboardCacheTTL)}
}

// leaderboardCacheKey builds a stable cache key from the query parameters.
func leaderboardCacheKey(startTime, endTime time.Time, limit int) string {
	return startTime.UTC().Format(time.RFC3339) + "|" + endTime.UTC().Format(time.RFC3339) + "|" + strconv.Itoa(limit)
}

// usageLeaderboard is embedded in UsageService to hold leaderboard state.
type usageLeaderboard struct {
	cache *leaderboardCache
}

// GetUserLeaderboard returns ranked leaderboard rows from the repo (or cache).
// Falls back to an empty response when the repo doesn't implement leaderboardRepository.
func (s *UsageService) GetUserLeaderboard(ctx context.Context, startTime, endTime time.Time, limit int, currentUserID int64) (*usagestats.UserLeaderboardResponse, error) {
	if limit <= 0 || limit > leaderboardMaxTop {
		limit = leaderboardMaxTop
	}
	repo, ok := s.usageRepo.(leaderboardRepository)
	if !ok {
		return &usagestats.UserLeaderboardResponse{
			Ranking:     []usagestats.UserLeaderboardItem{},
			GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		}, nil
	}

	cacheKey := leaderboardCacheKey(startTime, endTime, limit)
	if rows, ok := s.lb.cache.get(cacheKey, time.Now()); ok {
		return buildLeaderboardResponse(rows, currentUserID), nil
	}

	rows, err := repo.GetUserLeaderboard(ctx, startTime, endTime, limit, currentUserID)
	if err != nil {
		return nil, err
	}
	s.lb.cache.set(cacheKey, rows, time.Now())
	return buildLeaderboardResponse(rows, currentUserID), nil
}

func buildLeaderboardResponse(rows []UserLeaderboardRow, currentUserID int64) *usagestats.UserLeaderboardResponse {
	resp := &usagestats.UserLeaderboardResponse{
		Ranking:     make([]usagestats.UserLeaderboardItem, 0, len(rows)),
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	}
	for _, r := range rows {
		// Prefer username; fall back to email as the raw display name for masking.
		rawName := r.Username
		if rawName == "" {
			rawName = r.Email
		}
		item := usagestats.UserLeaderboardItem{
			Rank:                r.Rank,
			UserID:              r.UserID,
			RawName:             rawName,
			InputTokens:         r.InputTokens,
			OutputTokens:        r.OutputTokens,
			CacheCreationTokens: r.CacheCreationTokens,
			CacheReadTokens:     r.CacheReadTokens,
			ImageOutputTokens:   r.ImageOutputTokens,
			TotalTokens:         r.TotalTokens,
			IsCurrentUser:       r.UserID == currentUserID,
		}
		// DisplayName desensitization happens in the handler layer.
		resp.Ranking = append(resp.Ranking, item)
	}
	return resp
}
