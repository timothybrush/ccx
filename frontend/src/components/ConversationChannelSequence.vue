<template>
  <div class="channel-sequence" @click.stop>
    <div
      v-for="(ch, i) in channels"
      :key="ch.index"
      :class="['channel-item d-flex align-center pa-1 channel-item-animated', { demoted: isDemoted(i) }]"
      :style="{ animationDelay: `${Math.min(i, 12) * 35}ms` }"
    >
      <span class="seq-num">{{ String(i + 1).padStart(2, '0') }}</span>
      <span class="seq-arrow">&rarr;</span>
      <span class="text-caption flex-grow-1 channel-name" @click.stop="emit('moveToTop', ch, i)">{{ ch.name }}</span>
      <v-chip v-if="ch.index === currentChannel" size="x-small" color="primary" variant="flat" class="mr-1">CURRENT</v-chip>
      <v-chip
        v-else-if="ch.index === nextChannel"
        size="x-small"
        :color="nextChannelCircuitOpen ? 'error' : 'success'"
        variant="flat"
        class="next-channel-chip mr-1"
      >
        {{ nextChannelCircuitOpen ? 'TRIPPED' : 'NEXT' }}
      </v-chip>
      <v-chip v-if="ch.status === 'suspended'" size="x-small" variant="flat" class="fused-chip mr-1">PAUSED</v-chip>
      <v-chip v-if="ch.circuitOpen" size="x-small" color="error" variant="tonal" class="mr-1">TRIPPED</v-chip>
      <v-btn icon size="x-small" variant="text" :disabled="i === channels.length - 1" @click.stop="emit('demote', i)">
        <v-icon size="14">mdi-arrow-down</v-icon>
      </v-btn>
    </div>
  </div>
</template>

<script setup lang="ts">
interface ChannelInfo {
  index: number
  name: string
  status: string
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
</script>

<style scoped>
.channel-sequence {
  border: 1px solid rgba(var(--v-border-color), var(--v-border-opacity));
  border-radius: 0;
  overflow-x: hidden;
  overflow-y: auto;
  max-height: calc(8 * 36px);
  overscroll-behavior: auto;
}

.channel-item {
  border-bottom: 1px solid rgba(var(--v-border-color), calc(var(--v-border-opacity) * 0.6));
}

.channel-item:last-child {
  border-bottom: none;
}

.channel-item-animated {
  animation: ccx-slide-in 0.18s ease both;
}

.channel-item.demoted {
  opacity: 0.5;
}

.seq-num {
  font-size: 10px;
  font-weight: 700;
  opacity: 0.5;
  min-width: 2.5ch;
  font-variant-numeric: tabular-nums;
}

.seq-arrow {
  font-size: 10px;
  opacity: 0.35;
  margin: 0 4px;
}

.channel-name {
  cursor: pointer;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.channel-name:hover {
  text-decoration: underline;
  color: rgb(var(--v-theme-primary));
}

.next-channel-chip {
  font-weight: 700;
  animation: ccx-breathe 2s ease-in-out infinite;
}

.next-channel-chip :deep(.v-chip__content),
.next-channel-chip :deep(.v-chip__append) {
  color: #fff !important;
}

.fused-chip {
  background: repeating-linear-gradient(
    -45deg,
    var(--ccx-fused-stripe-b) 0px,
    var(--ccx-fused-stripe-b) 4px,
    var(--ccx-fused-stripe-a) 4px,
    var(--ccx-fused-stripe-a) 8px
  ) !important;
  color: #fff !important;
  border-radius: 0 !important;
  border: none !important;
  font-weight: 700;
  font-size: 9px !important;
  letter-spacing: 0.05em;
}

@keyframes ccx-breathe {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.55; }
}
</style>
