<template>
  <AppLayout>
    <div class="leaderboard-page space-y-6" :data-theme="theme">
      <!-- Header: title + period tabs + theme toggle + refresh -->
      <section class="lb-card lb-header">
        <div class="lb-header-main">
          <h1 class="lb-title">{{ t('leaderboard.title') }}</h1>
          <p class="lb-desc">{{ t('leaderboard.description') }}</p>
        </div>
        <div class="lb-header-actions">
          <div class="lb-tabs" role="tablist" :aria-label="t('leaderboard.periodLabel')">
            <button
              v-for="opt in periodOptions"
              :key="opt.days"
              type="button"
              role="tab"
              class="lb-tab"
              :class="{ 'lb-tab--active': days === opt.days }"
              :aria-selected="days === opt.days"
              @click="selectDays(opt.days)"
            >
              {{ opt.label }}
            </button>
          </div>
          <button type="button" class="lb-icon-btn" :title="themeToggleLabel" @click="toggleTheme">
            {{ theme === 'dark' ? '☀️' : '🌙' }}
          </button>
          <button
            type="button"
            class="lb-icon-btn"
            :disabled="loading"
            :title="t('leaderboard.refresh')"
            @click="reload"
          >
            <span :class="{ 'lb-spin': loading }">⟳</span>
          </button>
        </div>
      </section>

      <!-- Loading -->
      <section v-if="loading && !leaderboard" class="lb-card lb-state">
        <LoadingSpinner />
      </section>

      <!-- Error -->
      <section v-else-if="error" class="lb-card lb-state">
        <h2 class="lb-state-title">{{ t('leaderboard.errorTitle') }}</h2>
        <p class="lb-state-desc">{{ t('leaderboard.errorDescription') }}</p>
        <button type="button" class="lb-retry" @click="reload">{{ t('leaderboard.retry') }}</button>
      </section>

      <!-- Empty -->
      <section v-else-if="!items.length" class="lb-card lb-state">
        <h2 class="lb-state-title">{{ t('leaderboard.emptyTitle') }}</h2>
        <p class="lb-state-desc">{{ t('leaderboard.emptyDescription') }}</p>
      </section>

      <!-- Leaderboard table -->
      <section v-else class="lb-card lb-board">
        <div class="lb-board-head">
          <span class="lb-board-top">{{ t('leaderboard.top', { count: leaderboard?.limit ?? 20 }) }}</span>
          <span v-if="generatedAtLabel" class="lb-board-updated">
            {{ t('leaderboard.generatedAt', { time: generatedAtLabel }) }}
          </span>
        </div>

        <div class="lb-table-wrap">
          <table class="lb-table">
            <thead>
              <tr>
                <th class="lb-col-rank">{{ t('leaderboard.rank') }}</th>
                <th class="lb-col-user">{{ t('leaderboard.user') }}</th>
                <th class="lb-col-num">{{ t('leaderboard.totalTokens') }}</th>
                <th class="lb-col-num lb-hide-sm">{{ t('leaderboard.inputTokensShort') }}</th>
                <th class="lb-col-num lb-hide-sm">{{ t('leaderboard.outputTokensShort') }}</th>
                <th class="lb-col-num lb-hide-sm">{{ t('leaderboard.cacheTokensShort') }}</th>
                <th class="lb-col-num lb-hide-sm">{{ t('leaderboard.imageOutputShort') }}</th>
                <th class="lb-col-num">{{ t('leaderboard.requests') }}</th>
                <th class="lb-col-time lb-hide-sm">{{ t('leaderboard.lastActive') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="item in items"
                :key="item.rank"
                class="lb-row"
                :class="{ 'lb-row--me': item.is_me }"
              >
                <td class="lb-col-rank">
                  <span class="lb-rank" :class="medalClass(item.rank)">
                    <span v-if="medalEmoji(item.rank)" class="lb-medal">{{ medalEmoji(item.rank) }}</span>
                    <span v-else>{{ item.rank }}</span>
                  </span>
                </td>
                <td class="lb-col-user">
                  <span class="lb-user">{{ item.is_me ? t('leaderboard.me') : item.user }}</span>
                  <span v-if="item.is_me" class="lb-me-tag">{{ t('leaderboard.currentUser') }}</span>
                </td>
                <td class="lb-col-num lb-strong">{{ formatTokens(item.total_tokens) }}</td>
                <td class="lb-col-num lb-hide-sm">{{ formatTokens(item.input_tokens) }}</td>
                <td class="lb-col-num lb-hide-sm">{{ formatTokens(item.output_tokens) }}</td>
                <td class="lb-col-num lb-hide-sm">{{ formatTokens(item.cache_tokens) }}</td>
                <td class="lb-col-num lb-hide-sm">{{ formatTokens(item.image_output_tokens) }}</td>
                <td class="lb-col-num">{{ formatNumber(item.requests) }}</td>
                <td class="lb-col-time lb-hide-sm">{{ item.last_active_at || '—' }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import {
  usageAPI,
  type TokenLeaderboardResponse,
  type LeaderboardParams
} from '@/api/usage'

const { t } = useI18n()

type DaysWindow = 1 | 7 | 30

const THEME_STORAGE_KEY = 'leaderboard.theme'

const leaderboard = ref<TokenLeaderboardResponse | null>(null)
const loading = ref(false)
const error = ref(false)
const days = ref<DaysWindow>(1)
const theme = ref<'light' | 'dark'>('light')

let abortController: AbortController | null = null

const periodOptions = computed<{ days: DaysWindow; label: string }[]>(() => [
  { days: 1, label: t('leaderboard.period.day') },
  { days: 7, label: t('leaderboard.period.week') },
  { days: 30, label: t('leaderboard.period.month') }
])

const items = computed(() => leaderboard.value?.items ?? [])

const themeToggleLabel = computed(() =>
  theme.value === 'dark' ? t('leaderboard.theme.light') : t('leaderboard.theme.dark')
)

const generatedAtLabel = computed(() => {
  const end = leaderboard.value?.end
  if (!end) return ''
  const d = new Date(end)
  if (Number.isNaN(d.getTime())) return ''
  return d.toLocaleString(undefined, {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit'
  })
})

const browserTimezone = (() => {
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone || ''
  } catch {
    return ''
  }
})()

function medalEmoji(rank: number): string {
  switch (rank) {
    case 1:
      return '🥇'
    case 2:
      return '🥈'
    case 3:
      return '🥉'
    default:
      return ''
  }
}

function medalClass(rank: number): string {
  switch (rank) {
    case 1:
      return 'lb-rank--gold'
    case 2:
      return 'lb-rank--silver'
    case 3:
      return 'lb-rank--bronze'
    default:
      return ''
  }
}

const numberFormatter = new Intl.NumberFormat()

function formatNumber(value: number): string {
  return numberFormatter.format(value ?? 0)
}

function formatTokens(value: number): string {
  const v = value ?? 0
  if (v >= 1_000_000_000) return `${(v / 1_000_000_000).toFixed(2)}B`
  if (v >= 1_000_000) return `${(v / 1_000_000).toFixed(2)}M`
  if (v >= 1_000) return `${(v / 1_000).toFixed(2)}K`
  return numberFormatter.format(v)
}

async function load() {
  loading.value = true
  error.value = false

  if (abortController) {
    abortController.abort()
  }
  abortController = new AbortController()

  const params: LeaderboardParams = { days: days.value }
  if (browserTimezone) {
    params.timezone = browserTimezone
  }

  try {
    const data = await usageAPI.getDashboardLeaderboard(params, {
      signal: abortController.signal
    })
    leaderboard.value = data
  } catch (err: unknown) {
    if ((err as { name?: string })?.name === 'CanceledError' || (err as { code?: string })?.code === 'ERR_CANCELED') {
      return
    }
    console.error('Failed to load leaderboard:', err)
    error.value = true
  } finally {
    loading.value = false
  }
}

function reload() {
  void load()
}

function selectDays(next: DaysWindow) {
  if (days.value === next) return
  days.value = next
  void load()
}

function applyTheme(next: 'light' | 'dark') {
  theme.value = next
  try {
    localStorage.setItem(THEME_STORAGE_KEY, next)
  } catch {
    /* ignore storage errors */
  }
}

function toggleTheme() {
  applyTheme(theme.value === 'dark' ? 'light' : 'dark')
}

onMounted(() => {
  try {
    const saved = localStorage.getItem(THEME_STORAGE_KEY)
    if (saved === 'dark' || saved === 'light') {
      theme.value = saved
    }
  } catch {
    /* ignore storage errors */
  }
  void load()
})

onBeforeUnmount(() => {
  if (abortController) {
    abortController.abort()
    abortController = null
  }
})
</script>

<style scoped>
.leaderboard-page {
  --lb-bg: #ffffff;
  --lb-fg: #1f2937;
  --lb-muted: #6b7280;
  --lb-border: #e5e7eb;
  --lb-row-hover: #f9fafb;
  --lb-me-bg: #eff6ff;
  --lb-me-border: #3b82f6;
  --lb-accent: #2563eb;
}

.leaderboard-page[data-theme='dark'] {
  --lb-bg: #0f172a;
  --lb-fg: #e2e8f0;
  --lb-muted: #94a3b8;
  --lb-border: #1e293b;
  --lb-row-hover: #1e293b;
  --lb-me-bg: #1e3a5f;
  --lb-me-border: #3b82f6;
  --lb-accent: #60a5fa;
}

.lb-card {
  background: var(--lb-bg);
  color: var(--lb-fg);
  border: 1px solid var(--lb-border);
  border-radius: 12px;
  padding: 1.25rem;
}

.lb-header {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
}

.lb-title {
  font-size: 1.25rem;
  font-weight: 700;
  margin: 0;
}

.lb-desc {
  margin: 0.25rem 0 0;
  font-size: 0.85rem;
  color: var(--lb-muted);
}

.lb-header-actions {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  flex-wrap: wrap;
}

.lb-tabs {
  display: inline-flex;
  background: var(--lb-row-hover);
  border: 1px solid var(--lb-border);
  border-radius: 999px;
  padding: 3px;
}

.lb-tab {
  border: none;
  background: transparent;
  color: var(--lb-muted);
  padding: 0.35rem 0.85rem;
  border-radius: 999px;
  font-size: 0.85rem;
  cursor: pointer;
  transition: all 0.15s ease;
}

.lb-tab--active {
  background: var(--lb-bg);
  color: var(--lb-accent);
  font-weight: 600;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.08);
}

.lb-icon-btn {
  width: 34px;
  height: 34px;
  border-radius: 8px;
  border: 1px solid var(--lb-border);
  background: var(--lb-bg);
  color: var(--lb-fg);
  cursor: pointer;
  font-size: 1rem;
  line-height: 1;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}

.lb-icon-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.lb-spin {
  display: inline-block;
  animation: lb-spin 0.8s linear infinite;
}

@keyframes lb-spin {
  to {
    transform: rotate(360deg);
  }
}

.lb-state {
  text-align: center;
  padding: 3rem 1.25rem;
}

.lb-state-title {
  font-size: 1.05rem;
  font-weight: 600;
  margin: 0 0 0.35rem;
}

.lb-state-desc {
  color: var(--lb-muted);
  font-size: 0.9rem;
  margin: 0 0 1rem;
}

.lb-retry {
  border: 1px solid var(--lb-accent);
  color: var(--lb-accent);
  background: transparent;
  border-radius: 8px;
  padding: 0.4rem 1.1rem;
  cursor: pointer;
  font-size: 0.85rem;
}

.lb-board-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 0.75rem;
}

