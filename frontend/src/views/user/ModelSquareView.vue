<template>
  <AppLayout>
    <div class="mx-auto w-full max-w-[1440px] px-4 py-6 sm:px-6">
      <div class="mb-6 flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
        <div>
          <h1 class="text-xl font-semibold text-[var(--app-text)]">模型广场</h1>
          <p class="mt-1 text-sm text-[var(--app-text-muted)]">按模型汇聚渠道、可用分组和渠道基础定价。</p>
        </div>
        <div class="flex w-full gap-3 lg:w-auto">
          <input v-model="search" class="input min-w-0 flex-1 lg:w-80" placeholder="搜索模型、渠道、平台或分组..." />
          <button class="btn btn-secondary" :disabled="loading" title="刷新" @click="loadModels">
            <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
          </button>
        </div>
      </div>

      <div class="mb-4 flex flex-wrap gap-2">
        <button
          v-for="item in platforms"
          :key="item"
          class="rounded border px-3 py-1.5 text-sm"
          :class="platform === item ? 'border-[var(--app-primary)] bg-[var(--app-primary)] text-white' : 'border-[var(--app-border)] text-[var(--app-text-muted)]'"
          @click="platform = item"
        >{{ item === 'all' ? '全部平台' : item }}</button>
      </div>

      <p class="mb-4 rounded border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-800 dark:border-amber-900/70 dark:bg-amber-950/30 dark:text-amber-200">
        下方价格为渠道基础价，实际扣费按所选分组倍率计算；专属倍率和高峰倍率会直接显示在分组标签中。
      </p>

      <div v-if="loading" class="py-16 text-center text-sm text-[var(--app-text-muted)]">加载中...</div>
      <div v-else-if="filteredModels.length === 0" class="py-16 text-center text-sm text-[var(--app-text-muted)]">没有可展示的模型</div>
      <div v-else class="grid gap-4 xl:grid-cols-2">
        <article v-for="model in filteredModels" :key="model.key" class="overflow-hidden rounded-lg border border-[var(--app-border)] bg-[var(--app-surface)]">
          <header class="flex items-start justify-between gap-3 border-b border-[var(--app-border)] px-4 py-3.5">
            <div class="min-w-0">
              <h2 class="truncate font-semibold text-[var(--app-text)]">{{ model.name }}</h2>
              <p class="mt-1 text-xs text-[var(--app-text-muted)]">{{ model.platform }}</p>
            </div>
            <span class="shrink-0 text-xs text-[var(--app-text-muted)]">{{ channelCount(model) }} 个渠道</span>
          </header>

          <div class="divide-y divide-[var(--app-border)]">
            <section v-for="entry in model.entries" :key="entryKey(entry)" class="p-4">
              <div class="flex flex-wrap items-center justify-between gap-2">
                <div>
                  <p class="font-medium text-[var(--app-text)]">{{ entry.channel_name || '账号模型' }}</p>
                  <p class="mt-0.5 text-xs text-[var(--app-text-muted)]">{{ entry.account_count }} 个可调度账号</p>
                </div>
                <GroupBadge
                  :name="entry.group.name"
                  :platform="entry.group.platform as GroupPlatform"
                  :subscription-type="entry.group.subscription_type as SubscriptionType"
                  :rate-multiplier="entry.group.rate_multiplier"
                  :user-rate-multiplier="userGroupRates[entry.group.id] ?? null"
                  :peak-rate-enabled="entry.group.peak_rate_enabled"
                  :peak-start="entry.group.peak_start"
                  :peak-end="entry.group.peak_end"
                  :peak-rate-multiplier="entry.group.peak_rate_multiplier"
                  :always-show-rate="true"
                />
              </div>

              <div class="mt-4 rounded border border-[var(--app-border)] bg-[var(--app-bg)] p-3">
                <div class="flex flex-wrap items-center justify-between gap-2">
                  <p class="text-sm font-medium text-[var(--app-text)]">渠道基础定价</p>
                  <span class="text-xs text-[var(--app-text-muted)]">{{ billingModeLabel(entry) }}</span>
                </div>
                <div class="mt-3 grid grid-cols-2 gap-x-4 gap-y-3 text-sm sm:grid-cols-3">
                  <div v-for="item in priceItems(entry)" :key="item.label">
                    <p class="text-xs text-[var(--app-text-muted)]">{{ item.label }}</p>
                    <p class="mt-1 break-all font-mono text-[var(--app-text)]">{{ formatTokenPrice(item.value) }}</p>
                  </div>
                </div>
                <div v-if="isRequestBilling(entry)" class="mt-3 border-t border-[var(--app-border)] pt-3 text-sm">
                  <p class="text-xs text-[var(--app-text-muted)]">{{ entry.pricing?.billing_mode === 'image' ? '每张价格' : '每次价格' }}</p>
                  <p class="mt-1 font-mono text-[var(--app-text)]">{{ formatRequestPrice(entry.pricing?.per_request_price, entry.pricing?.billing_mode) }}</p>
                </div>
                <div v-if="entry.pricing?.intervals?.length" class="mt-3 border-t border-[var(--app-border)] pt-3 text-xs text-[var(--app-text-muted)]">
                  已配置 {{ entry.pricing.intervals.length }} 个阶梯价格
                </div>
              </div>
            </section>
          </div>
        </article>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import GroupBadge from '@/components/common/GroupBadge.vue'
