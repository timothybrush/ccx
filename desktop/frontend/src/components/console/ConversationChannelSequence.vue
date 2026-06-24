<script setup lang="ts">
import { ArrowDown } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'

interface ChannelInfo {
  index: number
  name: string
  status: string
  priority?: number
  circuitOpen?: boolean
}

const props = withDefaults(defineProps<{
  channels: ChannelInfo[]
  currentChannel?: number
  nextChannel?: number
  nextChannelCircuitOpen?: boolean
  overrideActive?: boolean
}>(), {
  currentChannel: undefined,
  nextChannel: undefined,
  nextChannelCircuitOpen: false,
  overrideActive: false,
})

const emit = defineEmits<{
  moveToTop: [channel: ChannelInfo, index: number]
  demote: [index: number]
}>()

function isDemoted(index: number): boolean {
  return props.overrideActive && index >= props.channels.length - 1
}

function badgeClass(channel: ChannelInfo): string {
  if (channel.index === props.currentChannel) return 'bg-primary text-primary-foreground'
  if (channel.index === props.nextChannel) {
    return props.nextChannelCircuitOpen ? 'bg-red-500 text-white' : 'bg-emerald-500 text-white'
  }
  return 'border border-border bg-muted/30 text-muted-foreground'
}
</script>

<template>
  <div class="channel-sequence" @click.stop>
    <div
      v-for="(channel, index) in channels"
      :key="channel.index"
      class="channel-item channel-item-animated flex items-center gap-1 border-b border-border/60 px-2 py-1.5 last:border-b-0"
      :class="{ demoted: isDemoted(index) }"
      :style="{ animationDelay: `${Math.min(index, 12) * 35}ms` }"
    >
      <span class="seq-num">{{ String(index + 1).padStart(2, '0') }}</span>
      <span class="seq-arrow">→</span>
      <button
        type="button"
        class="channel-name min-w-0 flex-1 truncate text-left text-xs"
        @click.stop="emit('moveToTop', channel, index)"
      >
        {{ channel.name }}
      </button>
      <span
        v-if="channel.index === currentChannel"
        class="sequence-badge"
        :class="badgeClass(channel)"
      >
        CURRENT
      </span>
      <span
        v-else-if="channel.index === nextChannel"
        class="sequence-badge next-channel-badge"
        :class="badgeClass(channel)"
      >
        {{ nextChannelCircuitOpen ? 'TRIPPED' : 'NEXT' }}
      </span>
      <span
        v-if="channel.status === 'suspended'"
        class="sequence-badge fused-chip text-white"
      >
        PAUSED
      </span>
      <span
        v-if="channel.circuitOpen"
        class="sequence-badge bg-red-500 text-white"
      >
        TRIPPED
      </span>
      <Button
        variant="ghost"
        size="icon-sm"
        class="h-6 w-6"
        :disabled="index === channels.length - 1"
        @click.stop="emit('demote', index)"
      >
        <ArrowDown class="h-3.5 w-3.5" />
      </Button>
    </div>
  </div>
</template>

<style scoped>
.channel-sequence {
  max-height: calc(8 * 36px);
  overflow-x: hidden;
  overflow-y: auto;
  border: 1px solid var(--color-border);
  border-radius: 0;
  overscroll-behavior: auto;
}

.channel-item-animated {
  animation: ccx-slide-in 0.18s ease both;
}

.channel-item.demoted {
  opacity: 0.5;
}

.seq-num {
  min-width: 2.5ch;
  font-variant-numeric: tabular-nums;
  font-size: 10px;
  font-weight: 700;
  opacity: 0.5;
}

.seq-arrow {
  font-size: 10px;
  opacity: 0.35;
}

.channel-name:hover {
  color: var(--color-primary);
  text-decoration: underline;
}

.sequence-badge {
  display: inline-flex;
  align-items: center;
  padding: 1px 6px;
  border-radius: 0;
  font-size: 9px;
  font-weight: 800;
  letter-spacing: 0.05em;
}

.next-channel-badge {
  animation: ccx-breathe 2s ease-in-out infinite;
}

.fused-chip {
  border: none;
  background: repeating-linear-gradient(
    -45deg,
    rgba(239, 68, 68, 0.9) 0,
    rgba(239, 68, 68, 0.9) 4px,
    rgba(127, 29, 29, 0.9) 4px,
    rgba(127, 29, 29, 0.9) 8px
  );
}

@keyframes ccx-breathe {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.55; }
}

@keyframes ccx-slide-in {
  from { opacity: 0; transform: translateX(-6px); }
  to { opacity: 1; transform: translateX(0); }
}
</style>
