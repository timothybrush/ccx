<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { AlertCircle, AlertTriangle, HeartPulse, HelpCircle, Loader2, ShieldAlert, XCircle } from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { useAdminApi } from '@/composables/useAdminApi'
import { useLanguage } from '@/composables/useLanguage'
import { healthCenterChannelEndpointsPath } from '@/services/admin-api'
import type { EndpointDetailItem, HealthState } from '@/services/admin-api'

const props = defineProps<{ channelUid: string }>()
const { t } = useLanguage()
const api = useAdminApi()

const endpoints = ref<EndpointDetailItem[]>([])
const loading = ref(true)

onMounted(async () => {
  try {
    const resp = await api.get<{ channelUid: string; endpoints: EndpointDetailItem[] }>(
      healthCenterChannelEndpointsPath(props.channelUid),
    )
    endpoints.value = resp.endpoints || []
  } catch {
    endpoints.value = []
  } finally {
    loading.value = false
  }
})

const evidenceEndpoints = computed(() =>
  endpoints.value.filter(ep => ep.healthEvidence || ep.suggestedAction),
)

const stateIconMap: Record<HealthState, typeof HeartPulse> = {
  healthy: HeartPulse,
  degraded: AlertTriangle,
  limited: AlertCircle,
  misconfigured: ShieldAlert,
  dead: XCircle,
  unknown: HelpCircle,
}

const stateBadgeClass: Record<HealthState, string> = {
  healthy: 'border-emerald-500/40 bg-emerald-500/10 text-emerald-500',
  degraded: 'border-amber-500/40 bg-amber-500/10 text-amber-500',
  limited: 'border-orange-500/40 bg-orange-500/10 text-orange-500',
  misconfigured: 'border-purple-500/40 bg-purple-500/10 text-purple-500',
  dead: 'border-red-500/40 bg-red-500/10 text-red-500',
  unknown: 'border-border/60 bg-muted/40 text-muted-foreground',
}

function formatPercent(value?: number): string {
  if (value == null) return '-'
  return (value * 100).toFixed(1) + '%'
}

function rateClass(rate?: number): string {
  if (rate == null) return ''
  if (rate >= 0.95) return 'text-emerald-500'
  if (rate >= 0.8) return 'text-amber-500'
  return 'text-red-500'
}
</script>

<template>
  <div class="border-t border-border/50 bg-muted/20 p-4">
    <div v-if="loading" class="flex items-center justify-center py-6 text-muted-foreground">
      <Loader2 class="size-5 animate-spin" />
    </div>

    <div v-else-if="endpoints.length === 0" class="py-4 text-center text-sm text-muted-foreground">
      {{ t('healthCenter.noChannels') }}
    </div>

    <div v-else class="space-y-3">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{{ t('healthCenter.col.status') }}</TableHead>
            <TableHead>{{ t('healthCenter.detail.baseUrl') }}</TableHead>
            <TableHead>{{ t('healthCenter.detail.keyHash') }}</TableHead>
            <TableHead>{{ t('healthCenter.detail.confidence') }}</TableHead>
            <TableHead>{{ t('healthCenter.detail.qualityTier') }}</TableHead>
            <TableHead>{{ t('healthCenter.detail.stabilityTier') }}</TableHead>
            <TableHead>{{ t('healthCenter.detail.speedTier') }}</TableHead>
            <TableHead>{{ t('healthCenter.detail.successRate15m') }}</TableHead>
            <TableHead>{{ t('healthCenter.detail.p95') }}</TableHead>
            <TableHead>{{ t('healthCenter.detail.firstByteP95') }}</TableHead>
            <TableHead>{{ t('healthCenter.detail.consecutiveFail') }}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          <TableRow v-for="ep in endpoints" :key="ep.endpointUid">
            <TableCell>
              <Badge variant="outline" class="gap-1" :class="stateBadgeClass[ep.healthState]">
                <component :is="stateIconMap[ep.healthState]" class="size-3" />
                {{ ep.healthState }}
              </Badge>
            </TableCell>
            <TableCell class="max-w-[220px] break-all text-xs text-muted-foreground">{{ ep.baseUrl }}</TableCell>
            <TableCell class="font-mono text-xs">{{ ep.keyHash }}</TableCell>
            <TableCell>{{ formatPercent(ep.healthConfidence) }}</TableCell>
            <TableCell>
              <Badge v-if="ep.qualityTier" variant="outline">{{ ep.qualityTier }}</Badge>
            </TableCell>
            <TableCell>
              <Badge v-if="ep.stabilityTier" variant="outline">{{ ep.stabilityTier }}</Badge>
            </TableCell>
            <TableCell>
              <Badge v-if="ep.speedTier" variant="outline">{{ ep.speedTier }}</Badge>
            </TableCell>
            <TableCell :class="rateClass(ep.successRate15m)">{{ formatPercent(ep.successRate15m) }}</TableCell>
            <TableCell>{{ ep.p95LatencyMs ? ep.p95LatencyMs.toFixed(0) + 'ms' : '-' }}</TableCell>
            <TableCell>
              {{ ep.p95FirstByteLatencyMs ? ep.p95FirstByteLatencyMs.toFixed(0) + 'ms' : '-' }}
              <span v-if="ep.firstByteSampleCount" class="text-xs text-muted-foreground">
                (n={{ ep.firstByteSampleCount }})
              </span>
            </TableCell>
            <TableCell>
              <span v-if="ep.consecutiveFail > 0" class="font-bold text-red-500">{{ ep.consecutiveFail }}</span>
              <span v-else>-</span>
            </TableCell>
          </TableRow>
        </TableBody>
      </Table>

      <!-- evidence / suggestedAction per endpoint -->
      <div v-if="evidenceEndpoints.length > 0" class="space-y-2">
        <div
          v-for="ep in evidenceEndpoints"
          :key="'evidence-' + ep.endpointUid"
          class="rounded-lg border border-border/50 bg-card/40 p-3 text-xs"
        >
          <div class="mb-1.5 flex items-center gap-2 font-medium">
            <component :is="stateIconMap[ep.healthState]" class="size-3.5" :class="stateBadgeClass[ep.healthState].split(' ').pop()" />
            <span class="font-mono">{{ ep.keyHash }}</span>
            <span class="text-muted-foreground">-- {{ ep.healthState }}</span>
          </div>
          <div v-if="ep.healthEvidence" class="mb-2">
            <div class="mb-0.5 font-semibold text-muted-foreground">{{ t('healthCenter.detail.evidence') }}</div>
            <div class="whitespace-pre-wrap text-foreground/90">{{ ep.healthEvidence }}</div>
          </div>
          <div v-if="ep.suggestedAction">
            <div class="mb-0.5 font-semibold text-muted-foreground">{{ t('healthCenter.detail.suggestedAction') }}</div>
            <Badge variant="secondary">{{ ep.suggestedAction }}</Badge>
          </div>
          <div v-if="ep.lastSuccessAt" class="mt-2 text-muted-foreground">
            {{ t('healthCenter.detail.lastSuccess') }}: {{ ep.lastSuccessAt }}
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
