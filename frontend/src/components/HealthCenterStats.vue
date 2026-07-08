<template>
  <v-row class="mb-4" dense>
    <v-col v-for="item in stateItems" :key="item.state" cols="6" sm="4" md="2">
      <v-card
        variant="tonal"
        :color="item.color"
        rounded="lg"
        class="pa-3 text-center"
      >
        <v-icon :color="item.color" size="28" class="mb-1">{{ item.icon }}</v-icon>
        <div class="text-h5 font-weight-bold">{{ item.count }}</div>
        <div class="text-caption text-medium-emphasis">{{ t(item.labelKey) }}</div>
      </v-card>
    </v-col>
  </v-row>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from '@/i18n'
import type { HealthCenterOverview } from '@/services/api-types'

const props = defineProps<{ overview: HealthCenterOverview }>()
const { t } = useI18n()

interface StateItem {
  state: string
  count: number
  color: string
  icon: string
  labelKey: string
}

const stateItems = computed<StateItem[]>(() => {
  const sc = props.overview.stateCounts
  return [
    { state: 'healthy', count: sc.healthy ?? 0, color: 'success', icon: 'mdi-heart-pulse', labelKey: 'healthCenter.state.healthy' },
    { state: 'degraded', count: sc.degraded ?? 0, color: 'warning', icon: 'mdi-alert', labelKey: 'healthCenter.state.degraded' },
    { state: 'limited', count: sc.limited ?? 0, color: 'orange', icon: 'mdi-alert-circle-outline', labelKey: 'healthCenter.state.limited' },
    { state: 'misconfigured', count: sc.misconfigured ?? 0, color: 'deep-purple', icon: 'mdi-shield-alert', labelKey: 'healthCenter.state.misconfigured' },
    { state: 'dead', count: sc.dead ?? 0, color: 'error', icon: 'mdi-close-circle', labelKey: 'healthCenter.state.dead' },
    { state: 'unknown', count: sc.unknown ?? 0, color: 'grey', icon: 'mdi-information', labelKey: 'healthCenter.state.unknown' },
  ]
})
</script>
