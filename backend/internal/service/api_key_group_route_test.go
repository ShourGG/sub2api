package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/stretchr/testify/require"
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

func TestAPIKeyServiceApplySelectedGroupRoutePreservesPrimaryGroup(t *testing.T) {
	primaryID := int64(75)
	primary := &Group{ID: primaryID, Name: "primary", Status: StatusActive}
	key := &APIKey{
		GroupID: &primaryID,
		Group:   primary,
		GroupRoutes: []domain.APIKeyGroupRoute{{
			GroupID:  95,
			Priority: 1,
			Weight:   1,
			Enabled:  true,
		}},
	}

	require.NoError(t, (&APIKeyService{}).ApplySelectedGroupRoute(context.Background(), key))
	if key.GroupID == nil || *key.GroupID != primaryID {
		t.Fatalf("primary group was replaced: %#v", key.GroupID)
	}
	if key.Group != primary {
		t.Fatalf("primary group object was replaced: %#v", key.Group)
	}
}

func TestAPIKeyServiceUpdateRoutesPreservesPrimaryGroup(t *testing.T) {
	primaryID := int64(75)
	routes := []domain.APIKeyGroupRoute{{GroupID: 95, Priority: 1, Weight: 1, Enabled: true}}
	primary := &Group{ID: primaryID, Name: "primary", Status: StatusActive}
	repo := &apiKeyRepoStubForGroupUpdate{key: &APIKey{
		ID:      1,
		UserID:  7,
		Key:     "sk-test",
		Status:  StatusActive,
		GroupID: &primaryID,
		Group:   primary,
	}}
	svc := &APIKeyService{
		apiKeyRepo: repo,
		userRepo: &userRepoStubForGroupUpdate{user: &User{
			ID:            7,
			AllowedGroups: []int64{95},
		}},
		groupRepo: &groupRepoStubForGroupUpdate{group: &Group{ID: 95, Status: StatusActive}},
	}

	updated, err := svc.Update(context.Background(), 1, 7, UpdateAPIKeyRequest{GroupRoutes: &routes})
	require.NoError(t, err)
	require.NotNil(t, updated.GroupID)
	require.Equal(t, primaryID, *updated.GroupID)
	require.NotNil(t, repo.updated)
	require.NotNil(t, repo.updated.GroupID)
	require.Equal(t, primaryID, *repo.updated.GroupID)
}
