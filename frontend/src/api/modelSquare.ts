import { apiClient } from './client'
import type { UserAvailableGroup, UserSupportedModelPricing } from './channels'

export interface ModelSquareEntry {
  name: string
  platform: string
  channel_id: number
  channel_name: string
  group: UserAvailableGroup
  account_count: number
  pricing: UserSupportedModelPricing | null
}

export async function listModelSquare(options?: { signal?: AbortSignal }): Promise<ModelSquareEntry[]> {
  const { data } = await apiClient.get<ModelSquareEntry[]>('/model-square', {
    signal: options?.signal
  })
  return data
}

export const modelSquareAPI = { list: listModelSquare }

export default modelSquareAPI
