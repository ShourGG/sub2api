<template>
  <AppLayout>
    <div class="mx-auto w-full max-w-[1440px] px-4 py-6 sm:px-6">
      <div class="mb-6 flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
        <div>
          <h1 class="text-xl font-semibold text-[var(--app-text)]">模型广场</h1>
          <p class="mt-1 text-sm text-[var(--app-text-muted)]">自动汇聚活跃分组、可调度账号和渠道定价。</p>
        </div>
        <div class="flex w-full gap-3 lg:w-auto">
          <input v-model="search" class="input min-w-0 flex-1 lg:w-80" placeholder="搜索模型、平台或分组..." />
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

      <div v-if="loading" class="py-16 text-center text-sm text-[var(--app-text-muted)]">加载中...</div>
      <div v-else-if="filteredModels.length === 0" class="py-16 text-center text-sm text-[var(--app-text-muted)]">没有可展示的模型</div>
      <div v-else class="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
        <article v-for="model in filteredModels" :key="`${model.group.id}:${model.name}`" class="rounded-lg border border-[var(--app-border)] bg-[var(--app-surface)] p-4">
          <div class="flex items-start justify-between gap-3">
            <div class="min-w-0">
              <h2 class="truncate font-medium text-[var(--app-text)]">{{ model.name }}</h2>
              <p class="mt-1 text-sm text-[var(--app-text-muted)]">{{ model.platform }}</p>
            </div>
            <span class="shrink-0 text-xs text-[var(--app-text-muted)]">{{ model.account_count }} 个账号</span>
          </div>
          <div class="mt-4 grid grid-cols-2 gap-3 text-sm">
            <div><p class="text-[var(--app-text-muted)]">输入</p><p class="mt-1 text-[var(--app-text)]">{{ formatPrice(model.pricing?.input_price ?? undefined) }}</p></div>
            <div><p class="text-[var(--app-text-muted)]">输出</p><p class="mt-1 text-[var(--app-text)]">{{ formatPrice(model.pricing?.output_price ?? undefined) }}</p></div>
          </div>
          <div class="mt-4 border-t border-[var(--app-border)] pt-3">
            <GroupBadge
              :name="model.group.name"
              :platform="model.group.platform as GroupPlatform"
              :subscription-type="model.group.subscription_type as SubscriptionType"
              :rate-multiplier="model.group.rate_multiplier"
              :peak-rate-enabled="model.group.peak_rate_enabled"
              :peak-start="model.group.peak_start"
              :peak-end="model.group.peak_end"
              :peak-rate-multiplier="model.group.peak_rate_multiplier"
              :always-show-rate="true"
            />
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
import type { GroupPlatform, SubscriptionType } from '@/types'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'

const appStore = useAppStore()
const loading = ref(false)
const search = ref('')
const platform = ref('all')
const models = ref<ModelSquareEntry[]>([])

const platforms = computed(() => ['all', ...Array.from(new Set(models.value.map((item) => item.platform))).sort()])
const filteredModels = computed(() => {
  const query = search.value.trim().toLowerCase()
  return models.value.filter((item) => (platform.value === 'all' || item.platform === platform.value) && (!query || [item.name, item.platform, item.group.name].join(' ').toLowerCase().includes(query)))
})

function formatPrice(value?: number) {
  return value === undefined || value === null ? '未配置' : `$${value.toFixed(value < 1 ? 3 : 2)}/M`
}

async function loadModels() {
  loading.value = true
  try {
    models.value = await modelSquareAPI.list()
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, '加载模型广场失败'))
  } finally {
    loading.value = false
  }
}

onMounted(loadModels)
</script>
