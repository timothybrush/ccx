<template>
  <div class="stream-timeout-section">
    <v-row dense>
      <v-col cols="12">
        <div class="d-flex align-center justify-space-between flex-wrap ga-2 mb-2">
          <span class="section-title">{{ t('channelEditor.timeout.title') }}</span>
          <span class="text-caption text-medium-emphasis">{{ t('channelEditor.timeout.hint') }}</span>
        </div>

        <div class="timeout-control-grid timeout-control-grid--two mb-4">
          <div class="timeout-control" :class="{ 'timeout-control--disabled': !requestTimeoutEnabled }">
            <div class="timeout-control-header">
              <span class="timeout-label">{{ t('channelEditor.transport.requestTimeout.label') }}</span>
              <div class="timeout-header-actions">
                <span class="timeout-value">{{ requestTimeoutEnabled ? `${requestTimeoutSeconds}s` : t('addChannel.streamTimeoutStrategyInherit') }}</span>
                <v-switch
                  :model-value="requestTimeoutEnabled"
                  color="primary"
                  density="compact"
                  hide-details
                  inset
                  @update:model-value="setRequestTimeoutEnabled(Boolean($event))"
                />
              </div>
            </div>
            <input
              :value="requestTimeoutSeconds"
              type="range"
              min="1"
              max="300"
              step="1"
              class="timeout-slider"
              :disabled="!requestTimeoutEnabled"
              @input="emitSeconds('update:requestTimeoutMs', $event)"
            />
            <div class="timeout-range">
              <span>1s</span><span>300s</span>
            </div>
          </div>

          <div class="timeout-control" :class="{ 'timeout-control--disabled': !responseHeaderTimeoutEnabled }">
            <div class="timeout-control-header">
              <span class="timeout-label">{{ t('channelEditor.transport.responseHeaderTimeout.label') }}</span>
              <div class="timeout-header-actions">
                <span class="timeout-value">{{ responseHeaderTimeoutEnabled ? `${responseHeaderTimeoutSeconds}s` : t('addChannel.streamTimeoutStrategyInherit') }}</span>
                <v-switch
                  :model-value="responseHeaderTimeoutEnabled"
                  color="primary"
                  density="compact"
                  hide-details
                  inset
                  @update:model-value="setResponseHeaderTimeoutEnabled(Boolean($event))"
                />
              </div>
            </div>
            <input
              :value="responseHeaderTimeoutSeconds"
              type="range"
              min="1"
              max="300"
              step="1"
              class="timeout-slider"
              :disabled="!responseHeaderTimeoutEnabled"
              @input="emitSeconds('update:responseHeaderTimeoutMs', $event)"
            />
            <div class="timeout-range">
              <span>1s</span><span>300s</span>
            </div>
          </div>
        </div>

        <div class="d-flex align-center justify-space-between flex-wrap ga-2 mb-2">
          <span class="section-title">{{ t('addChannel.streamTimeoutStrategyLabel') }}</span>
          <span class="text-caption text-medium-emphasis">
            {{ selectedStrategy === 'inherit' ? t('addChannel.streamTimeoutInheritHint') : t('addChannel.streamTimeoutOverrideHint') }}
          </span>
        </div>
        <v-btn-toggle
          :model-value="selectedStrategy"
          divided
          variant="outlined"
          density="comfortable"
          class="stream-timeout-strategy-toggle"
          @update:model-value="$emit('apply-strategy', String($event))"
        >
          <v-btn value="inherit">{{ t('addChannel.streamTimeoutStrategyInherit') }}</v-btn>
          <v-btn value="gentle">{{ t('dialog.circuitBreaker.presetGentle') }}</v-btn>
          <v-btn value="balanced">{{ t('dialog.circuitBreaker.presetBalanced') }}</v-btn>
          <v-btn value="aggressive">{{ t('dialog.circuitBreaker.presetAggressive') }}</v-btn>
        </v-btn-toggle>
      </v-col>

      <v-col cols="12">
        <div class="timeout-control-grid">
          <!-- 首字等待超时 -->
          <div class="timeout-control" :class="{ 'timeout-control--disabled': !firstContentEnabled }">
            <div class="timeout-control-header">
              <span class="timeout-label">{{ t('addChannel.streamFirstContentTimeoutLabel') }}</span>
              <span class="timeout-value">{{ firstContentMs / 1000 }}s</span>
            </div>
            <input
              :value="firstContentMs"
              type="range"
              min="5000"
              max="300000"
              step="1000"
              class="timeout-slider"
              :disabled="!firstContentEnabled"
              @input="emitNumber('update:firstContentMs', $event)"
            />
            <div class="timeout-range">
              <span>5s</span><span>300s</span>
            </div>
          </div>

          <!-- 首字后断流超时 -->
          <div class="timeout-control" :class="{ 'timeout-control--disabled': !inactivityEnabled }">
            <div class="timeout-control-header">
              <span class="timeout-label">{{ t('addChannel.streamInactivityTimeoutLabel') }}</span>
              <span class="timeout-value">{{ inactivityMs / 1000 }}s</span>
            </div>
            <input
              :value="inactivityMs"
              type="range"
              min="1000"
              max="180000"
              step="1000"
              class="timeout-slider"
              :disabled="!inactivityEnabled"
              @input="emitNumber('update:inactivityMs', $event)"
            />
            <div class="timeout-range">
              <span>1s</span><span>180s</span>
            </div>
          </div>

          <!-- 工具调用空闲超时 -->
          <div class="timeout-control" :class="{ 'timeout-control--disabled': !toolCallIdleEnabled }">
            <div class="timeout-control-header">
              <span class="timeout-label">{{ t('addChannel.streamToolCallIdleTimeoutLabel') }}</span>
              <span class="timeout-value">{{ toolCallIdleMs / 1000 }}s</span>
            </div>
            <input
              :value="toolCallIdleMs"
              type="range"
              min="30000"
              max="300000"
              step="1000"
              class="timeout-slider"
              :disabled="!toolCallIdleEnabled"
              @input="emitNumber('update:toolCallIdleMs', $event)"
            />
            <div class="timeout-range">
              <span>30s</span><span>300s</span>
            </div>
          </div>
        </div>
      </v-col>
    </v-row>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from '../../i18n'

