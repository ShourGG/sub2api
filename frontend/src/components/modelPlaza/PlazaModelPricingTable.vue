<template>
  <div class="plaza-pricing-table" :style="accentStyle">
    <div v-for="m in sortedModels" :key="m.name" class="model-row group/row">
      <!-- 模型名称与计费模式 -->
      <div class="model-info p-6 border-b border-gray-100 dark:border-dark-800/50 bg-white/30 dark:bg-dark-900/20">
        <div class="flex flex-wrap items-center justify-between gap-4">
          <div class="flex items-center gap-3">
            <div class="h-10 w-1 flex-shrink-0 rounded-full bg-[var(--plaza-accent)] opacity-40 group-hover/row:opacity-100 transition-opacity"></div>
            <span class="text-lg font-bold tracking-tight text-gray-900 dark:text-white">{{ m.name }}</span>
            <span
              v-if="billingMode(m) !== BILLING_MODE_TOKEN"
              class="rounded-full bg-gray-100 px-3 py-0.5 text-[10px] font-bold uppercase tracking-widest text-gray-500 dark:bg-dark-700/70 dark:text-dark-300 border border-gray-200 dark:border-dark-600"
            >
              {{ billingModeLabel(m) }}
            </span>
          </div>
          <div class="flex items-center gap-4 text-xs font-mono">
             <div v-if="hasCustomRate" class="flex items-center gap-1.5 bg-primary-500/5 px-2 py-1 rounded-lg border border-primary-500/10">
                <span class="text-gray-400 line-through">{{ rateMultiplier }}x</span>
                <span class="font-bold text-primary-600 dark:text-primary-400">{{ effectiveRate }}x</span>
             </div>
             <div v-else class="text-gray-400 dark:text-dark-500 font-bold bg-gray-100 dark:bg-dark-800 px-2 py-1 rounded-lg">
                {{ effectiveRate }}x
             </div>
          </div>
        </div>
      </div>

      <!-- 2x3 价格仪表盘 -->
      <div class="grid grid-cols-1 md:grid-cols-3 divide-y md:divide-y-0 md:divide-x divide-gray-100 dark:divide-dark-800/50">
        <!-- Input -->
        <div class="p-6 transition-colors hover:bg-white/50 dark:hover:bg-dark-800/20">
          <div class="text-[10px] font-bold uppercase tracking-[0.2em] text-gray-400 dark:text-dark-500 mb-3">{{ t('modelPlaza.table.input') }}</div>
          <div class="price-value">
            <template v-if="billingMode(m) === BILLING_MODE_TOKEN">
              <div v-if="tokenIntervals(m).length" class="space-y-2">
                <div v-for="(iv, idx) in tokenIntervals(m)" :key="idx" class="flex items-baseline justify-between font-mono">
                  <span class="text-[10px] text-gray-400 mr-2">{{ tierLabel(iv) }}</span>
                  <span class="text-base font-bold text-gray-900 dark:text-white">{{ paidPerMillion(iv.input_price) }}</span>
                </div>
              </div>
              <div v-else class="text-2xl font-black font-mono tracking-tighter text-gray-900 dark:text-white italic">
                {{ paidPerMillion(m.pricing?.input_price) }}
              </div>
            </template>
            <template v-else>
               <div v-if="requestIntervals(m).length" class="space-y-2">
                <div v-for="(iv, idx) in requestIntervals(m)" :key="idx" class="flex items-baseline justify-between font-mono">
                  <span class="text-[10px] text-gray-400 mr-2">{{ tierLabel(iv) }}</span>
                  <span class="text-base font-bold text-gray-900 dark:text-white">{{ paidRequestPrice(iv.per_request_price) }}</span>
                </div>
              </div>
              <div v-else-if="m.pricing?.per_request_price != null" class="text-2xl font-black font-mono tracking-tighter text-gray-900 dark:text-white italic">
                {{ paidRequestPrice(m.pricing.per_request_price) }}
              </div>
              <div v-else class="empty-state">--</div>
            </template>
          </div>
        </div>

        <!-- Output -->
        <div class="p-6 transition-colors hover:bg-white/50 dark:hover:bg-dark-800/20">
          <div class="text-[10px] font-bold uppercase tracking-[0.2em] text-gray-400 dark:text-dark-500 mb-3">{{ t('modelPlaza.table.output') }}</div>
          <div class="price-value">
            <template v-if="billingMode(m) === BILLING_MODE_TOKEN">
              <div v-if="tokenIntervals(m).length" class="space-y-2">
                <div v-for="(iv, idx) in tokenIntervals(m)" :key="idx" class="flex items-baseline justify-between font-mono">
                  <span class="text-[10px] text-gray-400 mr-2">{{ tierLabel(iv) }}</span>
                  <span class="text-base font-bold text-gray-900 dark:text-white">{{ paidPerMillion(iv.output_price) }}</span>
                </div>
              </div>
              <div v-else class="text-2xl font-black font-mono tracking-tighter text-gray-900 dark:text-white italic">
                {{ paidPerMillion(m.pricing?.output_price) }}
              </div>
            </template>
            <template v-else>
               <div class="empty-state">--</div>
            </template>
          </div>
        </div>

        <!-- Cache/Other -->
        <div class="p-6 transition-colors hover:bg-white/50 dark:hover:bg-dark-800/20">
          <div class="text-[10px] font-bold uppercase tracking-[0.2em] text-gray-400 dark:text-dark-500 mb-3">{{ t('modelPlaza.table.cache') }}</div>
          <div class="price-value">
            <div v-if="hasCachePricing(m)" class="space-y-2 font-mono">
              <div class="flex items-baseline justify-between">
                <span class="text-[10px] text-gray-400 uppercase">{{ t('modelPlaza.table.cacheWrite') }}</span>
                <span class="text-base font-bold text-gray-900 dark:text-white">{{ paidPerMillion(m.pricing?.cache_write_price) }}</span>
              </div>
              <div class="flex items-baseline justify-between">
                <span class="text-[10px] text-gray-400 uppercase">{{ t('modelPlaza.table.cacheRead') }}</span>
                <span class="text-base font-bold text-gray-900 dark:text-white">{{ paidPerMillion(m.pricing?.cache_read_price) }}</span>
              </div>
            </div>
            <div v-else class="empty-state">--</div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { formatScaled } from '@/utils/pricing'