import modelSquareAPI, { type ModelSquareEntry } from '@/api/modelSquare'
import type { UserSupportedModelPricing } from '@/api/channels'
import userGroupsAPI from '@/api/groups'
import type { GroupPlatform, SubscriptionType } from '@/types'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'

interface ModelSquareModel {
  key: string
  name: string
  platform: string
  entries: ModelSquareEntry[]
}

const appStore = useAppStore()
const loading = ref(false)
const search = ref('')
const platform = ref('all')
const models = ref<ModelSquareEntry[]>([])
const userGroupRates = ref<Record<number, number>>({})

const platforms = computed(() => ['all', ...Array.from(new Set(models.value.map((item) => item.platform))).sort()])
const modelGroups = computed<ModelSquareModel[]>(() => {
  const groups = new Map<string, ModelSquareModel>()
  for (const entry of models.value) {
    const key = `${entry.platform}:${entry.name.toLowerCase()}`
    const existing = groups.get(key)
    if (existing) existing.entries.push(entry)
    else groups.set(key, { key, name: entry.name, platform: entry.platform, entries: [entry] })
  }
  return Array.from(groups.values()).sort((a, b) => a.platform.localeCompare(b.platform) || a.name.localeCompare(b.name))
})
const filteredModels = computed(() => {
  const query = search.value.trim().toLowerCase()
  return modelGroups.value.filter((model) => {
    if (platform.value !== 'all' && model.platform !== platform.value) return false
    if (!query) return true
    return [model.name, model.platform, ...model.entries.flatMap((entry) => [entry.channel_name, entry.group.name])]
      .join(' ')
      .toLowerCase()
      .includes(query)
  })
})

const PER_MILLION_TOKENS = 1_000_000

function entryKey(entry: ModelSquareEntry) {
  return `${entry.channel_id}:${entry.group.id}:${entry.name}`
}

function channelCount(model: ModelSquareModel) {
  return new Set(model.entries.map((entry) => entry.channel_id || `account:${entry.group.id}`)).size
}

function formatTokenPrice(value: number | null | undefined) {
  if (value == null) return '未配置'
  const perMillion = value * PER_MILLION_TOKENS
  return `$${perMillion.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: perMillion < 1 ? 6 : 2 })}/M`
}

function formatRequestPrice(value: number | null | undefined, billingMode?: string) {
  if (value == null) return '未配置'
  return `$${value.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 6 })}/${billingMode === 'image' ? '张' : '次'}`
}

function billingModeLabel(entry: ModelSquareEntry) {
  switch (entry.pricing?.billing_mode) {
    case 'image': return '图片计费'
    case 'per_request': return '按次计费'
    default: return 'Token 计费'
  }
}

function isRequestBilling(entry: ModelSquareEntry) {
  return entry.pricing?.billing_mode === 'image' || entry.pricing?.billing_mode === 'per_request'
}

function priceItems(entry: ModelSquareEntry) {
  const pricing: UserSupportedModelPricing | null = entry.pricing
  return [
    { label: '输入', value: pricing?.input_price },
    { label: '输出', value: pricing?.output_price },
    { label: '缓存写入', value: pricing?.cache_write_price },
    { label: '缓存读取', value: pricing?.cache_read_price },
    { label: '图片输入', value: pricing?.image_input_price },
    { label: '图片输出', value: pricing?.image_output_price },
  ]
}

async function loadModels() {
  loading.value = true
  try {
    models.value = await modelSquareAPI.list()
    userGroupRates.value = await userGroupsAPI.getUserGroupRates().catch(() => ({}))
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, '加载模型广场失败'))
  } finally {
    loading.value = false
  }
}

onMounted(loadModels)
</script>
