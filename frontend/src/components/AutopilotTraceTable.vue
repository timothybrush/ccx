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
            <th style="width: 70px;">{{ t('autopilot.traceTable.col.match') }}</th>
            <th style="width: 80px;">{{ t('autopilot.traceTable.col.mode') }}</th>
            <th style="width: 90px;">{{ t('autopilot.traceTable.col.outcome') }}</th>
            <th style="width: 48px;"></th>
          </tr>
        </thead>
        <tbody>
          <template v-for="trace in filteredTraces" :key="trace.traceUid">
            <!-- 主行 -->
            <tr
              class="cursor-pointer"
              @click="toggleExpand(trace.traceUid)"
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
                    {{ shortenUid(trace.shadowChannelUid) }}
                  </v-chip>
                  <v-icon size="14">mdi-arrow-right</v-icon>
                  <v-chip size="x-small" variant="flat" color="secondary">
                    {{ shortenUid(trace.actualChannelUid) }}
                  </v-chip>
                </div>
              </td>
              <td>
                <v-chip
                  v-if="isComparable(trace)"
                  size="x-small"
                  :color="trace.match ? 'success' : 'error'"
                  variant="flat"
                >
                  {{ trace.match ? t('autopilot.traceTable.yes') : t('autopilot.traceTable.no') }}
                </v-chip>
                <span v-else class="text-caption text-medium-emphasis">-</span>
              </td>
              <td>
                <v-chip size="x-small" variant="tonal" :color="modeColor(trace.mode)">
                  {{ t(`autopilot.mode.${trace.mode}`) || trace.mode }}
                </v-chip>
              </td>
              <td>
                <v-chip v-if="trace.outcomeRecorded" size="x-small" variant="tonal" :color="outcomeColor(trace.outcome)">
                  {{ trace.outcome }}
                </v-chip>
                <span v-else class="text-caption text-medium-emphasis">-</span>
              </td>
              <td>
                <v-icon size="18">
                  {{ expanded[trace.traceUid] ? 'mdi-chevron-up' : 'mdi-chevron-down' }}
                </v-icon>
              </td>
            </tr>

            <!-- 展开详情行 -->
            <tr v-if="expanded[trace.traceUid]">
              <td colspan="9" class="pa-0">
                <v-expand-transition>
                  <div class="pa-4 bg-grey-lighten-5">
                    <!-- 候选列表 -->
                    <div v-if="trace.candidates && trace.candidates.length > 0" class="mb-3">
                      <div class="text-caption font-weight-bold mb-2">
                        {{ t('autopilot.traceTable.candidates') }}
                      </div>
                      <v-table density="compact" class="bg-transparent">
                        <thead>
                          <tr>
                            <th class="text-caption">Channel UID</th>
                            <th class="text-caption">Origin Tier</th>
                            <th class="text-caption">Health</th>
                            <th class="text-caption">Score</th>
                            <th class="text-caption">Domain</th>
                            <th class="text-caption">Selected</th>
                            <th class="text-caption">Filter Reasons</th>
                          </tr>
                        </thead>
                        <tbody>
                          <tr v-for="(cand, ci) in trace.candidates" :key="ci">
                            <td class="text-caption">{{ cand.channelUid }}</td>
                            <td class="text-caption">{{ cand.originTier || '-' }}</td>
                            <td class="text-caption">{{ cand.healthState || '-' }}</td>
                            <td class="text-caption">{{ cand.totalScore.toFixed(3) }}</td>
                            <td class="text-caption">
                              <div>{{ cand.domainEvidence?.source || '-' }}</div>
                              <div v-if="cand.domainEvidence?.canonicalModel" class="text-medium-emphasis">
                                {{ cand.domainEvidence.canonicalModel }} / {{ cand.domainEvidence.benchmarkCategory }}
                                · {{ cand.domainEvidence.canonicalCeiling?.toFixed(3) }} ×
                                {{ cand.domainEvidence.providerQualityFactor?.toFixed(3) }}
                              </div>
                            </td>
                            <td>
                              <v-icon v-if="cand.selected" size="14" color="success">mdi-check</v-icon>
                              <v-icon v-else size="14" color="grey">mdi-minus</v-icon>
                            </td>
                            <td class="text-caption">
                              {{ cand.filterReasons?.join('; ') || '-' }}
                            </td>
                          </tr>
                        </tbody>
                      </v-table>
                    </div>

                    <!-- 排序原因 -->
                    <div v-if="trace.sortReasons && trace.sortReasons.length > 0">
                      <div class="text-caption font-weight-bold mb-1">
                        {{ t('autopilot.traceTable.sortReasons') }}
                      </div>
                      <ul class="text-caption text-medium-emphasis ml-4">
                        <li v-for="(reason, ri) in trace.sortReasons" :key="ri">{{ reason }}</li>
                      </ul>
                    </div>
                  </div>
                </v-expand-transition>
              </td>
            </tr>
          </template>
        </tbody>
      </v-table>
    </v-card-text>
  </v-card>
</template>

<script setup lang="ts">
import { ref, reactive, computed } from 'vue'
import { useI18n } from '@/i18n'
import type { RoutingDecisionTrace } from '@/services/api-types'

const props = defineProps<{
  traces: RoutingDecisionTrace[]
  loading: boolean
}>()

const emit = defineEmits<{
  refresh: []
}>()

const { t } = useI18n()

const mismatchOnly = ref(false)
const expanded = reactive<Record<string, boolean>>({})

const filteredTraces = computed(() => {
  if (!mismatchOnly.value) return props.traces
  return props.traces.filter(tr => !tr.match && tr.shadowChannelUid && tr.actualChannelUid)
})

function toggleExpand(uid: string) {
  expanded[uid] = !expanded[uid]
}

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
  // 截取 ch_ 前缀后的前 8 位
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

function isComparable(trace: RoutingDecisionTrace): boolean {
  return trace.mode === 'shadow' && !!trace.shadowChannelUid && !!trace.actualChannelUid
}

function outcomeColor(outcome?: RoutingDecisionTrace['outcome']): string {
  if (outcome === 'success') return 'success'
  if (outcome === 'cancelled') return 'grey'
  if (outcome === 'attempt_failed') return 'warning'
  return 'error'
}
</script>
