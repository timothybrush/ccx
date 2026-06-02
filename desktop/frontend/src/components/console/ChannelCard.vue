<script setup lang="ts">
import { computed } from 'vue'
import type {
  Channel,
  ChannelMetrics,
  ChannelRecentActivity,
} from '@/services/admin-api'
import { useLanguage } from '@/composables/useLanguage'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import {
  Activity,
  AlertTriangle,
  Ban,
  CheckCircle2,
  Copy,
  Edit3,
  ExternalLink,
  Gauge,
  GripVertical,
  Key,
  MoreVertical,
  Pause,
  Play,
  RotateCcw,
  Sparkles,
  Terminal,
  Timer,
  Trash2,
  XCircle,
  Zap,
} from 'lucide-vue-next'

const props = withDefaults(defineProps<{
  channel: Channel
  metrics?: ChannelMetrics
  activity?: ChannelRecentActivity
  priority?: number
  inactive?: boolean
  supportsCapability?: boolean
}>(), {
  priority: 0,
  inactive: false,
  supportsCapability: true,
})

const emit = defineEmits<{
  edit: []
  delete: []
  ping: []
  logs: []
  capability: []
  status: []
  resume: []
  promote: []
  disable: []
  enable: []
}>()

const { tf } = useLanguage()

const isSuspended = computed(() => props.channel.status === 'suspended')
const isDisabled = computed(() => props.channel.status === 'disabled')
const isActive = computed(() => !isSuspended.value && !isDisabled.value)

const serviceTypeClass = computed(() => {
  const map: Record<string, string> = {
    openai: 'border-blue-500/25 bg-blue-500/10 text-blue-700 dark:text-blue-300',
    claude: 'border-orange-500/25 bg-orange-500/10 text-orange-700 dark:text-orange-300',
    gemini: 'border-purple-500/25 bg-purple-500/10 text-purple-700 dark:text-purple-300',
    responses: 'border-emerald-500/25 bg-emerald-500/10 text-emerald-700 dark:text-emerald-300',
  }
  return map[props.channel.serviceType] || map.openai
})

const statusConfig = computed(() => {
  if (isDisabled.value) {
    return {
      label: tf('console.channelStatus.disabled', 'Disabled'),
      icon: XCircle,
      class: 'border-rose-500/25 bg-rose-500/10 text-rose-700 dark:text-rose-300',
      dot: 'bg-rose-500',
    }
  }
  if (isSuspended.value) {
    return {
      label: tf('console.channelStatus.suspended', 'Suspended'),
      icon: Pause,
      class: 'border-amber-500/25 bg-amber-500/10 text-amber-700 dark:text-amber-300',
      dot: 'bg-amber-500',
    }
  }
  return {
    label: tf('console.channelStatus.active', 'Active'),
    icon: CheckCircle2,
    class: 'border-emerald-500/25 bg-emerald-500/10 text-emerald-700 dark:text-emerald-300',
    dot: 'bg-emerald-500',
  }
})

const circuitDisplay = computed(() => {
  const state = props.metrics?.circuitState
  if (state === 'open') {
    return {
      label: tf('console.circuit.open', 'Circuit Open'),
      class: 'border-rose-500/25 bg-rose-500/10 text-rose-700 dark:text-rose-300',
    }
  }
  if (state === 'half_open') {
    return {
      label: tf('console.circuit.halfOpen', 'Half-Open'),
      class: 'border-amber-500/25 bg-amber-500/10 text-amber-700 dark:text-amber-300',
    }
  }
  return null
})

const isPromoted = computed(() => {
  if (!props.channel.promotionUntil) return false
  return new Date(props.channel.promotionUntil).getTime() > Date.now()
})

const keyCount = computed(() => props.channel.apiKeys?.length ?? 0)
const disabledKeyCount = computed(() => props.channel.disabledApiKeys?.length ?? 0)

const latencyDisplay = computed(() => {
  const latency = props.metrics?.latency ?? props.channel.latency
  if (latency === undefined || latency === null || latency <= 0) return '—'
  return `${Math.round(latency)}ms`
})

const successRateDisplay = computed(() => {
  const raw = props.metrics?.successRate
  if (raw === undefined || raw === null) return '—'
  const value = raw <= 1 ? raw * 100 : raw
  return `${value.toFixed(0)}%`
})

