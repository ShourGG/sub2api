import type { ApiKey, Group } from '@/types'

// 说明：本文件从生图分支移植而来，但目标分支使用的是 group_routes（优先级/权重路由）
// 路由模型，而非上游的 multi_group_routes / routing_scope 模型。这里已适配到当前分支
// 的类型：能力判定基于每个候选 Group 的 platform 与 allow_image_generation 等既有字段。

export interface ApiKeyUnifiedAccessCapabilities {
  chat: boolean
  image: boolean
  video: boolean
}

const CHAT_PLATFORMS = new Set(['openai', 'anthropic', 'gemini', 'antigravity'])

function isActiveGroup(group: Group | undefined | null): group is Group {
  return !!group && group.status !== 'inactive'
}

function groupSupportsChat(group: Group | undefined | null): group is Group {
  return isActiveGroup(group) && CHAT_PLATFORMS.has(group.platform)
}

function groupSupportsOpenAIImage(group: Group | undefined | null): group is Group {
  return isActiveGroup(group) && group.platform === 'openai' && group.allow_image_generation === true
}

function groupSupportsVideo(group: Group | undefined | null): group is Group {
  // 当前分支的 Group 没有 routing_scope，用视频计费配置作为能力信号：
  // 独立视频计费或配置了任一分辨率单价，即视为支持视频生成。
  if (!isActiveGroup(group)) return false
  return (
    group.video_rate_independent === true ||
    group.video_price_480p != null ||
    group.video_price_720p != null ||
    group.video_price_1080p != null
  )
}

// 收集一个 API Key 可路由到的全部活跃分组：主分组 + group_routes 中已启用的分组。
export function apiKeyGroups(key: ApiKey): Group[] {
  const groups: Group[] = []
  const seen = new Set<number>()
  const append = (group: Group | undefined | null) => {
    if (!isActiveGroup(group) || seen.has(group.id)) return
    seen.add(group.id)
    groups.push(group)
  }

  append(key.group)
  // group_routes 只携带 group_id/priority/weight/enabled，不含分组详情，
  // 因此仅在其引用的分组正好是主分组时可判定；其余分组详情由后端在 key.group 之外
  // 不下发。这里保留主分组作为唯一可解析的能力来源，避免误判。
  return groups
}

export function apiKeyChatGroups(key: ApiKey): Group[] {
  return apiKeyGroups(key).filter(groupSupportsChat)
}

export function apiKeySupportsChat(key: ApiKey): boolean {
  return key.status === 'active' && apiKeyChatGroups(key).length > 0
}

export function apiKeySupportsOpenAI(key: ApiKey): boolean {
  return key.status === 'active' && apiKeyGroups(key).some((group) => group.platform === 'openai')
}

export function apiKeyOpenAIImageGroups(key: ApiKey): Group[] {
  return apiKeyGroups(key).filter(groupSupportsOpenAIImage)
}

export function apiKeySupportsOpenAIImageGeneration(key: ApiKey): boolean {
  return key.status === 'active' && apiKeyOpenAIImageGroups(key).length > 0
}

export function apiKeyVideoGroups(key: ApiKey): Group[] {
  return apiKeyGroups(key).filter(groupSupportsVideo)
}

export function apiKeySupportsVideoGeneration(key: ApiKey): boolean {
  return key.status === 'active' && apiKeyVideoGroups(key).length > 0
}

export function apiKeyUnifiedAccessCapabilities(key: ApiKey): ApiKeyUnifiedAccessCapabilities {
  return {
    chat: apiKeySupportsChat(key),
    image: apiKeySupportsOpenAIImageGeneration(key),
    video: apiKeySupportsVideoGeneration(key),
  }
}

export function apiKeySupportsUnifiedAccess(key: ApiKey): boolean {
  const capabilities = apiKeyUnifiedAccessCapabilities(key)
  return capabilities.chat && capabilities.image
}

export function primaryAPIKeyGroupName(key: ApiKey): string {
  return key.group?.name || apiKeyGroups(key)[0]?.name || ''
}

export function primaryAPIKeyImageGroupName(key: ApiKey): string {
  return apiKeyOpenAIImageGroups(key)[0]?.name || primaryAPIKeyGroupName(key)
}
