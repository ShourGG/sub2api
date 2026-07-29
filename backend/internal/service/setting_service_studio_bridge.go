package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strconv"
	"strings"
)

const (
	SettingKeyStudioBridgeLuoyeAI = "studio_bridge_luoye_ai"
)

func parseStudioBridgeAppSettings(raw string) *StudioBridgeAppSettings {
	cfg := defaultStudioBridgeAppSettings()
	if strings.TrimSpace(raw) == "" {
		return cfg
	}
	if err := json.Unmarshal([]byte(raw), cfg); err != nil {
		return defaultStudioBridgeAppSettings()
	}
	return normalizeStudioBridgeAppSettingsForSave(*cfg)
}

func normalizeStudioBridgeAppSettingsForSave(cfg StudioBridgeAppSettings) *StudioBridgeAppSettings {
	if strings.TrimSpace(cfg.SiteName) == "" {
		cfg.SiteName = "落叶创艺"
	}
	cfg.SiteName = strings.TrimSpace(cfg.SiteName)
	cfg.LaunchReturnURL = strings.TrimSpace(cfg.LaunchReturnURL)
	if cfg.LaunchReturnURL == "" {
		cfg.LaunchReturnURL = defaultStudioBridgeLaunchReturnURL
	}
	cfg.RechargeReturnURL = strings.TrimSpace(cfg.RechargeReturnURL)
	if cfg.RechargeReturnURL == "" {
		cfg.RechargeReturnURL = defaultStudioBridgeRechargeURL
	}
	cfg.DefaultChatGroup = strings.TrimSpace(cfg.DefaultChatGroup)
	cfg.DefaultImageGroup = strings.TrimSpace(cfg.DefaultImageGroup)
	cfg.DefaultVideoGroup = strings.TrimSpace(cfg.DefaultVideoGroup)
	cfg.DefaultFallbackGroup = strings.TrimSpace(cfg.DefaultFallbackGroup)
	cfg.DefaultAPIRoutes = normalizeStudioBridgeDefaultAPIRoutes(cfg.DefaultAPIRoutes)
	if len(cfg.DefaultAPIRoutes) == 0 {
		cfg.DefaultAPIRoutes = legacyStudioBridgeDefaultAPIRoutes(cfg)
	}
	cfg.InternalSecret = strings.TrimSpace(cfg.InternalSecret)
	cfg.AllowedReturnDomains = normalizeStudioBridgeStringSlice(cfg.AllowedReturnDomains)
	return &cfg
}

func validateStudioBridgeAppSettings(cfg StudioBridgeAppSettings) error {
	if !cfg.Enabled {
		return nil
	}
	routes := normalizeStudioBridgeDefaultAPIRoutes(cfg.DefaultAPIRoutes)
	if len(routes) == 0 {
		routes = legacyStudioBridgeDefaultAPIRoutes(cfg)
	}
	if len(routes) == 0 {
		return ErrStudioBridgeGroupRequired
	}
	return nil
}

func (s *SettingService) validateDefaultKeyFallbackGroup(ctx context.Context, cfg StudioBridgeAppSettings) error {
	raw := strings.TrimSpace(cfg.DefaultFallbackGroup)
	if raw == "" {
		return nil
	}
	groupID := parseStudioBridgeDefaultGroupID(raw)
	if groupID <= 0 || s == nil || s.studioBridgeDefaultGroupReader == nil {
		return ErrDefaultKeyFallbackGroupInvalid
	}
	group, err := s.studioBridgeDefaultGroupReader.GetByID(ctx, groupID)
	if err != nil || group == nil || !group.IsActive() {
		return ErrDefaultKeyFallbackGroupInvalid
	}
	return nil
}

func normalizeStudioBridgeDefaultAPIRoutes(routes []StudioBridgeDefaultAPIRoute) []StudioBridgeDefaultAPIRoute {
	if len(routes) == 0 {
		return []StudioBridgeDefaultAPIRoute{}
	}
	normalized := make([]StudioBridgeDefaultAPIRoute, 0, len(routes))
	seen := make(map[string]struct{}, len(routes))
	for _, route := range routes {
		route.GroupID = strings.TrimSpace(route.GroupID)
		if route.GroupID == "" {
			continue
		}
		route.ModelPatterns = normalizeStudioBridgeModelPatterns(route.ModelPatterns)
		key := studioBridgeDefaultAPIRouteDedupKey(route)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		if route.Priority <= 0 {
			route.Priority = 1
		}
		if route.Weight <= 0 {
			route.Weight = 1
		}
		if route.CooldownSeconds <= 0 {
			route.CooldownSeconds = apiKeyRouteDefaultCooldown
		}
		normalized = append(normalized, route)
	}
	return normalized
}

