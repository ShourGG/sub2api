<template>
  <AppLayout>
    <!-- 极简流体背景: 仅作为质感点缀，不干扰阅读 -->
    <div class="fixed inset-0 pointer-events-none z-0 overflow-hidden bg-gray-50 dark:bg-dark-950">
      <div class="absolute -top-48 -left-48 h-[800px] w-[800px] rounded-full bg-primary-500/5 blur-[120px] dark:bg-primary-900/10"></div>
      <div class="absolute bottom-0 right-0 h-[600px] w-[600px] rounded-full bg-blue-500/5 blur-[100px] dark:bg-blue-900/10"></div>
    </div>

    <div class="relative z-10 mx-auto w-full max-w-[1600px] px-4 py-8 sm:px-6">
      <!-- 页面标题与搜索控制栏 -->
      <div class="mb-8 flex flex-col gap-6 lg:flex-row lg:items-end lg:justify-between">
        <div class="px-2">
          <h1 class="text-3xl font-black tracking-tight text-gray-900 dark:text-white">模型广场</h1>
          <p class="mt-2 text-sm font-medium text-gray-500 dark:text-dark-400">按模型汇聚渠道、可用分组和渠道基础定价。</p>
        </div>
        <div class="flex items-center gap-3">
          <div class="relative group">
            <Icon name="search" size="sm" class="absolute left-4 top-1/2 -translate-y-1/2 text-gray-400 group-focus-within:text-primary-500 transition-colors" />
            <input v-model="search" class="w-full lg:w-80 h-11 bg-white/80 dark:bg-dark-900/60 border border-gray-200 dark:border-dark-800 rounded-xl py-2 pl-11 pr-4 text-sm font-bold outline-none focus:ring-4 focus:ring-primary-500/10 focus:border-primary-500 transition-all shadow-sm" placeholder="搜索模型、渠道、平台或分组..." />
          </div>
          <button class="flex h-11 w-11 items-center justify-center rounded-xl bg-white dark:bg-dark-900 border border-gray-200 dark:border-dark-800 hover:bg-gray-50 dark:hover:bg-dark-800 transition-all shadow-sm active:scale-95" :disabled="loading" @click="loadModels">
            <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
          </button>
        </div>
      </div>

      <!-- 平台筛选菜单 -->
      <div class="mb-8 flex flex-wrap gap-2 px-2">
        <button
          v-for="item in platforms"
          :key="item"
          class="px-5 py-2 rounded-xl text-xs font-black transition-all active:scale-95"
          :class="platform === item ? 'bg-gray-900 text-white shadow-md dark:bg-white dark:text-gray-900' : 'bg-white/80 text-gray-500 border border-gray-100 hover:border-gray-200 dark:bg-dark-900/60 dark:text-dark-400 dark:border-dark-800'"
          @click="platform = item"
        >
          {{ item === 'all' ? '全部平台' : item.toUpperCase() }}
        </button>
      </div>

      <!-- 提示条: 对齐图 7 风格 -->
      <div class="mb-8 mx-2 inline-flex items-center gap-2.5 rounded-xl bg-amber-500/5 px-5 py-3 text-xs font-bold text-amber-700 dark:text-amber-300 border border-amber-500/10 shadow-sm">
        <Icon name="infoCircle" size="xs" />
        提示：下方价格为渠道基础价，实际扣费按所选分组倍率计算；专属倍率和高峰倍率会直接显示在分组标签中。
      </div>

      <!-- 模型列表: 严格对照图 7 的 2 列布局 -->
      <div v-if="loading" class="py-32 text-center">
         <div class="inline-block h-8 w-8 animate-spin rounded-full border-4 border-primary-500/20 border-t-primary-500"></div>
      </div>
      <div v-else-if="filteredModels.length === 0" class="py-32 text-center text-gray-400 font-bold uppercase tracking-widest">没有可展示的模型</div>

      <div v-else class="grid gap-6 xl:grid-cols-2">
        <article v-for="model in filteredModels" :key="model.key" class="bg-white dark:bg-dark-900 border border-gray-200 dark:border-dark-800 rounded-2xl shadow-sm hover:shadow-md transition-all duration-300 overflow-hidden flex flex-col">
          <!-- 1. 卡片页眉 (对照图 7) -->
          <header class="px-6 py-5 flex items-start justify-between border-b border-gray-100 dark:border-dark-800 bg-gray-50/30 dark:bg-dark-950/20">
            <div class="min-w-0">
              <h2 class="text-xl font-black tracking-tight text-gray-900 dark:text-white truncate leading-tight">{{ model.name }}</h2>
              <span class="mt-1 inline-block text-[10px] font-black uppercase text-gray-400 dark:text-dark-500 tracking-widest leading-none">{{ model.platform }}</span>
            </div>
            <div class="shrink-0 text-xs font-black text-gray-400 dark:text-dark-500 uppercase tracking-widest">
              {{ channelCount(model) }} {{ t('modelPlaza.detail.channelCount', channelCount(model)) }}
            </div>
          </header>

          <div class="flex-1 divide-y divide-gray-100 dark:divide-dark-800">
            <section v-for="channel in model.channels" :key="channel.key" class="p-6 space-y-5">
              <!-- 2. 渠道与分组行 (结构 1:1 复刻图 7) -->
              <div class="grid grid-cols-[3rem_minmax(0,1fr)] items-start gap-x-4 gap-y-3">
                <div class="pt-1 text-xs font-black uppercase tracking-[0.16em] text-gray-400 dark:text-dark-500">渠道</div>
                <div class="min-w-0 break-words text-sm font-bold text-gray-800 dark:text-gray-200">{{ channel.name }}</div>
                <div class="pt-1 text-xs font-black uppercase tracking-[0.16em] text-gray-400 dark:text-dark-500">分组</div>
                <div class="model-square-groups min-w-0 flex flex-wrap gap-2 rounded-xl border border-gray-200/70 bg-white/60 p-2.5 dark:border-dark-700/70 dark:bg-dark-950/30">
                  <GroupBadge
                    v-for="entry in channel.entries"
                    :key="entryKey(entry)"
                    :name="entry.group.name"
                    :platform="entry.group.platform as GroupPlatform"
                    :subscription-type="entry.group.subscription_type as SubscriptionType"
                    :rate-multiplier="entry.group.rate_multiplier"
                    :user-rate-multiplier="userGroupRates[entry.group.id] ?? null"
                    :peak-rate-enabled="entry.group.peak_rate_enabled"
                    :peak-start="entry.group.peak_start"
                    :peak-end="entry.group.peak_end"
                    :peak-rate-multiplier="entry.group.peak_rate_multiplier"
                    always-show-rate
                    class="model-square-group-badge max-w-full min-w-0 rounded-lg px-3 py-1"
                  />
                </div>
              </div>

              <!-- 3. 定价详情框 (结构 1:1 复刻图 7) -->
              <div class="rounded-xl border border-gray-200 dark:border-dark-700 bg-gray-50/50 dark:bg-dark-950/40 p-5">
                <!-- 定价页眉 -->
                <div class="flex items-center justify-between mb-5">
                   <h3 class="text-sm font-black text-gray-500 dark:text-dark-400">渠道基础定价</h3>
                   <span class="text-[10px] font-black uppercase text-gray-400 dark:text-dark-500 tracking-widest">
                     {{ billingModeLabel(channel.pricing) }}
                   </span>
                </div>

                <!-- 2x3 网格定价 (图 7 核心结构) -->
                <div class="grid grid-cols-3 gap-y-5 gap-x-4">
                   <div v-for="item in fullPriceItems(channel.pricing)" :key="item.label" class="space-y-1.5">
                      <p class="text-[10px] font-bold text-gray-400 uppercase leading-none">{{ item.label }}</p>
                      <p class="text-sm font-black font-mono text-gray-900 dark:text-white leading-none break-all">
                        {{ formatTokenPrice(item.value) }}
                      </p>
                   </div>
                </div>

                <!-- 特殊计费提示 (对齐图 7 阶梯与计费逻辑) -->
                <div v-if="isRequestBilling(channel.pricing) || (channel.pricing?.intervals?.length)" class="mt-5 pt-5 border-t border-gray-200 dark:border-dark-700 flex flex-wrap items-center gap-6">
                   <div v-if="isRequestBilling(channel.pricing)" class="space-y-1.5">
                      <p class="text-[10px] font-bold text-gray-400 uppercase leading-none">{{ channel.pricing?.billing_mode === 'image' ? '每张价格' : '每次价格' }}</p>
                      <p class="text-sm font-black font-mono text-primary-600 dark:text-primary-400 leading-none">
                        {{ formatRequestPrice(channel.pricing?.per_request_price, channel.pricing?.billing_mode) }}
                      </p>
                   </div>
                   <div v-if="channel.pricing?.intervals?.length" class="flex items-center gap-2 text-primary-600 dark:text-primary-400 bg-primary-500/5 px-4 py-2 rounded-xl border border-primary-500/10">
                      <Icon name="shield" size="xs" />
                      <span class="text-[10px] font-black uppercase tracking-widest font-mono">已配置 {{ channel.pricing.intervals.length }} 档阶梯价格</span>
                   </div>
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
import { useI18n } from 'vue-i18n'

