/**
 * Stub: apiKeyCapabilities — no-op helpers matching all signatures used by
 * the ported image-creator / canvas / chat-studio views.
 */
import type { ApiKey } from '@/types'

export interface APIKeyGroup {
  id: number
  name: string
  platform: string
  allowImageGeneration: boolean
}

export function apiKeySupportsOpenAI(_key: ApiKey | null | undefined): boolean {
  return true
}
export function apiKeySupportsOpenAIImageGeneration(_key: ApiKey | null | undefined): boolean {
  return true
}
export function apiKeySupportsChat(_key: ApiKey | null | undefined): boolean {
  return true
}
export function apiKeyChatGroups(_key: ApiKey | null | undefined): APIKeyGroup[] {
  return []
}
export function primaryAPIKeyImageGroupName(_key: ApiKey | null | undefined): string {
  return ''
}
export function primaryAPIKeyGroupName(_key: ApiKey | null | undefined): string {
  return ''
}
