<script setup lang="ts">
import { computed } from 'vue'
import { AlertCircle, AlertTriangle, HeartPulse, HelpCircle, ShieldAlert, XCircle } from 'lucide-vue-next'
import { useLanguage } from '@/composables/useLanguage'
import type { HealthCenterOverview } from '@/services/admin-api'

const props = defineProps<{ overview: HealthCenterOverview }>()
const { t } = useLanguage()

interface StateItem {
  state: string
  count: number
  colorClass: string
  icon: typeof HeartPulse
  labelKey: string
}

const stateItems = computed<StateItem[]>(() => {
  const sc = props.overview.stateCounts || {}
  return [
    { state: 'healthy', count: sc.healthy ?? 0, colorClass: 'text-emerald-500 bg-emerald-500/10 border-emerald-500/30', icon: HeartPulse, labelKey: 'healthCenter.state.healthy' },
    { state: 'degraded', count: sc.degraded ?? 0, colorClass: 'text-amber-500 bg-amber-500/10 border-amber-500/30', icon: AlertTriangle, labelKey: 'healthCenter.state.degraded' },
    { state: 'limited', count: sc.limited ?? 0, colorClass: 'text-orange-500 bg-orange-500/10 border-orange-500/30', icon: AlertCircle, labelKey: 'healthCenter.state.limited' },
    { state: 'misconfigured', count: sc.misconfigured ?? 0, colorClass: 'text-purple-500 bg-purple-500/10 border-purple-500/30', icon: ShieldAlert, labelKey: 'healthCenter.state.misconfigured' },
    { state: 'dead', count: sc.dead ?? 0, colorClass: 'text-red-500 bg-red-500/10 border-red-500/30', icon: XCircle, labelKey: 'healthCenter.state.dead' },
    { state: 'unknown', count: sc.unknown ?? 0, colorClass: 'text-muted-foreground bg-muted/40 border-border/50', icon: HelpCircle, labelKey: 'healthCenter.state.unknown' },
  ]
})
</script>

<template>
  <div class="grid grid-cols-2 gap-2 sm:grid-cols-3 md:grid-cols-6">
    <div
      v-for="item in stateItems"
      :key="item.state"
      class="flex flex-col items-center gap-1 rounded-lg border px-3 py-3 text-center"
      :class="item.colorClass"
    >
      <component :is="item.icon" class="size-5" />
      <div class="text-xl font-bold">{{ item.count }}</div>
      <div class="text-[11px] text-muted-foreground">{{ t(item.labelKey) }}</div>
    </div>
  </div>
</template>