interface ModelSquareModel {
  key: string
  name: string
  platform: string
  entries: ModelSquareEntry[]
  channels: ModelSquareChannel[]
}

interface ModelSquareChannel {
  key: string
  name: string
  entries: ModelSquareEntry[]
  pricing: UserSupportedModelPricing | null
}

const { t } = useI18n()
const appStore = useAppStore()
const loading = ref(false)
const search = ref('')
const platform = ref('all')
const models = ref<ModelSquareEntry[]>([])
const userGroupRates = ref<Record<number, number>>({})

const platforms = computed(() => ['all', ...Array.from(new Set(models.value.map((item) => item.platform))).sort()])
const modelGroups = computed<ModelSquareModel[]>(() => {
  const modelsByKey = new Map<string, ModelSquareModel>()
  for (const entry of models.value) {
    const key = `${entry.platform}:${entry.name.toLowerCase()}`
    const existing = modelsByKey.get(key)
    if (existing) existing.entries.push(entry)
    else modelsByKey.set(key, { key, name: entry.name, platform: entry.platform, entries: [entry], channels: [] })
  }
  return Array.from(modelsByKey.values())
    .map((model) => ({ ...model, channels: groupChannels(model.entries) }))
    .sort((a, b) => a.platform.localeCompare(b.platform) || a.name.localeCompare(b.name))
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
  return model.channels.length
}

function groupChannels(entries: ModelSquareEntry[]): ModelSquareChannel[] {
  const channelsByKey = new Map<string, ModelSquareChannel>()
  for (const entry of entries) {
    const key = entry.channel_id > 0 ? `channel:${entry.channel_id}` : 'account-only'
    const existing = channelsByKey.get(key)
    if (existing) existing.entries.push(entry)
    else channelsByKey.set(key, {
      key,
      name: entry.channel_name || '未关联渠道',
      entries: [entry],
      pricing: entry.pricing,
    })
  }
  return Array.from(channelsByKey.values())
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

function billingModeLabel(pricing: UserSupportedModelPricing | null) {
  switch (pricing?.billing_mode) {
    case 'image': return '图片计费'
    case 'per_request': return '按次计费'
    default: return 'Token 计费'
  }
}

function isRequestBilling(pricing: UserSupportedModelPricing | null) {
  return pricing?.billing_mode === 'image' || pricing?.billing_mode === 'per_request'
}

function fullPriceItems(pricing: UserSupportedModelPricing | null) {
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

<style scoped>
.model-square-groups {
  transition: all 0.3s ease;
}
.model-square-group-badge {
  @apply transition-transform duration-200 hover:scale-110 active:scale-95;
}
</style>
