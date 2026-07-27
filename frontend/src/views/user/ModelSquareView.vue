<template>
  <AppLayout>
    <div class="mx-auto w-full max-w-[1440px] px-4 py-6 sm:px-6">
      <div class="mb-6 flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
        <div>
          <h1 class="text-xl font-semibold text-[var(--app-text)]">模型广场</h1>
          <p class="mt-1 text-sm text-[var(--app-text-muted)]">查看当前账号可用渠道中的模型与分组计价。</p>
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
        <article v-for="model in filteredModels" :key="model.name" class="rounded-lg border border-[var(--app-border)] bg-[var(--app-surface)] p-4">
          <div class="flex items-start justify-between gap-3">
            <div class="min-w-0">
              <h2 class="truncate font-medium text-[var(--app-text)]">{{ model.name }}</h2>
              <p class="mt-1 text-sm text-[var(--app-text-muted)]">{{ model.platform }}</p>
            </div>
            <span class="shrink-0 text-xs text-[var(--app-text-muted)]">{{ model.channelCount }} 个渠道</span>
          </div>
          <div class="mt-4 grid grid-cols-2 gap-3 text-sm">
            <div><p class="text-[var(--app-text-muted)]">输入</p><p class="mt-1 text-[var(--app-text)]">{{ formatPrice(model.inputPrice) }}</p></div>
            <div><p class="text-[var(--app-text-muted)]">输出</p><p class="mt-1 text-[var(--app-text)]">{{ formatPrice(model.outputPrice) }}</p></div>
          </div>
          <div class="mt-4 border-t border-[var(--app-border)] pt-3 text-xs text-[var(--app-text-muted)]">
            分组：{{ model.groups.join('、') || '未标注' }}
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
import userChannelsAPI, { type UserAvailableChannel } from '@/api/channels'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'

type SquareModel = { name: string; platform: string; inputPrice?: number; outputPrice?: number; groups: string[]; channelCount: number }
const appStore = useAppStore()
const loading = ref(false)
const search = ref('')
const platform = ref('all')
const models = ref<SquareModel[]>([])

const platforms = computed(() => ['all', ...Array.from(new Set(models.value.map((item) => item.platform))).sort()])
const filteredModels = computed(() => {
  const query = search.value.trim().toLowerCase()
  return models.value.filter((item) => (platform.value === 'all' || item.platform === platform.value) && (!query || [item.name, item.platform, ...item.groups].join(' ').toLowerCase().includes(query)))
})

function formatPrice(value?: number) {
  return value === undefined || value === null ? '未配置' : `$${value.toFixed(value < 1 ? 3 : 2)}/M`
}

function toModels(channels: UserAvailableChannel[]) {
  const result = new Map<string, SquareModel>()
  for (const channel of channels) {
    for (const section of channel.platforms) {
      for (const supported of section.supported_models) {
        const existing = result.get(`${section.platform}:${supported.name}`)
        const groups = section.groups.map((group) => group.name)
        if (existing) {
          existing.channelCount += 1
          existing.groups = Array.from(new Set([...existing.groups, ...groups]))
          continue
        }
        result.set(`${section.platform}:${supported.name}`, {
          name: supported.name,
          platform: section.platform,
          inputPrice: supported.pricing?.input_price ?? undefined,
          outputPrice: supported.pricing?.output_price ?? undefined,
          groups,
          channelCount: 1,
        })
      }
    }
  }
  return Array.from(result.values()).sort((a, b) => a.name.localeCompare(b.name))
}

async function loadModels() {
  loading.value = true
  try {
    models.value = toModels(await userChannelsAPI.getAvailable())
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, '加载模型广场失败'))
  } finally {
    loading.value = false
  }
}

onMounted(loadModels)
</script>