import { platformAccentColor } from '@/utils/platformColors'
import {
  BILLING_MODE_TOKEN,
  BILLING_MODE_IMAGE,
  type BillingMode
} from '@/constants/channel'
import type { PlazaModel } from '@/api/modelPlaza'
import type { UserPricingInterval } from '@/api/channels'

const props = defineProps<{
  models: PlazaModel[]
  /** 分组平台;实付分区底色随平台着色,未知平台回退品牌青。 */
  platform?: string
  /** 分组默认倍率。 */
  rateMultiplier: number
  /** 用户专属倍率;与默认不同,实付价按此计算并划线展示原倍率。 */
  userRateMultiplier?: number | null
}>()

const { t } = useI18n()

/** 实付分区只从平台拿一个主色,浅底/标题/下划线全部由 scoped CSS 用 color-mix 派生。 */
const accentStyle = computed(() => ({ '--plaza-accent': platformAccentColor(props.platform ?? '') }))

const PER_MILLION = 1_000_000

/** 展示顺序:官方输出价从高到低;无官方价的排最后;同价按名称升序。 */
const sortedModels = computed(() => {
  return [...props.models].sort((a, b) => {
    const pa = a.official_pricing?.output_price ?? null
    const pb = b.official_pricing?.output_price ?? null
    if (pa != null && pb != null && pa !== pb) return pb - pa
    if (pa != null && pb == null) return -1
    if (pa == null && pb != null) return 1
    return a.name.localeCompare(b.name)
  })
})

const effectiveRate = computed(() => props.userRateMultiplier ?? props.rateMultiplier)
const hasCustomRate = computed(
  () => props.userRateMultiplier != null && props.userRateMultiplier !== props.rateMultiplier
)

function billingMode(m: PlazaModel): BillingMode {
  return (m.pricing?.billing_mode || BILLING_MODE_TOKEN) as BillingMode
}

