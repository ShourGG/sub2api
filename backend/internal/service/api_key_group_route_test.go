package service

import (
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
)

func TestNormalizeAPIKeyGroupRoutes(t *testing.T) {
	routes, err := normalizeAPIKeyGroupRoutes([]domain.APIKeyGroupRoute{
		{GroupID: 2, Priority: 2, Weight: 1, Enabled: true},
		{GroupID: 1, Priority: 1, Weight: 1, Enabled: true},
	})
	if err != nil || routes[0].GroupID != 1 {
		t.Fatalf("normalize routes = %#v, %v", routes, err)
	}
	_, err = normalizeAPIKeyGroupRoutes([]domain.APIKeyGroupRoute{
		{GroupID: 1, Priority: 1, Weight: 1},
		{GroupID: 1, Priority: 2, Weight: 1},
	})
	if err == nil {
		t.Fatal("expected duplicate group validation error")
	}
}

func TestAPIKeyServiceSelectedGroupRoute(t *testing.T) {
	svc := &APIKeyService{}
	key := &APIKey{Key: "key", GroupRoutes: []domain.APIKeyGroupRoute{
		{GroupID: 9, Priority: 2, Weight: 1, Enabled: true},
		{GroupID: 1, Priority: 1, Weight: 1, Enabled: true},
		{GroupID: 2, Priority: 1, Weight: 2, Enabled: true},
	}}
	for range 3 {
		route := svc.selectedAPIKeyGroupRoute(key)
		if route == nil || route.Priority != 1 {
			t.Fatalf("selected route = %#v", route)
		}
	}
}

func TestAPIKeyServiceSelectedGroupRouteSkipsCooldownAndAttempts(t *testing.T) {
	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	svc := &APIKeyService{routeNow: func() time.Time { return now }}
	key := &APIKey{ID: 7, GroupRoutes: []domain.APIKeyGroupRoute{
		{GroupID: 1, Priority: 1, Weight: 1, Enabled: true, CooldownSeconds: 60},
		{GroupID: 2, Priority: 1, Weight: 1, Enabled: true},
		{GroupID: 3, Priority: 2, Weight: 1, Enabled: true},
	}}

	svc.MarkGroupRouteFailed(key, 1)
	route := svc.selectedAPIKeyGroupRoute(key)
	if route == nil || route.GroupID != 2 {
		t.Fatalf("route during cooldown = %#v", route)
	}

	route = svc.selectedAPIKeyGroupRouteExcluding(key, map[int64]struct{}{2: {}})
	if route == nil || route.GroupID != 3 {
		t.Fatalf("route after excluding same priority = %#v", route)
	}

	now = now.Add(61 * time.Second)
	route = svc.selectedAPIKeyGroupRoute(key)
	if route == nil || route.Priority != 1 {
		t.Fatalf("route after cooldown expiry = %#v", route)
	}
}
