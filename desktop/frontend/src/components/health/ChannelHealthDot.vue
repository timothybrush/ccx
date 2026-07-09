<script setup lang="ts">
import { computed } from 'vue'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { useLanguage } from '@/composables/useLanguage'
import type { ChannelHealthItem } from '@/services/admin-api'

const props = defineProps<{
  health?: ChannelHealthItem | null
}>()

const { t } = useLanguage()

const stateLabel = computed(() => {
  const state = props.health?.aggState || 'unknown'
  return t(`healthCenter.state.${state}`)
})

const healthyCount = computed(() => props.health?.healthyCount ?? 0)

// Inconsistent = endpoints that are NOT healthy (degraded + limited + dead + unknown)
const warningCount = computed(() => {
  const h = props.health
  if (!h) return 0
  return h.degradedCount + h.limitedCount + h.deadCount + h.unknownCount
})

const dotClass = computed(() => {
  const map: Record<string, string> = {
    healthy: 'bg-emerald-500 shadow-[0_0_4px_rgba(16,185,129,0.6)]',
    degraded: 'bg-amber-500 shadow-[0_0_4px_rgba(245,158,11,0.6)]',
    limited: 'bg-orange-500 shadow-[0_0_4px_rgba(249,115,22,0.6)]',
    misconfigured: 'bg-purple-500 shadow-[0_0_4px_rgba(168,85,247,0.6)]',
    dead: 'bg-red-500 shadow-[0_0_4px_rgba(239,68,68,0.6)] animate-pulse',
    unknown: 'bg-muted-foreground/60',
  }
  return map[props.health?.aggState || 'unknown'] ?? map.unknown
})
</script>

<template>
  <Tooltip v-if="health">
    <TooltipTrigger as-child>
      <span class="inline-flex size-[18px] shrink-0 cursor-help items-center justify-center rounded-full">
        <span class="size-2.5 rounded-full" :class="dotClass" />
      </span>
    </TooltipTrigger>
    <TooltipContent side="top" class="min-w-[140px]">
      <div class="mb-1 text-xs font-bold">
        {{ t('channelHealth.stateLabel') }}: {{ stateLabel }}
      </div>
      <div class="mb-1 text-xs text-muted-foreground">
        {{ healthyCount }}/{{ health.endpointCount }} {{ t('channelHealth.endpointsHealthy') }}
      </div>
      <div v-if="health.avgSuccessRate != null" class="mb-1 text-xs text-muted-foreground">
        {{ t('channelHealth.avgSuccessRate') }}: {{ (health.avgSuccessRate * 100).toFixed(1) }}%
      </div>
      <div v-if="warningCount > 0" class="mt-1 flex items-center gap-1 text-xs text-amber-500">
        {{ t('channelHealth.inconsistentWarning', { count: String(warningCount) }) }}
      </div>
    </TooltipContent>
  </Tooltip>
</template>