const DEFAULT_OPTIONAL_TIMEOUT_MS = 60_000

interface Props {
  requestTimeoutMs?: string | number | null
  responseHeaderTimeoutMs?: string | number | null
  selectedStrategy: string
  firstContentEnabled: boolean
  firstContentMs: number
  inactivityEnabled: boolean
  inactivityMs: number
  toolCallIdleEnabled: boolean
  toolCallIdleMs: number
}

const props = defineProps<Props>()

const emit = defineEmits<{
  'update:requestTimeoutMs': [string | number | null]
  'update:responseHeaderTimeoutMs': [string | number | null]
  'apply-strategy': [string]
  'update:firstContentMs': [number]
  'update:inactivityMs': [number]
  'update:toolCallIdleMs': [number]
}>()

const { t } = useI18n()

const isOptionalTimeoutEnabled = (value: string | number | null | undefined) => {
  if (value === null || value === undefined || value === '') return false
  return Number(value) > 0
}

const timeoutSeconds = (value: string | number | null | undefined, fallbackMs = DEFAULT_OPTIONAL_TIMEOUT_MS) => {
  const ms = Number(value)
  const safeMs = Number.isFinite(ms) && ms > 0 ? ms : fallbackMs
  return Math.min(300, Math.max(1, Math.round(safeMs / 1000)))
}

const requestTimeoutEnabled = computed(() => isOptionalTimeoutEnabled(props.requestTimeoutMs))
const responseHeaderTimeoutEnabled = computed(() => isOptionalTimeoutEnabled(props.responseHeaderTimeoutMs))
const requestTimeoutSeconds = computed(() => timeoutSeconds(props.requestTimeoutMs))
const responseHeaderTimeoutSeconds = computed(() => timeoutSeconds(props.responseHeaderTimeoutMs))

const setRequestTimeoutEnabled = (enabled: boolean) => {
  emit('update:requestTimeoutMs', enabled ? DEFAULT_OPTIONAL_TIMEOUT_MS : null)
}

const setResponseHeaderTimeoutEnabled = (enabled: boolean) => {
  emit('update:responseHeaderTimeoutMs', enabled ? DEFAULT_OPTIONAL_TIMEOUT_MS : null)
}

const emitSeconds = (
  eventName: 'update:requestTimeoutMs' | 'update:responseHeaderTimeoutMs',
  event: Event,
) => {
  const target = event.target
  if (!(target instanceof window.HTMLInputElement)) return
  const value = Number(target.value) * 1000
  if (eventName === 'update:requestTimeoutMs') {
    emit('update:requestTimeoutMs', value)
  } else {
    emit('update:responseHeaderTimeoutMs', value)
  }
}

const emitNumber = (
  eventName: 'update:firstContentMs' | 'update:inactivityMs' | 'update:toolCallIdleMs',
  event: Event,
) => {
  const target = event.target
  if (!(target instanceof window.HTMLInputElement)) return
  const value = Number(target.value)
  if (eventName === 'update:firstContentMs') {
    emit('update:firstContentMs', value)
  } else if (eventName === 'update:inactivityMs') {
    emit('update:inactivityMs', value)
  } else {
    emit('update:toolCallIdleMs', value)
  }
}
</script>

