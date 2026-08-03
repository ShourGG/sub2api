import { describe, expect, it } from 'vitest'
import { syncPrimaryGroupRoute } from '@/utils/apiKeyGroupRoutes'
import type { ApiKeyGroupRoute } from '@/types'

const route = (group_id: number, priority: number, weight = 1): ApiKeyGroupRoute => ({
  group_id,
  priority,
  weight,
  enabled: true,
  cooldown_seconds: 0
})

describe('syncPrimaryGroupRoute', () => {
  it('updates the first enabled route when the selected group is new', () => {
    const routes = [route(10, 1), route(20, 2)]

    expect(syncPrimaryGroupRoute(30, routes)).toEqual([route(30, 1), route(20, 2)])
    expect(routes).toEqual([route(10, 1), route(20, 2)])
  })

  it('promotes an existing route without duplicating the group', () => {
    const routes = [route(10, 1, 2), route(20, 2, 3), route(30, 3, 4)]

    expect(syncPrimaryGroupRoute(20, routes)).toEqual([
      route(20, 1, 3),
      route(10, 2, 2),
      route(30, 3, 4)
    ])
  })

  it('ignores disabled routes when selecting the primary route', () => {
    const routes = [
      { ...route(10, 1), enabled: false },
      route(20, 2)
    ]

    expect(syncPrimaryGroupRoute(30, routes)).toEqual([
      { ...route(10, 1), enabled: false },
      route(30, 2)
    ])
  })

  it('enables a disabled target route when promoting it to primary', () => {
    const routes = [
      route(10, 1),
      { ...route(20, 2), enabled: false }
    ]

    expect(syncPrimaryGroupRoute(20, routes)).toEqual([
      { ...route(20, 1), enabled: true },
      { ...route(10, 2), enabled: true }
    ])
  })
})
