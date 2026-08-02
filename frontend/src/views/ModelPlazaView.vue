<template>
  <!-- 后台内嵌形态:?embedded=1 且已登录,套完整后台布局 -->
  <AppLayout v-if="isEmbedded">
    <ModelPlazaContent :response="data" :loading="loading" :error="loadFailed" embedded />
  </AppLayout>

  <!-- 独立形态:自带导航条(logo/站名 + 登录/回后台) -->
  <div v-else class="relative min-h-screen bg-gray-50 dark:bg-dark-950 overflow-hidden">
    <!-- 背景装饰光晕: 模拟极光/氛围感 -->
    <div class="pointer-events-none absolute -top-48 -left-24 h-[500px] w-[500px] rounded-full bg-primary-500/5 blur-[120px] dark:bg-primary-900/5"></div>
    <div class="pointer-events-none absolute top-1/4 -right-48 h-[600px] w-[600px] rounded-full bg-purple-500/5 blur-[140px] dark:bg-purple-900/5"></div>
    <div class="pointer-events-none absolute bottom-0 left-1/4 h-[400px] w-[800px] rounded-full bg-blue-500/5 blur-[100px] dark:bg-blue-900/5"></div>

    <PlazaNavBar class="relative z-10" />
    <main class="relative z-10 mx-auto max-w-7xl px-4 py-8 sm:px-6 lg:px-8 lg:py-12">
      <ModelPlazaContent :response="data" :loading="loading" :error="loadFailed" />
    </main>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import PlazaNavBar from '@/components/modelPlaza/PlazaNavBar.vue'
import ModelPlazaContent from '@/components/modelPlaza/ModelPlazaContent.vue'
import { getModelPlaza, type ModelPlazaResponse } from '@/api/modelPlaza'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'

const route = useRoute()
const appStore = useAppStore()
const authStore = useAuthStore()

// embedded=1 但未登录(如转发的链接)自动降级为独立形态。
const isEmbedded = computed(() => route.query.embedded === '1' && authStore.isAuthenticated)

const data = ref<ModelPlazaResponse | null>(null)
const loading = ref(true)
const loadFailed = ref(false)

onMounted(async () => {
  // 独立形态导航条需要站点名/Logo;有 __APP_CONFIG__ 注入时同步命中缓存。
  void appStore.fetchPublicSettings()
  try {
    data.value = await getModelPlaza()
  } catch {
    loadFailed.value = true
  } finally {
    loading.value = false
  }
})
</script>
