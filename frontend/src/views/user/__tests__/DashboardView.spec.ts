import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import DashboardView from '../DashboardView.vue'

const { refreshUser, getDashboardStats, getDashboardTrend, getDashboardModels, getByDateRange, getMyPlatformQuotas } = vi.hoisted(() => ({
  refreshUser: vi.fn(),
  getDashboardStats: vi.fn(),
  getDashboardTrend: vi.fn(),
  getDashboardModels: vi.fn(),
  getByDateRange: vi.fn(),
  getMyPlatformQuotas: vi.fn()
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    user: { balance: 0 },
    isSimpleMode: false,
    refreshUser
  })
}))

vi.mock('@/api/usage', () => ({
  usageAPI: {
    getDashboardStats,
    getDashboardTrend,
    getDashboardModels,
    getByDateRange
  }
}))

vi.mock('@/api/user', () => ({ getMyPlatformQuotas }))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key })
  }
})

const dashboardStats = {
  balance: 0,
  api_keys: 0,
  today_requests: 0,
  today_cost: 0,
  today_tokens: 0,
  total_tokens: 0,
  average_duration: 0
}

const mountDashboard = () => mount(DashboardView, {
  global: {
    stubs: {
      AppLayout: { template: '<div><slot /></div>' },
      LoadingSpinner: true,
      Icon: true,
      UserDashboardStats: { template: '<div data-testid="dashboard-stats" />' },
      UserDashboardCharts: true,
      UserDashboardRecentUsage: true,
      UserDashboardQuickActions: true
    }
  }
})

describe('user DashboardView', () => {
  beforeEach(() => {
    refreshUser.mockReset()
    getDashboardStats.mockReset()
    getDashboardTrend.mockReset()
    getDashboardModels.mockReset()
    getByDateRange.mockReset()
    getMyPlatformQuotas.mockReset()

    refreshUser.mockResolvedValue(undefined)
    getDashboardStats.mockResolvedValue(dashboardStats)
    getDashboardTrend.mockResolvedValue({ trend: [] })
    getDashboardModels.mockResolvedValue({ models: [] })
    getByDateRange.mockResolvedValue({ items: [] })
    getMyPlatformQuotas.mockResolvedValue({ platform_quotas: [] })
  })

  it('shows a reload state instead of a blank page when dashboard stats fail', async () => {
    getDashboardStats.mockRejectedValueOnce(new Error('network failed'))

    const wrapper = mountDashboard()
    await flushPromises()

    expect(wrapper.text()).toContain('dashboard.loadFailed')
    expect(wrapper.find('[data-testid="dashboard-stats"]').exists()).toBe(false)

    getDashboardStats.mockResolvedValueOnce(dashboardStats)
    await wrapper.get('button').trigger('click')
    await flushPromises()

    expect(wrapper.find('[data-testid="dashboard-stats"]').exists()).toBe(true)
  })

  it('keeps the dashboard available when the background user refresh fails', async () => {
    refreshUser.mockRejectedValueOnce(new Error('profile refresh failed'))

    const wrapper = mountDashboard()
    await flushPromises()

    expect(wrapper.find('[data-testid="dashboard-stats"]').exists()).toBe(true)
  })
})
