<template>
  <AppLayout>
    <div class="admin-leaderboard space-y-6" :data-theme="theme">
      <!-- Real backend search filters -->
      <UsageFilters
        v-model="filters"
        flat
        mode="ranking"
        :start-date="startDate"
        :end-date="endDate"
        :model-options="modelOptions"
        :model-creatable="true"
        :exporting="false"
        show-actions
        @change="applyFilters"
        @refresh="rankingRef?.reload"
        @reset="resetFilters"
      />

      <!-- Ranking table -->
      <div class="card overflow-hidden">
        <div class="admin-lb-controls">
          <div class="flex flex-wrap items-end gap-3">
            <div class="w-40">
              <label class="input-label">{{ t('leaderboard.periodLabel') }}</label>
              <Select v-model="days" :options="periodOptions" @change="onDaysChange" />
            </div>
            <div class="w-28">
              <label class="input-label">{{ t('leaderboard.limit') }}</label>
              <Select v-model="limit" :options="limitOptions" @change="onLimitChange" />
            </div>
          </div>
          <div class="flex flex-wrap items-center gap-3">
            <button type="button" class="btn btn-secondary" @click="rankingRef?.reload">
              {{ t('leaderboard.refresh') }}
            </button>
            <button type="button" class="btn btn-secondary" @click="resetFilters">
              {{ t('leaderboard.reset') }}
            </button>
            <button
              type="button"
              class="admin-lb-theme-button"
              :title="themeToggleLabel"
              @click="toggleTheme"
            >
              {{ theme === 'dark' ? '☀️' : '🌙' }}
            </button>
          </div>
        </div>
        <UserTokenRanking
          ref="rankingRef"
          :start-date="startDate"
          :end-date="endDate"
          :filters="breakdownFilters"
          :model="filters.model"
          :limit="limit"
          :limit-options="limitOptions"
          :show-toolbar="false"
          @select-user="handleSelectUser"
        />
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import UsageFilters from '@/components/admin/usage/UsageFilters.vue'
import UserTokenRanking from '@/components/admin/usage/UserTokenRanking.vue'
import Select from '@/components/common/Select.vue'
import { adminAPI } from '@/api/admin'
import type { ModelStat } from '@/types'

const { t } = useI18n()
const router = useRouter()

type DaysWindow = 1 | 3 | 7 | 14 | 30
type LeaderboardTheme = 'light' | 'dark'

const THEME_STORAGE_KEY = 'admin-leaderboard.theme'

const days = ref<DaysWindow>(7)
const limit = ref(20)
const filters = ref<Record<string, any>>({})
const rankingRef = ref<InstanceType<typeof UserTokenRanking> | null>(null)
const requestedModelStats = ref<ModelStat[]>([])
const modelStatsLoading = ref(false)
const theme = ref<LeaderboardTheme>('light')

const periodOptions = computed(() => [
  { value: 1, label: t('leaderboard.period.day1') },
  { value: 3, label: t('leaderboard.period.day3') },
  { value: 7, label: t('leaderboard.period.day7') },
  { value: 14, label: t('leaderboard.period.day14') },
  { value: 30, label: t('leaderboard.period.day30') },
])

const limitOptions = computed(() => [
  { value: 10, label: 'Top 10' },
  { value: 20, label: 'Top 20' },
  { value: 50, label: 'Top 50' },
  { value: 100, label: 'Top 100' },
])