<style scoped>
.section-title {
  font-size: 0.875rem;
  line-height: 1.4;
  font-weight: 600;
  letter-spacing: 0;
}

.stream-timeout-strategy-toggle {
  display: flex;
  flex-wrap: wrap;
  width: 100%;
}

.stream-timeout-strategy-toggle :deep(.v-btn) {
  flex: 1 1 120px;
  min-width: 0;
}

.timeout-control-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 0;
  margin-bottom: 16px;
  border: 2px solid rgb(var(--v-theme-on-surface));
  background: rgb(var(--v-theme-surface));
}

.timeout-control-grid--two {
  grid-template-columns: repeat(2, 1fr);
}

.v-theme--dark .timeout-control-grid {
  border-color: rgba(255, 255, 255, 0.6);
}

.timeout-control {
  padding: 12px 14px;
  position: relative;
  transition: opacity 0.2s ease;
}

.timeout-control--disabled {
  opacity: 0.4;
}

.timeout-control:not(:last-child)::after {
  content: '';
  position: absolute;
  right: 0;
  top: 8px;
  bottom: 8px;
  width: 2px;
  background: rgb(var(--v-theme-on-surface));
  opacity: 0.18;
}

.v-theme--dark .timeout-control:not(:last-child)::after {
  background: rgba(255, 255, 255, 0.6);
}

.timeout-control-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
  gap: 6px;
}

.timeout-header-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}

.timeout-header-actions :deep(.v-switch) {
  flex: 0 0 auto;
}

.timeout-label {
  font-size: 0.75rem;
  font-weight: 600;
  color: rgb(var(--v-theme-on-surface) / 70%);
  text-transform: uppercase;
  letter-spacing: 0;
  line-height: 1.3;
}

.timeout-value {
  font-family: 'JetBrains Mono', 'Fira Code', monospace;
  font-size: 0.8125rem;
  font-weight: 700;
  color: rgb(var(--v-theme-primary));
  padding: 2px 8px;
  border: 2px solid rgb(var(--v-theme-on-surface));
  background: rgb(var(--v-theme-surface));
  flex-shrink: 0;
  min-width: 40px;
  text-align: center;
}

.v-theme--dark .timeout-value {
  border-color: rgba(255, 255, 255, 0.5);
}

.timeout-slider {
  -webkit-appearance: none;
  appearance: none;
  width: 100%;
  height: 8px;
  border-radius: 0;
  border: 2px solid rgb(var(--v-theme-on-surface) / 20%);
  background: rgb(var(--v-theme-on-surface) / 8%);
  outline: none;
  cursor: pointer;
}

.timeout-slider::-webkit-slider-thumb {
  -webkit-appearance: none;
  appearance: none;
  width: 20px;
  height: 20px;
  border-radius: 0;
  background: rgb(var(--v-theme-primary));
  cursor: pointer;
  border: 2px solid rgb(var(--v-theme-on-surface));
  box-shadow: 2px 2px 0 0 rgb(var(--v-theme-on-surface));
  transition: box-shadow 0.1s ease;
}

.timeout-slider::-webkit-slider-thumb:hover {
  transform: translate(-1px, -1px);
  box-shadow: 3px 3px 0 0 rgb(var(--v-theme-on-surface));
}

.timeout-slider::-moz-range-thumb {
  width: 20px;
  height: 20px;
  border-radius: 0;
  background: rgb(var(--v-theme-primary));
  cursor: pointer;
  border: 2px solid rgb(var(--v-theme-on-surface));
  box-shadow: 2px 2px 0 0 rgb(var(--v-theme-on-surface));
}

.timeout-slider:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.v-theme--dark .timeout-slider {
  border-color: rgba(255, 255, 255, 0.2);
  background: rgba(255, 255, 255, 0.06);
}

.v-theme--dark .timeout-slider::-webkit-slider-thumb {
  border-color: rgba(255, 255, 255, 0.6);
  box-shadow: 2px 2px 0 0 rgba(255, 255, 255, 0.6);
}

.v-theme--dark .timeout-slider::-moz-range-thumb {
  border-color: rgba(255, 255, 255, 0.6);
  box-shadow: 2px 2px 0 0 rgba(255, 255, 255, 0.6);
}

.timeout-range {
  display: flex;
  justify-content: space-between;
  font-size: 0.6875rem;
  color: rgb(var(--v-theme-on-surface) / 50%);
  margin-top: 4px;
}

@media (max-width: 768px) {
  .timeout-control-grid {
    grid-template-columns: 1fr;
  }

  .timeout-control:not(:last-child)::after {
    display: none;
  }
}
</style>