const requestDisplay = computed(() => {
  const value = props.metrics?.requestCount ?? 0
  if (value >= 1_000_000) return `${(value / 1_000_000).toFixed(1)}M`
  if (value >= 1_000) return `${(value / 1_000).toFixed(1)}K`
  return `${value}`
})

const rpmDisplay = computed(() => {
  const rpm = props.activity?.rpm ?? props.channel.rpm ?? 0
  if (!rpm) return '—'
  return rpm >= 10 ? rpm.toFixed(0) : rpm.toFixed(1)
})

const tpmDisplay = computed(() => {
  const tpm = props.activity?.tpm ?? 0
  if (!tpm) return '—'
  if (tpm >= 1_000_000) return `${(tpm / 1_000_000).toFixed(1)}M`
  if (tpm >= 1_000) return `${(tpm / 1_000).toFixed(1)}K`
  return tpm.toFixed(0)
})

const websiteUrl = computed(() => {
  if (props.channel.website) return props.channel.website
  try {
    const url = new URL(props.channel.baseUrl)
    return `${url.protocol}//${url.host}`
  } catch {
    return ''
  }
})

const baseUrlText = computed(() => {
  if (props.channel.baseUrls?.length) return props.channel.baseUrls.join(' · ')
  return props.channel.baseUrl || '—'
})

async function copyChannelInfo() {
  const lines = [
    ...(props.channel.baseUrls?.length ? props.channel.baseUrls : [props.channel.baseUrl]),
    ...(props.channel.apiKeys ?? []),
  ].map(item => item?.trim()).filter(Boolean)
  await navigator.clipboard?.writeText(lines.join('\n'))
}
</script>

