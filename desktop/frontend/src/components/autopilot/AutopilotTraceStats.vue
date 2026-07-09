<script setup lang="ts">
import { computed } from 'vue'
import { Badge } from '@/components/ui/badge'
import { useLanguage } from '@/composables/useLanguage'
import type { AutopilotTraceStats } from '@/services/admin-api'

const props = defineProps<{ stats: AutopilotTraceStats }>()
const { t } = useLanguage()

const mismatchRateDisplay = computed(() => {
  if (props.stats.totalCount === 0) return '-'
  return `${(props.stats.mismatchRate * 100).toFixed(1)}%`
})

const mismatchRateColorClass = computed(() => {
  const rate = props.stats.mismatchRate
  if (props.stats.totalCount === 0) return 'text-muted-foreground bg-muted/40 border-border/50'
  if (rate <= 0.05) return 'text-emerald-500 bg-emerald-500/10 border-emerald-500/30'
  if (rate <= 0.15) return 'text-amber-500 bg-amber-500/10 border-amber-500/30'
  return 'text-red-500 bg-red-500/10 border-red-500/30'
})

const modeDistItems = computed(() => {
  const dist = props.stats.modeDist
  if (!dist) return []
  return Object.entries(dist)
    .map(([mode, count]) => ({ mode, count }))
    .sort((a, b) => b.count - a.count)
})

const taskClassDistItems = computed(() => {
  const dist = props.stats.taskClassDist
  if (!dist) return []
  return Object.entries(dist)
    .map(([taskClass, count]) => ({ taskClass, count }))
    .sort((a, b) => b.count - a.count)
})
</script>

<template>
  <div class="rounded-xl border border-border/60 bg-card/40 p-4">
    <h4 class="mb-3 text-sm font-bold">{{ t('autopilot.traceStats.title') }}</h4>

    <div class="mb-3 grid grid-cols-3 gap-2">
      <div class="rounded-lg border border-border/50 bg-muted/20 px-3 py-3 text-center">
        <div class="text-xl font-bold">{{ stats.totalCount }}</div>
        <div class="text-[11px] text-muted-foreground">{{ t('autopilot.traceStats.total') }}</div>
      </div>
      <div class="rounded-lg border border-border/50 bg-muted/20 px-3 py-3 text-center">
        <div class="text-xl font-bold">{{ stats.mismatchCount }}</div>
        <div class="text-[11px] text-muted-foreground">{{ t('autopilot.traceStats.mismatches') }}</div>
      </div>
      <div class="rounded-lg border px-3 py-3 text-center" :class="mismatchRateColorClass">
        <div class="text-xl font-bold">{{ mismatchRateDisplay }}</div>
        <div class="text-[11px] text-muted-foreground">{{ t('autopilot.traceStats.mismatchRate') }}</div>
      </div>
    </div>

    <div v-if="modeDistItems.length > 0" class="mb-3">
      <div class="mb-2 text-xs text-muted-foreground">{{ t('autopilot.traceStats.modeDistribution') }}</div>
      <div class="flex flex-wrap gap-2">
        <Badge v-for="item in modeDistItems" :key="item.mode" variant="secondary">
          {{ t(`autopilot.mode.${item.mode}`) || item.mode }}: {{ item.count }}
        </Badge>
      </div>
    </div>

    <div v-if="taskClassDistItems.length > 0">
      <div class="mb-2 text-xs text-muted-foreground">{{ t('autopilot.traceStats.taskClassDistribution') }}</div>
      <div class="flex flex-wrap gap-2">
        <Badge v-for="item in taskClassDistItems" :key="item.taskClass" variant="outline">
          {{ item.taskClass }}: {{ item.count }}
        </Badge>
      </div>
    </div>
  </div>
</template>