func studioBridgeDefaultAPIRouteDedupKey(route StudioBridgeDefaultAPIRoute) string {
	patterns := make([]string, 0, len(route.ModelPatterns))
	for _, pattern := range route.ModelPatterns {
		if pattern = strings.TrimSpace(pattern); pattern != "" {
			patterns = append(patterns, strings.ToLower(pattern))
		}
	}
	return fmt.Sprintf("%s|%t|%t|%s", route.GroupID, route.ImageOnly, route.TextOnly, strings.Join(patterns, "\n"))
}

func normalizeStudioBridgeModelPatterns(patterns []string) []string {
	normalized := normalizeStudioBridgeStringSlice(patterns)
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

func legacyStudioBridgeDefaultAPIRoutes(cfg StudioBridgeAppSettings) []StudioBridgeDefaultAPIRoute {
	routes := make([]StudioBridgeDefaultAPIRoute, 0, 3)
	if groupID := strings.TrimSpace(cfg.DefaultChatGroup); groupID != "" {
		routes = append(routes, studioBridgeDefaultAPIRoute(groupID, true, false, nil))
	}
	if groupID := strings.TrimSpace(cfg.DefaultImageGroup); groupID != "" {
		routes = append(routes, studioBridgeDefaultAPIRoute(groupID, false, true, nil))
	}
	if groupID := strings.TrimSpace(cfg.DefaultVideoGroup); groupID != "" {
		routes = append(routes, studioBridgeDefaultAPIRoute(groupID, false, false, []string{"doubao-seedance-*", "*-video-*"}))
	}
	return routes
}

func studioBridgeDefaultAPIRoute(groupID string, textOnly, imageOnly bool, modelPatterns []string) StudioBridgeDefaultAPIRoute {
	return StudioBridgeDefaultAPIRoute{
		GroupID:         groupID,
		Priority:        1,
		Weight:          apiKeyRouteDefaultWeight,
		CooldownSeconds: apiKeyRouteDefaultCooldown,
		Enabled:         true,
		ModelPatterns:   modelPatterns,
		ImageOnly:       imageOnly,
		TextOnly:        textOnly,
	}
}

func normalizeStudioBridgeStringSlice(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	normalized := make([]string, 0, len(values))
	for _, v := range values {
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			continue
		}
		normalized = append(normalized, trimmed)
	}
	return normalized
}

func marshalStudioBridgeAppSettings(cfg StudioBridgeAppSettings) (string, error) {
	normalized := normalizeStudioBridgeAppSettingsForSave(cfg)
	raw, err := json.Marshal(normalized)
	if err != nil {
		return "", fmt.Errorf("marshal studio bridge settings: %w", err)
	}
	return string(raw), nil
}

func marshalStudioBridgeAppSettingsOrDefault(cfg *StudioBridgeAppSettings) string {
	if cfg == nil {
		cfg = defaultStudioBridgeAppSettings()
	}
	raw, err := marshalStudioBridgeAppSettings(*cfg)
	if err != nil {
		return "{}"
	}
	return raw
}

func defaultStudioBridgeSettingsFromEnv() *StudioBridgeAppSettings {
	cfg := defaultStudioBridgeAppSettings()
	cfg.InternalSecret = strings.TrimSpace(os.Getenv("STUDIO_BRIDGE_LUOYE_AI_INTERNAL_SECRET"))
	return normalizeStudioBridgeAppSettingsForSave(*cfg)
}

func studioBridgeEnvSecret() string {
	return strings.TrimSpace(os.Getenv("STUDIO_BRIDGE_LUOYE_AI_INTERNAL_SECRET"))
}

func (s *SettingService) localStudioBridgeDefaults(ctx context.Context, base *StudioBridgeAppSettings) *StudioBridgeAppSettings {
	cfg := defaultStudioBridgeAppSettings()
	if base != nil {
		cfg.SiteName = firstNonEmpty(strings.TrimSpace(base.SiteName), cfg.SiteName)
		cfg.DefaultVideoGroup = strings.TrimSpace(base.DefaultVideoGroup)
	}
	cfg.InternalSecret = studioBridgeEnvSecret()
	cfg.LaunchReturnURL = defaultStudioBridgeLaunchReturnURL
	cfg.RechargeReturnURL = defaultStudioBridgeRechargeURL
	cfg.AllowedReturnDomains = []string{"127.0.0.1", "localhost"}
	if imageGroup, textGroup := s.defaultStudioBridgeGroups(ctx); imageGroup != "" {
		cfg.Enabled = true
		cfg.DefaultImageGroup = imageGroup
		cfg.DefaultChatGroup = firstNonEmpty(textGroup, imageGroup)
		cfg.DefaultAPIRoutes = legacyStudioBridgeDefaultAPIRoutes(*cfg)
	} else if base != nil {
		cfg.Enabled = base.Enabled && len(normalizeStudioBridgeDefaultAPIRoutes(base.DefaultAPIRoutes)) > 0
		cfg.DefaultImageGroup = strings.TrimSpace(base.DefaultImageGroup)
		cfg.DefaultChatGroup = strings.TrimSpace(base.DefaultChatGroup)
		cfg.DefaultAPIRoutes = normalizeStudioBridgeDefaultAPIRoutes(base.DefaultAPIRoutes)
	}
	return normalizeStudioBridgeAppSettingsForSave(*cfg)
}

