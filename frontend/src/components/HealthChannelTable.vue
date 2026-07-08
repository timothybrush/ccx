<template>
  <div>
    <div v-if="channels.length === 0" class="text-center py-8 text-medium-emphasis">
      {{ t('healthCenter.noChannels') }}
    </div>

    <v-table v-else hover density="compact">
      <thead>
        <tr>
          <th style="width: 90px;">{{ t('healthCenter.col.status') }}</th>
          <th style="width: 100px;">{{ t('healthCenter.col.kind') }}</th>
          <th>{{ t('healthCenter.col.channel') }}</th>
          <th style="width: 100px;">{{ t('healthCenter.col.endpoints') }}</th>
          <th style="width: 120px;">{{ t('healthCenter.col.successRate') }}</th>
          <th style="width: 60px;"></th>
        </tr>
      </thead>
      <tbody>
        <template v-for="ch in channels" :key="ch.channelUid">
          <tr
            class="cursor-pointer"
            @click="toggleExpand(ch.channelUid)"
          >
            <td>
              <v-chip
                size="small"
                :color="stateColor(ch.aggState)"
                variant="tonal"
              >
                <v-icon size="14" start>{{ stateIcon(ch.aggState) }}</v-icon>
                {{ ch.aggState }}
              </v-chip>
            </td>
            <td>
              <v-chip size="x-small" variant="outlined">
                {{ kindLabel(ch.channelKind) }}
              </v-chip>
            </td>
            <td>
              <span class="font-weight-medium">{{ ch.channelName || ch.channelUid }}</span>
              <div class="d-flex ga-1 mt-1">
                <v-chip
                  v-if="ch.healthyCount > 0"
                  size="x-small"
                  color="success"
                  variant="flat"
                >{{ ch.healthyCount }} ok</v-chip>
                <v-chip
                  v-if="ch.degradedCount > 0"
                  size="x-small"
                  color="warning"
                  variant="flat"
                >{{ ch.degradedCount }} degraded</v-chip>
                <v-chip
                  v-if="ch.limitedCount > 0"
                  size="x-small"
                  color="orange"
                  variant="flat"
                >{{ ch.limitedCount }} limited</v-chip>
                <v-chip
                  v-if="ch.deadCount > 0"
                  size="x-small"
                  color="error"
                  variant="flat"
                >{{ ch.deadCount }} dead</v-chip>
                <v-chip
                  v-if="ch.unknownCount > 0"
                  size="x-small"
                  color="grey"
                  variant="flat"
                >{{ ch.unknownCount }} ?</v-chip>
              </div>
            </td>
            <td>{{ ch.endpointCount }}</td>
            <td :class="rateColor(ch.avgSuccessRate)">
              {{ formatPercent(ch.avgSuccessRate) }}
            </td>
            <td>
              <v-icon size="18">
                {{ expanded[ch.channelUid] ? 'mdi-chevron-up' : 'mdi-chevron-down' }}
              </v-icon>
            </td>
          </tr>

          <!-- Expanded detail row -->
          <tr v-if="expanded[ch.channelUid]">
            <td colspan="6" class="pa-0">
              <v-expand-transition>
                <HealthChannelDetail
                  v-if="expanded[ch.channelUid]"
                  :channel-uid="ch.channelUid"
                />
              </v-expand-transition>
            </td>
          </tr>
        </template>
      </tbody>
    </v-table>
  </div>
</template>

<script setup lang="ts">
import { reactive } from 'vue'
import { useI18n } from '@/i18n'
import HealthChannelDetail from './HealthChannelDetail.vue'
import type { ChannelHealthItem, HealthState } from '@/services/api-types'

defineProps<{ channels: ChannelHealthItem[] }>()
const { t } = useI18n()

const expanded = reactive<Record<string, boolean>>({})

function toggleExpand(uid: string) {
  expanded[uid] = !expanded[uid]
}

const knownKinds = new Set(['messages', 'chat', 'responses', 'gemini', 'images', 'vectors'])

function kindLabel(kind: string): string {
  if (knownKinds.has(kind)) {
    return t('healthCenter.kind.' + kind)
  }
  return kind
}

function stateColor(state: HealthState): string {
  const map: Record<HealthState, string> = {
    healthy: 'success',
    degraded: 'warning',
    limited: 'orange',
    misconfigured: 'deep-purple',
    dead: 'error',
    unknown: 'grey',
  }
  return map[state] ?? 'grey'
}

function stateIcon(state: HealthState): string {
  const map: Record<HealthState, string> = {
    healthy: 'mdi-heart-pulse',
    degraded: 'mdi-alert',
    limited: 'mdi-alert-circle-outline',
    misconfigured: 'mdi-shield-alert',
    dead: 'mdi-close-circle',
    unknown: 'mdi-information',
  }
  return map[state] ?? 'mdi-information'
}

function formatPercent(value?: number): string {
  if (value == null) return '-'
  return (value * 100).toFixed(1) + '%'
}

function rateColor(rate?: number): string {
  if (rate == null) return ''
  if (rate >= 0.95) return 'text-success'
  if (rate >= 0.8) return 'text-warning'
  return 'text-error'
}
</script>
