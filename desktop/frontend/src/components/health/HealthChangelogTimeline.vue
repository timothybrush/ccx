<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { Radar, RefreshCw, Sparkles, WifiOff } from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { connectHealthChangelogEvents, fetchHealthChangelog, type ProfileEventsConnectionStatus } from '@/composables/useHealthChangelog'
import { useLanguage } from '@/composables/useLanguage'
import type { ProfileChangeEvent, ProfileChangeEventType } from '@/services/admin-api'

const { t } = useLanguage()

// 最多渲染的事件数量：避免长时间挂着页面导致 DOM 无限增长
const MAX_RENDERED_EVENTS = 200

const events = ref<ProfileChangeEvent[]>([])
const status = ref<ProfileEventsConnectionStatus>('connecting')

let disconnect: (() => void) | null = null

function eventIconFor(type: ProfileChangeEventType) {
  switch (type) {
    case 'health_changed': return RefreshCw
    case 'discovery_completed': return Radar
    case 'auto_mapping_applied': return Sparkles
    default: return RefreshCw
  }
}

function eventColorClass(type: ProfileChangeEventType): string {
  switch (type) {
    case 'health_changed': return 'text-amber-500'
    case 'discovery_completed': return 'text-sky-500'
    case 'auto_mapping_applied': return 'text-primary'
    default: return 'text-muted-foreground'
  }
}

function eventTypeLabel(type: ProfileChangeEventType): string {
  switch (type) {
    case 'health_changed': return t('healthCenter.changelog.type.healthChanged')
    case 'discovery_completed': return t('healthCenter.changelog.type.discoveryCompleted')
    case 'auto_mapping_applied': return t('healthCenter.changelog.type.autoMappingApplied')
    default: return t('healthCenter.changelog.type.profileUpdated')
  }
}

function relativeTime(iso: string): string {
  const then = new Date(iso).getTime()
  if (Number.isNaN(then)) return ''
  const diffSec = Math.max(0, Math.floor((Date.now() - then) / 1000))
  if (diffSec < 60) return `${diffSec}s`
  const diffMin = Math.floor(diffSec / 60)
  if (diffMin < 60) return `${diffMin}m`
  const diffHour = Math.floor(diffMin / 60)
  if (diffHour < 24) return `${diffHour}h`
  return `${Math.floor(diffHour / 24)}d`
}

function prependEvent(event: ProfileChangeEvent) {
  events.value = [event, ...events.value].slice(0, MAX_RENDERED_EVENTS)
}

async function loadHistory() {
  try {
    const resp = await fetchHealthChangelog({ limit: 50 })
    events.value = resp.events || []
  } catch {
    // 历史拉取失败不阻塞实时连接，保持空列表即可
  }
}

onMounted(() => {
  loadHistory()
  disconnect = connectHealthChangelogEvents({
    onEvent: prependEvent,
    onStatusChange: (s) => {
      status.value = s
    },
  })
})

onBeforeUnmount(() => {
  disconnect?.()
})
</script>

<template>
  <div class="rounded-xl border border-border/60 bg-card/40">
    <div class="flex items-center justify-between border-b border-border/50 px-4 py-2.5">
      <span class="text-sm font-semibold">{{ t('healthCenter.changelog.title') }}</span>
      <Badge v-if="status === 'open'" variant="outline" class="gap-1 border-emerald-500/40 bg-emerald-500/10 text-emerald-500">
        <span class="size-1.5 rounded-full bg-emerald-500" />
        {{ t('healthCenter.changelog.live') }}
      </Badge>
      <Badge v-else-if="status === 'connecting'" variant="outline" class="text-muted-foreground">
        {{ t('healthCenter.changelog.connecting') }}
      </Badge>
      <Badge v-else variant="outline" class="gap-1 border-red-500/40 bg-red-500/10 text-red-500">
        <WifiOff class="size-3" />
        {{ t('healthCenter.changelog.disconnected') }}
      </Badge>
    </div>

    <div class="max-h-80 overflow-y-auto">
      <div v-if="events.length === 0" class="py-8 text-center text-sm text-muted-foreground">
        {{ t('healthCenter.changelog.empty') }}
      </div>

      <ul v-else class="divide-y divide-border/40">
        <li
          v-for="event in events"
          :key="event.eventUid"
          class="flex items-start gap-2.5 px-4 py-2.5"
        >
          <component :is="eventIconFor(event.eventType)" class="mt-0.5 size-4 shrink-0" :class="eventColorClass(event.eventType)" />
          <div class="min-w-0 flex-1">
            <div class="text-xs font-medium">
              {{ eventTypeLabel(event.eventType) }}
              <span class="text-muted-foreground">— {{ event.channelUid }}</span>
            </div>
            <div class="truncate text-xs text-muted-foreground">{{ event.summary }}</div>
          </div>
          <span class="shrink-0 text-[11px] text-muted-foreground">{{ relativeTime(event.createdAt) }}</span>
        </li>
      </ul>
    </div>
  </div>
</template>