function billingModeLabel(m: PlazaModel): string {
  return billingMode(m) === BILLING_MODE_IMAGE
    ? t('modelPlaza.table.perImage')
    : t('modelPlaza.table.perRequest')
}

/** 价格统一保底 2 位小数,更长的有效小数原样保留。 */
const MIN_DECIMALS = 2

/** 实付价 = 渠道单价 × 生效倍率,按 $/1M token 展示。 */
function paidPerMillion(value: number | null | undefined): string {
  if (value == null) return '-'
  return formatScaled(value * effectiveRate.value, PER_MILLION, MIN_DECIMALS)
}

/** 按次 / 按图片单价(乘生效倍率,不换算 1M)。 */
function paidRequestPrice(value: number | null | undefined): string {
  if (value == null) return '-'
  return formatScaled(value * effectiveRate.value, 1, MIN_DECIMALS)
}

/** 官方参考价不乘倍率。 */
function official(value: number | null | undefined): string {
  if (value == null) return '-'
  return formatScaled(value, PER_MILLION, MIN_DECIMALS)
}

/** 非 token 计费的单位后缀:按图片 → “/ 张”,按次 → “/ 次”。 */
function perUnitSuffix(m: PlazaModel): string {
  return billingMode(m) === BILLING_MODE_IMAGE
    ? t('modelPlaza.table.perUnitImage')
    : t('modelPlaza.table.perUnitRequest')
}

function hasCachePricing(m: PlazaModel): boolean {
  return m.pricing?.cache_write_price != null || m.pricing?.cache_read_price != null
}

function hasOfficialCache(o: NonNullable<PlazaModel['official_pricing']>): boolean {
  return o.cache_write_price != null || o.cache_read_price != null || o.cache_write_1h_price != null
}

/** token 模式的阶梯定价(内联进输入/输出列)。 */
function tokenIntervals(m: PlazaModel): UserPricingInterval[] {
  return m.pricing?.intervals ?? []
}

/** 按次/按图模式的阶梯定价(仅保留配了按次价的档位)。 */
function requestIntervals(m: PlazaModel): UserPricingInterval[] {
  return (m.pricing?.intervals ?? []).filter((iv) => iv.per_request_price != null)
}

/** 档位标签:优先管理员配置的 tier_label,否则按 token 区间生成(≤200K / >200K / 200K–1M)。 */
function tierLabel(iv: UserPricingInterval): string {
  if (iv.tier_label) return iv.tier_label
  const { min_tokens: min, max_tokens: max } = iv
  if (max == null) return `>${formatTokenCount(min)}`
  if (min === 0) return `≤${formatTokenCount(max)}`
  return `${formatTokenCount(min)}–${formatTokenCount(max)}`
}

function formatTokenCount(n: number): string {
  if (n >= 1_000_000) return `${trimZero(n / 1_000_000)}M`
  if (n >= 1_000) return `${trimZero(n / 1_000)}K`
  return String(n)
}

function trimZero(n: number): string {
  return String(Math.round(n * 100) / 100)
}
</script>

<style scoped>
.plaza-pricing-table {
  --pz-accent-alpha: color-mix(in srgb, var(--plaza-accent) 15%, transparent);
}

.model-row {
  @apply mb-6 last:mb-0 overflow-hidden;
}

.price-value {
  @apply min-h-[40px] flex flex-col justify-center;
}

.empty-state {
  @apply text-gray-300 dark:text-dark-600 font-mono text-sm tracking-widest;
  background-image: repeating-linear-gradient(
    45deg,
    transparent,
    transparent 5px,
    rgba(156, 163, 175, 0.05) 5px,
    rgba(156, 163, 175, 0.05) 10px
  );
}

.dark .empty-state {
  background-image: repeating-linear-gradient(
    45deg,
    transparent,
    transparent 5px,
    rgba(75, 85, 99, 0.1) 5px,
    rgba(75, 85, 99, 0.1) 10px
  );
}

/* 增强数值显示 */
.font-black {
  text-shadow: 0 2px 10px color-mix(in srgb, var(--plaza-accent) 20%, transparent);
}
</style>