func (s *SettingService) defaultStudioBridgeGroups(ctx context.Context) (imageGroup string, textGroup string) {
	if s == nil || s.studioBridgeDefaultGroupReader == nil {
		return "", ""
	}
	groups, err := s.studioBridgeDefaultGroupReader.ListActive(ctx)
	if err != nil {
		slog.Warn("failed to list active groups for studio bridge local defaults", "error", err)
		return "", ""
	}
	sort.SliceStable(groups, func(i, j int) bool {
		if groups[i].SortOrder != groups[j].SortOrder {
			return groups[i].SortOrder < groups[j].SortOrder
		}
		return groups[i].ID < groups[j].ID
	})
	for i := range groups {
		group := groups[i]
		if !group.IsActive() {
			continue
		}
		switch group.EffectiveRoutingScope() {
		case GroupRoutingScopeImage:
			if imageGroup == "" && group.AllowImageGeneration {
				imageGroup = strconv.FormatInt(group.ID, 10)
			}
		case GroupRoutingScopeInference:
			if textGroup == "" {
				textGroup = strconv.FormatInt(group.ID, 10)
			}
		}
	}
	return imageGroup, textGroup
}

func studioBridgeSettingsNeedLocalRepair(raw string, cfg *StudioBridgeAppSettings) bool {
	if studioBridgeEnvSecret() == "" {
		return false
	}
	if strings.TrimSpace(raw) == "" || cfg == nil {
		return true
	}
	if !cfg.Enabled {
		return true
	}
	if strings.TrimSpace(cfg.InternalSecret) == "" {
		return true
	}
	if len(normalizeStudioBridgeDefaultAPIRoutes(cfg.DefaultAPIRoutes)) == 0 {
		return true
	}
	return studioBridgeHasPlaceholderURL(cfg.LaunchReturnURL) ||
		studioBridgeHasPlaceholderURL(cfg.RechargeReturnURL) ||
		studioBridgeDomainsArePlaceholder(cfg.AllowedReturnDomains)
}

func studioBridgeHasPlaceholderURL(value string) bool {
	return strings.Contains(strings.ToLower(strings.TrimSpace(value)), "example.com")
}

func studioBridgeDomainsArePlaceholder(values []string) bool {
	if len(values) == 0 {
		return true
	}
	for _, value := range values {
		if strings.Contains(strings.ToLower(strings.TrimSpace(value)), "example.com") {
			return true
		}
	}
	return false
}

// SetStudioBridgeDefaultGroupReader injects a group reader for local studio bridge defaults.
func (s *SettingService) SetStudioBridgeDefaultGroupReader(reader StudioBridgeGroupReader) {
	s.studioBridgeDefaultGroupReader = reader
}

func (s *SettingService) GetStudioBridgeLuoyeAISettings(ctx context.Context) (*StudioBridgeAppSettings, error) {
	raw, err := s.settingRepo.GetValue(ctx, SettingKeyStudioBridgeLuoyeAI)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			return defaultStudioBridgeAppSettings(), nil
		}
		return nil, fmt.Errorf("get studio bridge settings: %w", err)
	}
	return parseStudioBridgeAppSettings(raw), nil
}

func (s *SettingService) repairLocalStudioBridgeDefaults(ctx context.Context) error {
	raw, err := s.settingRepo.GetValue(ctx, SettingKeyStudioBridgeLuoyeAI)
	if err != nil && !errors.Is(err, ErrSettingNotFound) {
		return fmt.Errorf("check studio bridge settings: %w", err)
	}
	if errors.Is(err, ErrSettingNotFound) {
		raw = ""
	}
	current := parseStudioBridgeAppSettings(raw)
	if !studioBridgeSettingsNeedLocalRepair(raw, current) {
		return nil
	}
	updated := s.localStudioBridgeDefaults(ctx, current)
	value, err := marshalStudioBridgeAppSettings(*updated)
	if err != nil {
		return err
	}
	if err := s.settingRepo.Set(ctx, SettingKeyStudioBridgeLuoyeAI, value); err != nil {
		return fmt.Errorf("repair studio bridge local defaults: %w", err)
	}
	return nil
}
