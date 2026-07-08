<template>
  <div class="health-center">
    <!-- Header -->
    <div class="d-flex align-center justify-space-between mb-4">
      <div class="d-flex align-center">
        <v-icon size="28" class="mr-2" color="primary">mdi-stethoscope</v-icon>
        <span class="text-h5 font-weight-bold">{{ t('healthCenter.title') }}</span>
      </div>
      <v-btn
        variant="tonal"
        prepend-icon="mdi-refresh"
        :loading="loading"
        @click="fetchAll"
      >
        {{ t('app.actions.refresh') }}
      </v-btn>
    </div>

    <!-- Overview stats -->
    <HealthCenterStats v-if="overview" :overview="overview" class="mb-2" />

    <!-- Summary line -->
    <div v-if="overview" class="d-flex ga-4 text-caption text-medium-emphasis mb-4">
      <span>{{ t('healthCenter.totalChannels') }}: {{ overview.totalChannels }}</span>
      <span>{{ t('healthCenter.totalEndpoints') }}: {{ overview.totalEndpoints }}</span>
    </div>

    <!-- Loading state -->
    <div v-if="loading && !overview" class="text-center py-12">
      <v-progress-circular indeterminate color="primary" size="48" />
    </div>

    <!-- Channel table -->
    <HealthChannelTable v-else-if="overview" :channels="channels" />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from '@/i18n'
import { api } from '@/services/api'
import HealthCenterStats from '@/components/HealthCenterStats.vue'
import HealthChannelTable from '@/components/HealthChannelTable.vue'
import type { HealthCenterOverview, ChannelHealthItem } from '@/services/api-types'

const { t } = useI18n()

const overview = ref<HealthCenterOverview | null>(null)
const channels = ref<ChannelHealthItem[]>([])
const loading = ref(true)

async function fetchAll() {
  loading.value = true
  try {
    const [ov, ch] = await Promise.all([
      api.getHealthCenterOverview(),
      api.getHealthCenterChannels(),
    ])
    overview.value = ov
    channels.value = ch.channels
  } finally {
    loading.value = false
  }
}

onMounted(fetchAll)
</script>
