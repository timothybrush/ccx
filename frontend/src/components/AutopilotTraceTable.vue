<template>
  <v-card variant="outlined" rounded="lg">
    <v-card-title class="d-flex align-center justify-space-between text-subtitle-1 font-weight-bold pb-0">
      <div class="d-flex align-center">
        <v-icon size="20" class="mr-2" color="info">mdi-format-list-bulleted</v-icon>
        {{ t('autopilot.traceTable.title') }}
      </div>
      <div class="d-flex align-center ga-2">
        <!-- 只看不一致开关 -->
        <v-switch
          v-model="mismatchOnly"
          :label="t('autopilot.traceTable.mismatchOnly')"
          color="warning"
          density="compact"
          hide-details
          @update:model-value="onFilterChange"
        />
        <v-btn
          variant="tonal"
          size="small"
          prepend-icon="mdi-refresh"
          :loading="loading"
          @click="emit('refresh')"
        >
          {{ t('app.actions.refresh') }}
        </v-btn>
      </div>
    </v-card-title>

    <v-card-text>
      <!-- 部分行损坏提示 -->
      <v-alert v-if="partial" type="warning" variant="tonal" density="compact" class="mb-2">
        {{ t('autopilot.traceTable.partial') }}
      </v-alert>

      <!-- 空态 -->
      <div v-if="filteredTraces.length === 0 && !loading" class="text-center py-8 text-medium-emphasis">
        {{ t('autopilot.traceTable.empty') }}
      </div>

      <!-- 表格 -->
      <v-table v-else hover density="compact">
        <thead>
          <tr>
            <th style="width: 150px;">{{ t('autopilot.traceTable.col.time') }}</th>
            <th style="width: 80px;">{{ t('autopilot.traceTable.col.kind') }}</th>
            <th style="width: 100px;">{{ t('autopilot.traceTable.col.taskClass') }}</th>
            <th style="width: 160px;">{{ t('autopilot.traceTable.col.model') }}</th>
            <th>{{ t('autopilot.traceTable.col.shadowVsActual') }}</th>
            <th style="width: 90px;">{{ t('autopilot.traceTable.col.comparison') }}</th>
            <th style="width: 80px;">{{ t('autopilot.traceTable.col.mode') }}</th>
            <th style="width: 90px;">{{ t('autopilot.traceTable.col.outcome') }}</th>
            <th style="width: 48px;"></th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="trace in filteredTraces"
            :key="trace.traceUid"
            class="cursor-pointer"
            @click="emit('select', trace.traceUid)"
          >
            <td class="text-caption">{{ formatTime(trace.createdAt) }}</td>
            <td>
              <v-chip size="x-small" variant="outlined" color="primary">
                {{ trace.requestKind }}
              </v-chip>
            </td>
            <td>
              <v-chip size="x-small" variant="tonal" color="secondary">
                {{ trace.taskClass || '-' }}
              </v-chip>
            </td>
            <td class="text-caption" style="max-width: 160px; overflow: hidden; text-overflow: ellipsis;">
              {{ trace.requestedModel || '-' }}
            </td>
            <td>
              <div class="d-flex align-center ga-1 text-caption">
                <v-chip size="x-small" variant="flat" color="info">
                  {{ shortenUid(trace.recommendedChannelUid) }}
                </v-chip>
                <v-icon size="14">mdi-arrow-right</v-icon>
                <v-chip size="x-small" variant="flat" color="secondary">
                  {{ shortenUid(trace.actualChannelUid) }}
                </v-chip>
              </div>
            </td>
            <td>
              <v-chip
                size="x-small"
                :color="comparisonColor(trace.comparisonStatus)"
                variant="flat"
              >
                {{ t(`autopilot.traceTable.comparison.${trace.comparisonStatus}`) }}
              </v-chip>
            </td>
            <td>
              <v-chip size="x-small" variant="tonal" :color="modeColor(trace.mode)">
                {{ t(`autopilot.mode.${trace.mode}`) || trace.mode }}
              </v-chip>
            </td>
            <td>
              <v-chip v-if="trace.outcome" size="x-small" variant="tonal" :color="outcomeColor(trace.outcome)">
                {{ trace.outcome }}
              </v-chip>
              <span v-else class="text-caption text-medium-emphasis">-</span>
            </td>
            <td>
              <v-icon size="18" color="primary">mdi-chevron-right</v-icon>
            </td>
          </tr>
        </tbody>
      </v-table>
    </v-card-text>
  </v-card>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { useI18n } from '@/i18n'
import type { TraceSummary } from '@/services/api-types'

const props = defineProps<{
  traces: TraceSummary[]
  loading: boolean
  partial?: boolean
}>()

const emit = defineEmits<{
  refresh: []
  select: [traceUid: string]
}>()

const { t } = useI18n()

const mismatchOnly = ref(false)

const filteredTraces = computed(() => {
  if (!mismatchOnly.value) return props.traces
  return props.traces.filter(tr => tr.comparisonStatus === 'mismatched')
})

function onFilterChange() {
  // 过滤由前端本地计算，无需重新请求
}

function formatTime(iso: string): string {
  if (!iso) return '-'
  try {
    const d = new Date(iso)
    return d.toLocaleString()
  } catch {
    return iso
  }
}

function shortenUid(uid?: string): string {
  if (!uid) return '-'
  const stripped = uid.replace(/^ch_/, '')
  return stripped.length > 8 ? stripped.slice(0, 8) + '...' : stripped
}

function modeColor(mode: string): string {
  const map: Record<string, string> = {
    off: 'grey',
    shadow: 'info',
    assist: 'warning',
    auto: 'success',
    active: 'primary',
    dry_run: 'info',
  }
  return map[mode] ?? 'grey'
}

function comparisonColor(status: string): string {
  if (status === 'matched') return 'success'
  if (status === 'mismatched') return 'error'
  return 'grey'
}

function outcomeColor(outcome?: string): string {
  if (outcome === 'success') return 'success'
  if (outcome === 'cancelled') return 'grey'
  if (outcome === 'attempt_failed') return 'warning'
  return 'error'
}
</script>
