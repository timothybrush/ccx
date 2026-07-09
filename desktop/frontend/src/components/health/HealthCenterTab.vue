<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { Loader2, RefreshCw } from 'lucide-vue-next'
import { Alert } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { useAdminApi } from '@/composables/useAdminApi'
import { useLanguage } from '@/composables/useLanguage'
import {
  HEALTH_CENTER_CHANNELS_PATH,
  HEALTH_CENTER_OVERVIEW_PATH,
} from '@/services/admin-api'
import type { ChannelHealthItem, HealthCenterOverview } from '@/services/admin-api'
import HealthCenterStats from './HealthCenterStats.vue'
import HealthChangelogTimeline from './HealthChangelogTimeline.vue'
import HealthChannelTable from './HealthChannelTable.vue'

const { t } = useLanguage()
const api = useAdminApi()

const overview = ref<HealthCenterOverview | null>(null)
const channels = ref<ChannelHealthItem[]>([])
const loading = ref(true)
const errorMessage = ref('')
let refreshTimer: ReturnType<typeof setInterval> | undefined

async function loadData() {
  errorMessage.value = ''
  try {
    const [overviewResp, channelsResp] = await Promise.all([
      api.get<HealthCenterOverview>(HEALTH_CENTER_OVERVIEW_PATH),
      api.get<{ channels: ChannelHealthItem[] }>(HEALTH_CENTER_CHANNELS_PATH),
    ])
    overview.value = overviewResp
    channels.value = channelsResp.channels || []
  } catch (err) {
    errorMessage.value = err instanceof Error ? err.message : String(err)
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  loadData()
  refreshTimer = setInterval(loadData, 30000)
})

onBeforeUnmount(() => {
  if (refreshTimer) clearInterval(refreshTimer)
})
</script>

<template>
  <div class="mx-auto flex w-full max-w-[1680px] flex-col gap-4">
    <div class="flex items-center justify-between">
      <h3 class="text-lg font-bold">{{ t('healthCenter.title') }}</h3>
      <Button variant="outline" size="sm" :disabled="loading" @click="loadData">
        <Loader2 v-if="loading" class="size-4 animate-spin" />
        <RefreshCw v-else class="size-4" />
      </Button>
    </div>

    <Alert v-if="errorMessage" variant="destructive">
      <p class="text-sm">{{ errorMessage }}</p>
    </Alert>

    <div v-if="loading && !overview" class="flex items-center justify-center py-16 text-muted-foreground">
      <Loader2 class="size-6 animate-spin" />
    </div>

    <template v-else>
      <div class="grid grid-cols-2 gap-3 sm:grid-cols-4">
        <div class="rounded-lg border border-border/60 bg-card/40 px-4 py-3">
          <div class="text-2xl font-bold">{{ overview?.totalChannels ?? 0 }}</div>
          <div class="text-xs text-muted-foreground">{{ t('healthCenter.totalChannels') }}</div>
        </div>
        <div class="rounded-lg border border-border/60 bg-card/40 px-4 py-3">
          <div class="text-2xl font-bold">{{ overview?.totalEndpoints ?? 0 }}</div>
          <div class="text-xs text-muted-foreground">{{ t('healthCenter.totalEndpoints') }}</div>
        </div>
      </div>

      <HealthCenterStats v-if="overview" :overview="overview" />

      <div class="grid grid-cols-1 gap-4 lg:grid-cols-[2fr_1fr]">
        <HealthChannelTable :channels="channels" />
        <HealthChangelogTimeline />
      </div>
    </template>
  </div>
</template>