const formatLocalDate = (d: Date) => {
  const year = d.getFullYear()
  const month = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

const computeRange = (d: DaysWindow) => {
  const end = new Date()
  const start = new Date(end.getTime())
  start.setDate(end.getDate() - (d - 1))
  return { start: formatLocalDate(start), end: formatLocalDate(end) }
}

const range = computed(() => computeRange(days.value))
const startDate = computed(() => range.value.start)
const endDate = computed(() => range.value.end)

const modelOptions = computed(() =>
  Array.from(new Set(requestedModelStats.value.map((m) => m.model).filter(Boolean))).sort()
)

const themeToggleLabel = computed(() =>
  theme.value === 'dark' ? t('leaderboard.theme.light') : t('leaderboard.theme.dark')
)

// Exclude date fields because UserTokenRanking receives them separately.
const breakdownFilters = computed(() => {
  const f: Record<string, any> = {}
  const keys = ['user_id', 'api_key_id', 'account_id', 'group_id', 'request_type', 'billing_type', 'model']
  for (const k of keys) {
    if (filters.value[k] != null) f[k] = filters.value[k]
  }
  return f
})

const loadModelStats = async () => {
  modelStatsLoading.value = true
  try {
    const res = await adminAPI.dashboard.getModelStats({
      start_date: startDate.value,
      end_date: endDate.value,
      model_source: 'requested',
    })
    requestedModelStats.value = res.models || []
  } catch {
    requestedModelStats.value = []
  } finally {
    modelStatsLoading.value = false
  }
}

const onDaysChange = () => {
  loadModelStats()
}

const onLimitChange = () => {
  // UserTokenRanking watches the limit prop and reloads automatically.
}

const applyFilters = () => {
  // Ranking reload is triggered by the changed filters via UserTokenRanking watch.
}

const resetFilters = () => {
  filters.value = {}
  days.value = 7
  limit.value = 20
  loadModelStats()
}

const handleSelectUser = (userId: number) => {
  router.push({
    path: '/admin/usage',
    query: {
      user_id: String(userId),
      start_date: startDate.value,
      end_date: endDate.value,
    },
  })
}

const toggleTheme = () => {
  theme.value = theme.value === 'dark' ? 'light' : 'dark'
  try {
    localStorage.setItem(THEME_STORAGE_KEY, theme.value)
  } catch {
    // Storage is optional for this page-local preference.
  }
}

onMounted(() => {
  try {
    const saved = localStorage.getItem(THEME_STORAGE_KEY)
    if (saved === 'dark' || saved === 'light') theme.value = saved
  } catch {
    // Storage is optional for this page-local preference.
  }
  loadModelStats()
})
watch([startDate, endDate], () => loadModelStats())
</script>

<style scoped>
.admin-lb-theme-button {
  display: inline-flex;
  height: 2.5rem;
  width: 2.5rem;
  align-items: center;
  justify-content: center;
  border: 1px solid #d1d5db;
  border-radius: 0.5rem;
  background: #fff;
  font-size: 1rem;
  line-height: 1;
}

.admin-lb-controls {
  display: flex;
  flex-wrap: wrap;
  align-items: end;
  justify-content: space-between;
  gap: 1rem;
  border-bottom: 1px solid #e5e7eb;
  padding: 1rem 1.5rem;
}

.admin-leaderboard[data-theme='dark'] {
  min-height: calc(100dvh - 8rem);
  margin: -1.5rem;
  padding: 1.5rem;
  background: #020617;
  color: #e2e8f0;
}

.admin-leaderboard[data-theme='dark'] :deep(.card),
.admin-leaderboard[data-theme='dark'] :deep(.input),
.admin-leaderboard[data-theme='dark'] :deep(.btn),
.admin-leaderboard[data-theme='dark'] :deep(.admin-lb-theme-button),
.admin-leaderboard[data-theme='dark'] :deep(table tbody) {
  border-color: #334155;
  background-color: #0f172a;
  color: #e2e8f0;
}

.admin-leaderboard[data-theme='dark'] :deep(.input-label),
.admin-leaderboard[data-theme='dark'] :deep(.text-gray-500),
.admin-leaderboard[data-theme='dark'] :deep(.text-gray-400) {
  color: #94a3b8;
}

.admin-leaderboard[data-theme='dark'] :deep(.text-gray-900),
.admin-leaderboard[data-theme='dark'] :deep(.text-gray-700) {
  color: #e2e8f0;
}

.admin-leaderboard[data-theme='dark'] :deep(thead),
.admin-leaderboard[data-theme='dark'] :deep(.bg-gray-50) {
  background-color: #151e32;
}

.admin-leaderboard[data-theme='dark'] :deep(.border-gray-100),
.admin-leaderboard[data-theme='dark'] :deep(.border-gray-200),
.admin-leaderboard[data-theme='dark'] :deep(.divide-gray-200) {
  border-color: #334155;
}

.admin-leaderboard[data-theme='dark'] .admin-lb-controls {
  border-color: #334155;
}

.admin-leaderboard[data-theme='dark'] :deep(tr:hover) {
  background-color: #1e293b;
}
</style>
