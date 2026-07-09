<script setup lang="ts">
import { ref } from 'vue'
import { ChevronDown, ChevronRight } from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { useLanguage } from '@/composables/useLanguage'
import ChannelHealthDot from './ChannelHealthDot.vue'
import HealthChannelDetail from './HealthChannelDetail.vue'
import type { ChannelHealthItem } from '@/services/admin-api'

defineProps<{ channels: ChannelHealthItem[] }>()
const { t } = useLanguage()

const expanded = ref<Set<string>>(new Set())

function toggle(channelUid: string) {
  const next = new Set(expanded.value)
  if (next.has(channelUid)) {
    next.delete(channelUid)
  } else {
    next.add(channelUid)
  }
  expanded.value = next
}

function originBadgeLabel(tier?: string): string {
  switch (tier) {
    case 'first': return t('channelHealth.originOfficial')
    case 'second': return t('channelHealth.originRelay')
    case 'third': return t('channelHealth.originCommunity')
    default: return t('channelHealth.originUnknown')
  }
}

function poolBadgeLabel(pool?: string): string | null {
  if (pool === 'free') return t('channelHealth.poolFree')
  if (pool === 'temp') return t('channelHealth.poolTemp')
  return null
}

function formatPercent(value?: number): string {
  if (value == null) return '-'
  return (value * 100).toFixed(1) + '%'
}
</script>

<template>
  <div class="overflow-hidden rounded-xl border border-border/60">
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead class="w-8" />
          <TableHead>{{ t('healthCenter.col.status') }}</TableHead>
          <TableHead>{{ t('healthCenter.col.kind') }}</TableHead>
          <TableHead>{{ t('healthCenter.col.channel') }}</TableHead>
          <TableHead>{{ t('healthCenter.col.endpoints') }}</TableHead>
          <TableHead>{{ t('healthCenter.col.successRate') }}</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        <template v-if="channels.length === 0">
          <TableRow>
            <TableCell colspan="6" class="py-8 text-center text-sm text-muted-foreground">
              {{ t('healthCenter.noChannels') }}
            </TableCell>
          </TableRow>
        </template>
        <template v-for="channel in channels" :key="channel.channelUid">
          <TableRow
            class="cursor-pointer"
            @click="toggle(channel.channelUid)"
          >
            <TableCell>
              <component :is="expanded.has(channel.channelUid) ? ChevronDown : ChevronRight" class="size-4 text-muted-foreground" />
            </TableCell>
            <TableCell>
              <ChannelHealthDot :health="channel" />
            </TableCell>
            <TableCell>
              <Badge variant="outline" class="uppercase">{{ t(`healthCenter.kind.${channel.channelKind}`) || channel.channelKind }}</Badge>
            </TableCell>
            <TableCell>
              <div class="flex flex-col gap-1">
                <span class="font-medium">{{ channel.channelName || channel.channelUid }}</span>
                <div class="flex items-center gap-1.5">
                  <Badge variant="secondary" class="text-[10px]">{{ originBadgeLabel(channel.originTier) }}</Badge>
                  <Badge v-if="poolBadgeLabel(channel.poolTag)" variant="outline" class="text-[10px]">
                    {{ poolBadgeLabel(channel.poolTag) }}
                  </Badge>
                </div>
              </div>
            </TableCell>
            <TableCell>{{ channel.healthyCount }}/{{ channel.endpointCount }}</TableCell>
            <TableCell>{{ formatPercent(channel.avgSuccessRate) }}</TableCell>
          </TableRow>
          <TableRow v-if="expanded.has(channel.channelUid)">
            <TableCell colspan="6" class="p-0">
              <HealthChannelDetail :channel-uid="channel.channelUid" />
            </TableCell>
          </TableRow>
        </template>
      </TableBody>
    </Table>
  </div>
</template>
