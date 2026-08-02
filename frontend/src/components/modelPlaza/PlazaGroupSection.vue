<template>
  <section
    class="relative group rounded-[2.5rem] border bg-white/70 shadow-glass backdrop-blur-md transition-all duration-300 hover:shadow-card-hover dark:bg-dark-800/40"
    :class="[platformBorderStrongClass(group.platform)]"
  >
    <!-- 顶部装饰光晕 (随平台变化) -->
    <div
      class="absolute -top-px left-10 right-10 h-px bg-gradient-to-r from-transparent via-[var(--chip-accent,theme(colors.primary.500))] to-transparent opacity-0 group-hover:opacity-100 transition-opacity"
      :style="{ '--chip-accent': platformAccentColor(group.platform) }"
    ></div>

    <!-- 分组头部 -->
    <header class="px-8 py-6">
      <div class="flex flex-wrap items-center justify-between gap-4">
        <div class="flex flex-wrap items-center gap-3">
          <GroupBadge
            :name="group.name"
            :platform="group.platform as GroupPlatform"
            :subscription-type="(group.subscription_type || 'standard') as SubscriptionType"
            :rate-multiplier="group.rate_multiplier"
            :user-rate-multiplier="group.user_rate_multiplier ?? null"
            :peak-rate-enabled="group.peak_rate_enabled"
            :peak-start="group.peak_start"
            :peak-end="group.peak_end"
            :peak-rate-multiplier="group.peak_rate_multiplier"
            always-show-rate
            class="scale-110 origin-left"
          />
          <span
            v-if="group.is_exclusive"
            class="inline-flex items-center gap-1.5 rounded-full bg-purple-500/10 px-3 py-1 text-[11px] font-bold uppercase tracking-wider text-purple-600 dark:text-purple-400 border border-purple-500/20"
          >
            <Icon name="shield" size="xs" class="h-3 w-3" />
            {{ t('modelPlaza.badges.exclusive') }}
          </span>
          <span
            v-if="group.subscription_type === 'subscription'"
            class="inline-flex items-center rounded-full bg-violet-500/10 px-3 py-1 text-[11px] font-bold uppercase tracking-wider text-violet-600 dark:text-violet-400 border border-violet-500/20"
          >
            {{ t('modelPlaza.badges.subscription') }}
          </span>
        </div>

        <div v-if="group.models.length > 0" class="text-xs font-bold text-gray-400 dark:text-dark-500 uppercase tracking-widest bg-gray-50 dark:bg-dark-900/50 px-3 py-1 rounded-full border border-gray-100 dark:border-dark-700/50">
          {{ group.models.length }} {{ t('modelPlaza.detail.modelCount', group.models.length) }}
        </div>
      </div>

      <p v-if="group.description" class="mt-4 text-sm leading-relaxed text-gray-500 dark:text-dark-400 max-w-3xl">
        {{ group.description }}
      </p>

      <div
        v-if="peakNote"
        class="mt-4 inline-flex items-center gap-2 rounded-lg bg-amber-500/5 px-3 py-1.5 text-xs font-semibold text-amber-600 dark:text-amber-400 border border-amber-500/10"
      >
        <Icon name="clock" size="xs" class="h-3.5 w-3.5" />
        {{ peakNote }}
      </div>
    </header>

    <!-- 模型价格网格 -->
    <div class="px-8 pb-8">
      <div class="rounded-3xl overflow-hidden border border-gray-100 dark:border-dark-700/50 bg-gray-50/50 dark:bg-dark-950/30">
        <PlazaModelPricingTable
          v-if="group.models.length > 0"
          :models="group.models"
          :platform="group.platform"
          :rate-multiplier="group.rate_multiplier"
          :user-rate-multiplier="group.user_rate_multiplier ?? null"
        />
        <p v-else class="py-12 text-center text-sm font-medium text-gray-400 dark:text-dark-500">
          {{ t('modelPlaza.detail.noModels') }}
        </p>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import GroupBadge from '@/components/common/GroupBadge.vue'
import PlazaModelPricingTable from './PlazaModelPricingTable.vue'
import type { ModelPlazaGroup } from '@/api/modelPlaza'
import type { GroupPlatform, SubscriptionType } from '@/types'
import { platformBorderStrongClass } from '@/utils/platformColors'
import { hasPeakRate, formatPeakRateWindow, serverTimezoneLabel } from '@/utils/peak-rate'
import { useAppStore } from '@/stores/app'

const props = defineProps<{
  group: ModelPlazaGroup
}>()

const { t } = useI18n()
const appStore = useAppStore()

const peakNote = computed(() => {
  if (!hasPeakRate(props.group)) return ''
  const window = formatPeakRateWindow(
    props.group,
    serverTimezoneLabel(appStore.cachedPublicSettings?.server_utc_offset)
  )
  return t('modelPlaza.detail.peakNote', {
    window,
    multiplier: props.group.peak_rate_multiplier
  })
})
</script>
