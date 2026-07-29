package service

// imagegen_compat.go — 生图功能移植兼容层
//
// zz fork (ref-imagegen) 的路由模型与当前分支不同:
//   zz: multi_group_routes + routing_scope 多租户路由
//   当前: group_routes 优先级/权重路由
// 本文件为编译兼容提供最小 shim 实现。
// 实际路由行为由 APIKeyService.ApplySelectedGroupRoute 控制。

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

// ============================================================================
// MembershipService stub
// ============================================================================

// MembershipService 是会员等级服务的占位类型。
// 完整会员系统尚未移植。将 nil 传给 NewImageCreatorService；服务已做 nil 守卫。
type MembershipService struct{}

// ImageActiveTaskLimit 返回用户最大并发生图任务数。
// stub 固定返回 2（默认上限）。
func (m *MembershipService) ImageActiveTaskLimit(_ context.Context, _ int64) int {
	return 2
}

// ============================================================================
// Group 路由 scope 常量与 shim
// ============================================================================

const (
	// GroupRoutingScopeInference 推理类请求的路由 scope。
	GroupRoutingScopeInference = "inference"
	// GroupRoutingScopeImage 生图类请求的路由 scope。
	GroupRoutingScopeImage = "image"
	// GroupRoutingScopeVideo 视频类请求的路由 scope。
	GroupRoutingScopeVideo = "video"
)

// EffectiveRoutingScope 根据 Group 的能力配置返回兼容 scope 字符串。
// 当前分支数据库无 routing_scope 字段，从 platform + 能力标志推断。
func (g *Group) EffectiveRoutingScope() string {
	if g == nil {
		return GroupRoutingScopeInference
	}
	if g.AllowImageGeneration && g.Platform == PlatformOpenAI {
		return GroupRoutingScopeImage
	}
	if g.VideoRateIndependent || g.VideoPrice480P != nil {
		return GroupRoutingScopeVideo
	}
	return GroupRoutingScopeInference
}

// ============================================================================
// APIKey 路由 shim
// ============================================================================

// ResolveForModelRequest 返回 key 本身。
// 当前分支路由由 APIKeyService.ApplySelectedGroupRoute 在网关层处理，
// APIKey 结构体无需自行执行多路由解析。
func (k *APIKey) ResolveForModelRequest(path, forcePlatform, requestedModel string, imageIntent bool) *APIKey {
	if k == nil {
		return nil
	}
	return k
}

// ============================================================================
// APIKeyService 路由与 key 加载 shim
// ============================================================================

// ResolveForRequest 返回 key 本身，提供编译兼容。
func (s *APIKeyService) ResolveForRequest(_ context.Context, apiKey *APIKey, _, _ string) *APIKey {
	if apiKey == nil {
		return nil
	}
	return apiKey
}

// ResolveForModelRequest 是 ResolveForRequest 的模型感知变体。
func (s *APIKeyService) ResolveForModelRequest(ctx context.Context, apiKey *APIKey, path, forcePlatform, _ string, _ bool) *APIKey {
	return s.ResolveForRequest(ctx, apiKey, path, forcePlatform)
}

// loadDefaultAPIKey 返回用户的第一个活跃 API Key。
// 对应 zz fork 的同名私有方法，供 StudioBridgeService 使用。
func (s *APIKeyService) loadDefaultAPIKey(ctx context.Context, userID int64) (*APIKey, error) {
	if s == nil {
		return nil, fmt.Errorf("api key service is unavailable")
	}
	keys, _, err := s.List(ctx, userID, pagination.PaginationParams{Page: 1, PageSize: 20}, APIKeyListFilters{})
	if err != nil {
		return nil, fmt.Errorf("load default api key: %w", err)
	}
	for i := range keys {
		if keys[i].IsActive() {
			return &keys[i], nil
		}
	}
	return nil, infraerrors.NotFound("DEFAULT_API_KEY_NOT_FOUND", "no active api key found for user")
}

// ============================================================================
// Studio Bridge — api_key 相关常量与工具
// ============================================================================

const (
	// apiKeyRouteDefaultWeight 新路由条目的默认权重。
	apiKeyRouteDefaultWeight = 1
	// apiKeyRouteDefaultCooldown 失败路由的默认冷却秒数。
	apiKeyRouteDefaultCooldown = 30
	// DefaultAPIKeyName 默认 API Key 的名称（由 StudioBridge 等功能引用）。
	DefaultAPIKeyName = "默认 API Key（勿删）"
)

var (
	// ErrDefaultKeyFallbackGroupInvalid 默认 Key 兜底分组不存在或未启用时返回此错误。
	ErrDefaultKeyFallbackGroupInvalid = infraerrors.BadRequest(
		"DEFAULT_KEY_FALLBACK_GROUP_INVALID", "默认 Key 兜底分组不存在或未启用",
	)
)

// parseStudioBridgeDefaultGroupID 从设置字符串中解析整数分组 ID。
// 空字符串或非法格式返回 0。
func parseStudioBridgeDefaultGroupID(raw string) int64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0
	}
	return id
}

// ============================================================================
// IsVideoGenerationIntent stub
// ============================================================================

// IsVideoGenerationIntent 报告请求是否针对视频生成。
// stub 实现，供 model_catalog.go 编译兼容使用。
func IsVideoGenerationIntent(_ string, _ string, _ interface{}) bool {
	return false
}

// openAIBaseURLHost 从 OpenAI base URL 中提取 hostname。
// 供 apimart_gpt_image2_pricing.go 使用。
func openAIBaseURLHost(rawBaseURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawBaseURL))
	if err != nil {
		return ""
	}
	return parsed.Hostname()
}

// ============================================================================
// StudioBridgeGroupReader interface
// ============================================================================

// StudioBridgeGroupReader 可按 ID 读取 Group，供 SettingService 验证兜底分组使用。
type StudioBridgeGroupReader interface {
	GetByID(ctx context.Context, id int64) (*Group, error)
	ListActive(ctx context.Context) ([]*Group, error)
}
