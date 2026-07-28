<template>
  <AppLayout>
    <div class="space-y-6">
      <div class="flex items-center justify-between">
        <div>
          <h1 class="text-xl font-semibold">{{ t('leaderboard.title') }}</h1>
          <p class="mt-1 text-sm text-muted-foreground">{{ t('leaderboard.description') }}</p>
        </div>
        <button
          class="btn btn-secondary btn-sm"
          :disabled="loading"
          type="button"
          @click="load"
        >
          {{ t('leaderboard.retry') }}
        </button>
      </div>

      <!-- Loading -->
      <div v-if="loading" class="flex items-center justify-center py-16">
        <LoadingSpinner />
        <span class="ml-3 text-sm text-muted-foreground">{{ t('leaderboard.loading') }}</span>
      </div>

      <!-- Error -->
      <div v-else-if="error" class="card p-8 text-center">
        <h2 class="text-base font-semibold">{{ t('leaderboard.errorTitle') }}</h2>
        <p class="mt-2 text-sm text-muted-foreground">{{ t('leaderboard.errorDescription') }}</p>
        <button class="btn btn-primary mt-4" type="button" @click="load">
          {{ t('leaderboard.retry') }}
        </button>
      </div>

      <!-- Leaderboard table -->
      <template v-else-if="leaderboard">
        <!-- My entry (always visible even if outside top-20) -->
        <div
          v-if="leaderboard.current_user_rank"
          class="card rounded-xl border-l-4 border-primary p-4"
          data-testid="leaderboard-my-rank"
        >
          <p class="mb-1 text-sm font-medium text-muted-foreground">{{ t('leaderboard.currentUser') }}</p>
          <div class="flex items-center gap-4">
            <span
              class="inline-flex h-7 w-7 items-center justify-center rounded-full text-xs font-bold"
              :class="rankBadgeClass(leaderboard.current_user_rank.rank)"
            >#{{ leaderboard.current_user_rank.rank }}</span>
            <span class="font-medium">{{ leaderboard.current_user_rank.display_name }}</span>
            <span class="ml-auto font-mono tabular-nums text-sm">{{ formatTokens(leaderboard.current_user_rank.total_tokens) }} tokens</span>
          </div>
        </div>

        <!-- Rankings -->
        <div class="card overflow-hidden">
          <div class="overflow-x-auto">
            <table class="w-full text-sm" aria-label="Token leaderboard">
              <thead>
                <tr class="border-b text-left text-xs text-muted-foreground">
                  <th class="px-4 py-3 w-12">{{ t('leaderboard.rank') }}</th>
                  <th class="px-4 py-3">{{ t('leaderboard.user') }}</th>
                  <th class="px-4 py-3 text-right">{{ t('leaderboard.tokens') }}</th>
                  <th class="hidden px-4 py-3 text-right md:table-cell">{{ t('leaderboard.inputTokens') }}</th>
                  <th class="hidden px-4 py-3 text-right md:table-cell">{{ t('leaderboard.outputTokens') }}</th>
                  <th class="hidden px-4 py-3 text-right lg:table-cell">{{ t('leaderboard.cacheTokens') }}</th>
                  <th class="hidden px-4 py-3 text-right lg:table-cell">{{ t('leaderboard.imageTokens') }}</th>
                </tr>
              </thead>
              <tbody>
                <template v-if="leaderboard.ranking.length">
                  <tr
                    v-for="item in leaderboard.ranking"
                    :key="item.user_id"
                    class="border-b last:border-0 transition-colors hover:bg-muted/30"
                    :class="item.is_current_user ? 'bg-primary/5 font-medium' : ''"
                    :data-testid="`leaderboard-row-${item.rank}`"
                  >
                    <td class="px-4 py-3">
                      <span
                        class="inline-flex h-7 w-7 items-center justify-center rounded-full text-xs font-bold"
                        :class="rankBadgeClass(item.rank)"
                      >{{ item.rank }}</span>
                    </td>
                    <td class="px-4 py-3">
                      <span>{{ item.display_name }}</span>
                      <span v-if="item.is_current_user" class="ml-2 rounded bg-primary/10 px-1.5 py-0.5 text-xs text-primary">
                        {{ t('leaderboard.currentUser') }}
                      </span>
                    </td>
                    <td class="px-4 py-3 text-right font-mono tabular-nums">
                      {{ formatTokens(item.total_tokens) }}
                    </td>
                    <td class="hidden px-4 py-3 text-right font-mono tabular-nums text-muted-foreground md:table-cell">
                      {{ formatTokens(item.input_tokens) }}
                    </td>
                    <td class="hidden px-4 py-3 text-right font-mono tabular-nums text-muted-foreground md:table-cell">
                      {{ formatTokens(item.output_tokens) }}
                    </td>
                    <td class="hidden px-4 py-3 text-right font-mono tabular-nums text-muted-foreground lg:table-cell">
                      {{ formatTokens(item.cache_creation_tokens + item.cache_read_tokens) }}
                    </td>
                    <td class="hidden px-4 py-3 text-right font-mono tabular-nums text-muted-foreground lg:table-cell">
                      {{ formatTokens(item.image_output_tokens) }}
                    </td>
                  </tr>
                </template>
                <tr v-else>
                  <td colspan="7" class="px-4 py-12 text-center text-sm text-muted-foreground">
                    {{ t('leaderboard.emptyTitle') }}
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
          <div v-if="leaderboard.generated_at" class="border-t px-4 py-2 text-xs text-muted-foreground">
            {{ t('leaderboard.generatedAt') }} {{ formatTime(leaderboard.generated_at) }}
          </div>
        </div>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import { usageAPI, type LeaderboardResponse } from '@/api/usage'

const { t } = useI18n()

const loading = ref(false)
const error = ref(false)
const leaderboard = ref<LeaderboardResponse | null>(null)

async function load() {
  loading.value = true
  error.value = false
  try {
    leaderboard.value = await usageAPI.getDashboardLeaderboard(20)
  } catch {
    error.value = true
  } finally {
    loading.value = false
  }
}

onMounted(load)

function formatTokens(n: number): string {
  if (n >= 1_000_000_000) return (n / 1_000_000_000).toFixed(1) + 'B'
  if (n >= 1_000_000) return (n / 1_000_000).toFixed(1) + 'M'
  if (n >= 1_000) return (n / 1_000).toFixed(1) + 'K'
  return String(n)
}

function formatTime(iso: string): string {
  try {
    return new Date(iso).toLocaleString()
  } catch {
    return iso
  }
}

function rankBadgeClass(rank: number): string {
  if (rank === 1) return 'bg-yellow-400 text-yellow-900'
  if (rank === 2) return 'bg-slate-300 text-slate-800'
  if (rank === 3) return 'bg-amber-600 text-white'
  return 'bg-muted text-muted-foreground'
}
</script>