<template>
  <div
    :class="[
      'group grid grid-cols-[42px_minmax(220px,1fr)_120px_96px_96px_88px_150px] items-center gap-3 border px-3 py-2.5 transition-all duration-200',
      'bg-card/75 hover:bg-card dark:bg-card/55 dark:hover:bg-card/80',
      inactive ? 'border-dashed border-border/80 opacity-80' : 'border-border',
      circuitDisplay ? 'ring-1 ring-rose-500/20' : '',
    ]"
  >
    <div class="flex items-center gap-2 text-muted-foreground">
      <GripVertical class="h-4 w-4 cursor-grab opacity-60 group-hover:opacity-100" />
      <span class="min-w-5 text-right font-mono text-[11px] font-bold text-foreground/70">
        {{ priority || '—' }}
      </span>
    </div>

    <div class="min-w-0 space-y-1">
      <div class="flex min-w-0 items-center gap-2">
        <span
          class="h-2 w-2 shrink-0 rounded-full shadow-[0_0_8px_currentColor]"
          :class="statusConfig.dot"
        />
        <span class="truncate text-sm font-semibold text-foreground">
          {{ channel.name }}
        </span>
        <Badge class="shrink-0 border text-[10px] uppercase" :class="serviceTypeClass">
          {{ channel.serviceType }}
        </Badge>
        <span
          v-if="isPromoted"
          class="inline-flex shrink-0 items-center gap-1 border border-purple-500/25 bg-purple-500/10 px-1.5 py-0.5 text-[10px] font-bold text-purple-700 dark:text-purple-300"
        >
          <Sparkles class="h-3 w-3" />
          PROMO
        </span>
        <span
          v-if="circuitDisplay"
          class="inline-flex shrink-0 items-center gap-1 border px-1.5 py-0.5 text-[10px] font-bold"
          :class="circuitDisplay.class"
        >
          <AlertTriangle class="h-3 w-3" />
          {{ circuitDisplay.label }}
        </span>
      </div>
      <div class="truncate font-mono text-[11px] text-muted-foreground" :title="baseUrlText">
        {{ baseUrlText }}
      </div>
      <div v-if="channel.description" class="truncate text-[11px] text-muted-foreground" :title="channel.description">
        {{ channel.description }}
      </div>
    </div>

    <div class="inline-flex items-center justify-center gap-1.5 border px-2 py-1 text-[11px] font-semibold" :class="statusConfig.class">
      <component :is="statusConfig.icon" class="h-3.5 w-3.5" />
      {{ statusConfig.label }}
    </div>

    <div class="space-y-0.5 text-right font-mono">
      <div class="text-xs font-bold text-foreground">{{ latencyDisplay }}</div>
      <div class="text-[10px] text-muted-foreground">LATENCY</div>
    </div>

    <div class="space-y-0.5 text-right font-mono">
      <div class="text-xs font-bold text-foreground">{{ rpmDisplay }} / {{ tpmDisplay }}</div>
      <div class="text-[10px] text-muted-foreground">RPM / TPM</div>
    </div>

    <div class="space-y-0.5 text-right font-mono">
      <div class="text-xs font-bold text-foreground">{{ requestDisplay }}</div>
      <div class="text-[10px] text-muted-foreground">{{ successRateDisplay }}</div>
    </div>

    <div class="flex items-center justify-end gap-1.5">
      <Button variant="outline" size="sm" @click="emit('ping')">
        <Gauge class="h-3.5 w-3.5" />
        {{ tf('console.actions.ping', 'Ping') }}
      </Button>
      <Button v-if="isDisabled" variant="outline" size="sm" @click="emit('enable')">
        <Play class="h-3.5 w-3.5" />
        {{ tf('console.actions.enable', 'Enable') }}
      </Button>
      <Button v-else-if="isSuspended" variant="outline" size="sm" @click="emit('status')">
        <Play class="h-3.5 w-3.5" />
        {{ tf('console.actions.resume', 'Resume') }}
      </Button>
      <Button v-else variant="outline" size="sm" @click="emit('status')">
        <Pause class="h-3.5 w-3.5" />
        {{ tf('console.actions.suspend', 'Suspend') }}
      </Button>

      <DropdownMenu>
        <DropdownMenuTrigger as-child>
          <Button variant="ghost" size="icon-sm">
            <MoreVertical class="h-4 w-4" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" class="w-52">
          <DropdownMenuLabel>{{ tf('console.actions.label', 'Actions') }}</DropdownMenuLabel>
          <DropdownMenuSeparator />
          <DropdownMenuGroup>
            <DropdownMenuItem @click="emit('edit')">
              <Edit3 class="h-4 w-4" />
              {{ tf('console.actions.edit', 'Edit Channel') }}
            </DropdownMenuItem>
            <DropdownMenuItem v-if="supportsCapability" @click="emit('capability')">
              <Zap class="h-4 w-4" />
              {{ tf('console.actions.capability', 'Capability Test') }}
            </DropdownMenuItem>
            <DropdownMenuItem @click="emit('logs')">
              <Terminal class="h-4 w-4" />
              {{ tf('console.actions.logs', 'View Logs') }}
            </DropdownMenuItem>
            <DropdownMenuItem @click="copyChannelInfo">
              <Copy class="h-4 w-4" />
              {{ tf('console.actions.copy', 'Copy Config') }}
            </DropdownMenuItem>
            <DropdownMenuItem v-if="websiteUrl" as="a" :href="websiteUrl" target="_blank" rel="noopener">
              <ExternalLink class="h-4 w-4" />
              {{ tf('console.actions.website', 'Visit Website') }}
            </DropdownMenuItem>
          </DropdownMenuGroup>

          <DropdownMenuSeparator />
          <DropdownMenuGroup>
            <DropdownMenuItem v-if="circuitDisplay" @click="emit('resume')">
              <RotateCcw class="h-4 w-4" />
              {{ tf('console.actions.resetCircuit', 'Reset Circuit Breaker') }}
            </DropdownMenuItem>
            <DropdownMenuItem v-if="!isPromoted && !isDisabled" @click="emit('promote')">
              <Sparkles class="h-4 w-4" />
              {{ tf('console.actions.promote', 'Promote') }}
            </DropdownMenuItem>
            <DropdownMenuItem v-if="!isDisabled" @click="emit('disable')">
              <Ban class="h-4 w-4" />
              {{ tf('console.actions.disable', 'Disable') }}
            </DropdownMenuItem>
          </DropdownMenuGroup>

          <DropdownMenuSeparator />
          <div class="px-2 py-1 text-[10px] text-muted-foreground">
            <Key class="mr-1 inline h-3 w-3" />
            {{ keyCount }} {{ tf('console.keys.active', 'active keys') }}
            <span v-if="disabledKeyCount"> · {{ disabledKeyCount }} disabled</span>
          </div>
          <DropdownMenuItem variant="destructive" @click="emit('delete')">
            <Trash2 class="h-4 w-4" />
            {{ tf('console.actions.delete', 'Delete Channel') }}
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  </div>
</template>