.lb-board-top {
  font-weight: 600;
}

.lb-board-updated {
  font-size: 0.8rem;
  color: var(--lb-muted);
}

.lb-table-wrap {
  overflow-x: auto;
}

.lb-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.88rem;
}

.lb-table th,
.lb-table td {
  padding: 0.6rem 0.75rem;
  text-align: left;
  border-bottom: 1px solid var(--lb-border);
  white-space: nowrap;
}

.lb-table th {
  color: var(--lb-muted);
  font-weight: 600;
  font-size: 0.78rem;
  text-transform: uppercase;
  letter-spacing: 0.02em;
}

.lb-col-num {
  text-align: right;
}

.lb-col-time {
  text-align: right;
}

.lb-row:hover {
  background: var(--lb-row-hover);
}

.lb-row--me {
  background: var(--lb-me-bg);
  box-shadow: inset 3px 0 0 var(--lb-me-border);
}

.lb-row--me:hover {
  background: var(--lb-me-bg);
}

.lb-rank {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 1.75rem;
  font-weight: 600;
}

.lb-medal {
  font-size: 1.1rem;
}

.lb-rank--gold,
.lb-rank--silver,
.lb-rank--bronze {
  font-weight: 700;
}

.lb-user {
  font-weight: 500;
}

.lb-me-tag {
  margin-left: 0.4rem;
  font-size: 0.7rem;
  color: var(--lb-accent);
  border: 1px solid var(--lb-accent);
  border-radius: 999px;
  padding: 0.05rem 0.4rem;
}

.lb-strong {
  font-weight: 700;
}

@media (max-width: 640px) {
  .lb-hide-sm {
    display: none;
  }
}
</style>
