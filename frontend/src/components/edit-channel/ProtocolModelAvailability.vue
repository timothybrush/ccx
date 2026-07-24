<template>
  <div v-if="normalizedRoutes.length" class="protocol-model-availability">
    <div class="protocol-model-availability__header">
      <v-icon color="primary" size="20">mdi-routes</v-icon>
      <div>
        <div class="text-subtitle-2 font-weight-medium">
          {{ t('channelEditor.protocolModels.title') }}
        </div>
        <div class="text-caption text-medium-emphasis">
          {{ t('channelEditor.protocolModels.hint') }}
        </div>
      </div>
      <v-progress-circular v-if="loading" class="ml-auto" color="primary" indeterminate size="18" width="2" />
    </div>

    <div class="protocol-model-availability__rows">
      <div
        v-for="route in normalizedRoutes"
        :key="`${route.kind}:${route.channelUid || route.index}`"
        class="protocol-model-route"
        :data-kind="route.upstreamKind"
      >
        <div class="protocol-model-route__identity">
          <v-icon size="18" color="primary">{{ route.icon }}</v-icon>
          <div class="protocol-model-route__label">
            <span class="text-body-2 font-weight-medium">{{ route.label }}</span>
            <code class="protocol-model-route__path">{{ route.path }}</code>
          </div>
          <v-chip v-if="route.hasInventory" size="x-small" variant="tonal" color="primary">
            {{ t('channelEditor.protocolModels.count', { count: route.models.length }) }}
          </v-chip>
        </div>

        <div v-if="route.hasInventory" class="protocol-model-route__discovery-meta">
          <span class="text-caption text-medium-emphasis">
            {{ t('channelEditor.protocolModels.lastDiscovered') }}
            {{ route.discoveryTime || t('channelEditor.protocolModels.discoveryTimeUnknown') }}
          </span>
          <v-chip size="x-small" variant="tonal" color="secondary">
            {{ route.discoverySourceLabel }}
          </v-chip>
          <v-btn
            v-if="route.channelUid"
            class="protocol-model-route__rediscover"
            size="x-small"
            variant="tonal"
            color="primary"
            :loading="isRediscovering(route)"
            :disabled="isRediscovering(route)"
            @click="handleRediscover(route)"
          >
            <v-icon start size="14">mdi-refresh</v-icon>
            {{ isRediscovering(route)
              ? t('channelEditor.protocolModels.rediscovering')
              : t('channelEditor.protocolModels.rediscover') }}
          </v-btn>
          <span v-if="route.modelDiscoveryMessage" class="text-caption text-medium-emphasis">
            {{ route.modelDiscoveryMessage }}
          </span>
        </div>
        <div v-if="route.rediscoverError" class="text-caption text-error">
          {{ route.rediscoverError }}
        </div>

        <!-- 多 Key 按可用 Key 集合归组，直接展示共同模型与各子集专有模型。 -->
        <div v-if="route.coverageGroups.length" class="protocol-model-route__coverage-groups">
          <div class="protocol-model-route__coverage-groups-header">
            <v-icon :color="route.hasBindingDifferences ? 'warning' : 'success'" size="16">
              {{ route.hasBindingDifferences ? 'mdi-key-alert' : 'mdi-check-all' }}
            </v-icon>
            <span
              class="text-caption font-weight-medium"
              :class="route.hasBindingDifferences ? 'text-warning' : 'text-success'"
            >
              {{ route.hasBindingDifferences
                ? t('channelEditor.protocolModels.diffCount', { count: route.diffModelCount })
                : t('channelEditor.protocolModels.consistent', { count: route.bindings.length }) }}
            </span>
          </div>
          <div class="protocol-model-route__coverage-group-list">
            <section
              v-for="group in route.coverageGroups"
              :key="group.signature"
              class="protocol-model-coverage-group"
            >
              <div class="protocol-model-coverage-group__meta">
                <span class="text-caption font-weight-medium">
                  {{ group.isSharedByAll
                    ? t('channelEditor.protocolModels.coverageGroupShared', { count: route.bindings.length })
                    : group.availableBindings.length
                      ? t('channelEditor.protocolModels.coverageGroupExclusive', { count: group.availableBindings.length })
                      : t('channelEditor.protocolModels.coverageGroupUnavailable') }}
                </span>
                <v-chip
                  size="x-small"
                  variant="tonal"
                  :color="coverageGroupColor(group, route.bindings.length)"
                >
                  {{ t('channelEditor.protocolModels.coverageGroupModelCount', { count: group.models.length }) }}
                </v-chip>
              </div>
              <div v-if="group.availableBindings.length" class="protocol-model-coverage-group__keys">
                <v-chip
                  v-for="binding in group.availableBindings"
                  :key="binding.credentialUid || binding.keyMask"
                  size="x-small"
                  variant="tonal"
                  :color="coverageGroupColor(group, route.bindings.length)"
                  class="protocol-model-coverage-group__key"
                >
                  {{ binding.keyMask }}
                </v-chip>
              </div>
              <div class="protocol-model-coverage-group__models">
                <v-chip
                  v-for="model in group.models"
                  :key="model"
                  size="small"
                  variant="outlined"
                  class="protocol-model-coverage-group__model"
                >
                  {{ model }}
                </v-chip>
              </div>
            </section>
          </div>
          <div v-if="route.hasBindingDifferences" class="protocol-model-route__coverage">
            <v-chip
              v-for="binding in route.bindings"
              :key="binding.credentialUid || binding.keyMask"
              size="x-small"
              variant="tonal"
              :color="binding.models.length === route.models.length ? 'success' : 'warning'"
            >
              {{ binding.keyMask }} ·
              {{ t('channelEditor.protocolModels.coverage', { available: binding.models.length, total: route.models.length }) }}
            </v-chip>
          </div>
        </div>

        <details v-if="route.models.length" class="protocol-model-route__all">
          <summary class="protocol-model-route__all-summary">
            <span class="text-caption">
              {{ t('channelEditor.protocolModels.viewAll', { count: route.models.length }) }}
            </span>
            <v-icon class="protocol-model-route__all-chevron" size="16">mdi-chevron-down</v-icon>
          </summary>
          <div class="protocol-model-route__models">
            <v-chip
              v-for="model in route.models"
              :key="model"
              size="small"
              variant="outlined"
              class="protocol-model-route__model"
            >
              {{ model }}
            </v-chip>
          </div>
        </details>
        <div v-else class="text-caption text-medium-emphasis">
          {{ t('channelEditor.protocolModels.empty') }}
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, reactive } from 'vue'

import { useI18n } from '../../i18n'
import type { ChannelKind, ChannelProtocolRoute } from '../../services/api'
import { autoDiscoverChannel, getChannelAutoStatus } from '../../services/autopilot-api'

interface ProtocolDefinition {
  labelKey: string
  path: string
  icon: string
}

interface NormalizedModelBinding {
  credentialUid?: string
  keyMask: string
  models: string[]
}

interface ModelCoverageGroup {
  signature: string
  models: string[]
  availableBindings: NormalizedModelBinding[]
  isSharedByAll: boolean
}

const protocolDefinitions: Record<ChannelKind, ProtocolDefinition> = {
  messages: {
    labelKey: 'channelEditor.protocolModels.messages',
    path: '/v1/messages',
    icon: 'mdi-message-text-outline',
  },
  chat: {
    labelKey: 'channelEditor.protocolModels.chat',
    path: '/v1/chat/completions',
    icon: 'mdi-forum-outline',
  },
  responses: {
    labelKey: 'channelEditor.protocolModels.responses',
    path: '/v1/responses',
    icon: 'mdi-code-json',
  },
  gemini: {
    labelKey: 'channelEditor.protocolModels.gemini',
    path: '/v1beta/models/{model}:generateContent',
    icon: 'mdi-creation-outline',
  },
  images: {
    labelKey: 'channelEditor.protocolModels.images',
    path: '/v1/images/*',
    icon: 'mdi-image-outline',
  },
  vectors: {
    labelKey: 'channelEditor.protocolModels.vectors',
    path: '/v1/embeddings',
    icon: 'mdi-vector-polyline',
  },
}

const props = withDefaults(defineProps<{
  routes?: ChannelProtocolRoute[]
  loading?: boolean
}>(), {
  loading: false,
})

const emit = defineEmits<{
  refreshed: []
}>()

const { t } = useI18n()

// 每个 route 的重新发现状态（按 channelUid 跟踪）。
const rediscoverState = reactive<Record<string, { running: boolean; error: string }>>({})

const routeKey = (route: ChannelProtocolRoute) => route.channelUid ?? ''

const isRediscovering = (route: ChannelProtocolRoute) => rediscoverState[routeKey(route)]?.running === true

const REDISCOVER_POLL_INTERVAL_MS = 1500
const REDISCOVER_POLL_TIMEOUT_MS = 30000

const sleep = (ms: number) => new Promise(resolve => setTimeout(resolve, ms))

const handleRediscover = async (route: ChannelProtocolRoute) => {
  const key = routeKey(route)
  if (!key || rediscoverState[key]?.running) return
  const state = (rediscoverState[key] ??= { running: false, error: '' })
  state.running = true
  state.error = ''

  try {
    try {
      await autoDiscoverChannel(route.kind, key)
    } catch (err) {
      // 409 表示发现任务已在运行，直接进入轮询等待，不算错误。
      const status = (err as { status?: number }).status
      if (status !== 409) throw err
    }

    const deadline = Date.now() + REDISCOVER_POLL_TIMEOUT_MS
    let discoveryError = ''
    for (;;) {
      await sleep(REDISCOVER_POLL_INTERVAL_MS)
      const status = await getChannelAutoStatus(route.kind, key)
      const discovery = status.discovery
      if (discovery?.status === 'failed') {
        discoveryError = discovery.error || t('channelEditor.protocolModels.rediscoverFailed')
        break
      }
      if (!discovery || (discovery.status !== 'pending' && discovery.status !== 'running')) {
        break
      }
      if (Date.now() >= deadline) break
    }
    if (discoveryError) {
      state.error = discoveryError
      return
    }

    // 任务结束后通知父组件刷新模型清单。
    emit('refreshed')
  } catch (err) {
    state.error = err instanceof Error ? err.message : t('channelEditor.protocolModels.rediscoverFailed')
  } finally {
    state.running = false
  }
}

const discoverySourceKey: Record<string, string> = {
  control_plane: 'channelEditor.protocolModels.source.controlPlane',
  models_api: 'channelEditor.protocolModels.source.modelsApi',
  builtin_manifest: 'channelEditor.protocolModels.source.builtinManifest',
  builtin_fallback: 'channelEditor.protocolModels.source.builtinFallback',
  mixed: 'channelEditor.protocolModels.source.mixed',
}

const discoverySourceLabel = (source?: string) => {
  const key = source ? discoverySourceKey[source] : undefined
  return t(key ?? 'channelEditor.protocolModels.source.unknown')
}

const discoveryDateTimeFormat = new Intl.DateTimeFormat(undefined, {
  dateStyle: 'medium',
  timeStyle: 'medium',
})

const formatDiscoveryTime = (value?: string) => {
  if (!value) return ''
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '' : discoveryDateTimeFormat.format(date)
}

const normalizeModels = (models?: string[]) => Array.from(new Set(
  (models ?? []).map(model => model.trim()).filter(Boolean),
)).sort((left, right) => left.localeCompare(right))

const groupModelsByAvailability = (models: string[], bindings: NormalizedModelBinding[]): ModelCoverageGroup[] => {
  if (bindings.length < 2) return []

  const groups = new Map<string, ModelCoverageGroup>()
  for (const model of models) {
    const availability = bindings.map(binding => binding.models.includes(model))
    const availableBindings = bindings.filter((_, index) => availability[index])
    const signature = availability.map(available => (available ? '1' : '0')).join('')
    const group = groups.get(signature)
    if (group) {
      group.models.push(model)
      continue
    }
    groups.set(signature, {
      signature,
      models: [model],
      availableBindings,
      isSharedByAll: availableBindings.length === bindings.length,
    })
  }

  return Array.from(groups.values()).sort((left, right) => {
    const coverageDifference = right.availableBindings.length - left.availableBindings.length
    return coverageDifference || left.models[0].localeCompare(right.models[0])
  })
}

const coverageGroupColor = (group: ModelCoverageGroup, bindingCount: number) => {
  if (group.availableBindings.length === bindingCount) return 'success'
  return group.availableBindings.length > 0 ? 'warning' : 'error'
}

const normalizedRoutes = computed(() => (props.routes ?? []).map((route) => {
  const upstreamKind = route.upstreamKind ?? route.kind
  const definition = protocolDefinitions[upstreamKind]
  const hasDiscoveredInventory = route.modelInventoryKnown === true || Array.isArray(route.discoveredModels)
  const inventoryModels = hasDiscoveredInventory
    ? normalizeModels(route.discoveredModels)
    : normalizeModels(route.supportedModels)
  const bindings: NormalizedModelBinding[] = (route.modelBindings ?? []).map(binding => ({
    credentialUid: binding.credentialUid,
    keyMask: binding.keyMask,
    models: normalizeModels(binding.models),
  }))
  const models = normalizeModels([
    ...inventoryModels,
    ...bindings.flatMap(binding => binding.models),
  ])
  const coverageGroups = groupModelsByAvailability(models, bindings)
  const diffModelCount = coverageGroups.reduce(
    (count, group) => count + (group.isSharedByAll ? 0 : group.models.length),
    0,
  )
  const hasBindingDifferences = diffModelCount > 0

  return {
    ...route,
    upstreamKind,
    label: t(definition.labelKey),
    path: definition.path,
    icon: definition.icon,
    models,
    bindings,
    coverageGroups,
    diffModelCount,
    hasBindingDifferences,
    hasInventory: hasDiscoveredInventory || models.length > 0,
    discoveryTime: formatDiscoveryTime(route.modelsDiscoveredAt),
    discoverySourceLabel: discoverySourceLabel(route.modelDiscoverySource),
    rediscoverError: rediscoverState[routeKey(route)]?.error ?? '',
  }
}))
</script>

<style scoped>
.protocol-model-availability {
  margin-top: 8px;
  border-top: 1px solid rgba(var(--v-theme-on-surface), 0.12);
}

.protocol-model-availability__header {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 18px 0 12px;
}

.protocol-model-availability__rows {
  border: 1px solid rgba(var(--v-theme-on-surface), 0.12);
  border-radius: 6px;
  overflow: hidden;
}

.protocol-model-route {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 14px 16px;
}

.protocol-model-route + .protocol-model-route {
  border-top: 1px solid rgba(var(--v-theme-on-surface), 0.1);
}

.protocol-model-route__identity {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  min-width: 0;
}

.protocol-model-route__label {
  display: flex;
  flex: 1;
  min-width: 0;
  flex-direction: column;
  gap: 2px;
}

.protocol-model-route__discovery-meta {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px;
}

.protocol-model-route__rediscover {
  margin-left: auto;
}

.protocol-model-route__path {
  overflow-wrap: anywhere;
  color: rgba(var(--v-theme-on-surface), 0.62);
  font-size: 0.72rem;
  line-height: 1.35;
}

.protocol-model-route__coverage-groups {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 10px 12px;
  border: 1px dashed rgba(var(--v-theme-on-surface), 0.22);
  border-radius: 6px;
  background: rgba(var(--v-theme-on-surface), 0.02);
}

.protocol-model-route__coverage-groups-header {
  display: flex;
  align-items: center;
  gap: 6px;
}

.protocol-model-route__coverage-group-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.protocol-model-coverage-group {
  display: flex;
  flex-direction: column;
  flex-wrap: wrap;
  gap: 6px;
}

.protocol-model-coverage-group + .protocol-model-coverage-group {
  padding-top: 10px;
  border-top: 1px dashed rgba(var(--v-theme-on-surface), 0.16);
}

.protocol-model-coverage-group__meta,
.protocol-model-coverage-group__keys,
.protocol-model-coverage-group__models {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px;
}

.protocol-model-coverage-group__meta {
  gap: 8px;
}

.protocol-model-coverage-group__key {
  font-family: var(--v-font-family-mono, monospace);
}

.protocol-model-coverage-group__models {
  align-items: flex-start;
}

.protocol-model-coverage-group__model {
  height: auto;
  min-height: 24px;
  max-width: 100%;
}

.protocol-model-coverage-group__model :deep(.v-chip__content) {
  overflow-wrap: anywhere;
  white-space: normal;
  line-height: 1.35;
}

.protocol-model-route__coverage {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  padding-top: 6px;
  border-top: 1px dashed rgba(var(--v-theme-warning), 0.25);
}

.protocol-model-route__all-summary {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  width: fit-content;
  cursor: pointer;
  list-style: none;
  color: rgba(var(--v-theme-on-surface), 0.62);
}

.protocol-model-route__all-summary::-webkit-details-marker {
  display: none;
}

.protocol-model-route__all-chevron {
  transition: transform 0.16s ease;
}

.protocol-model-route__all[open] .protocol-model-route__all-chevron {
  transform: rotate(180deg);
}

.protocol-model-route__models {
  display: flex;
  align-items: flex-start;
  align-content: flex-start;
  flex-wrap: wrap;
  gap: 6px;
  min-width: 0;
  padding-top: 8px;
}

.protocol-model-route__model {
  height: auto;
  min-height: 24px;
  max-width: 100%;
}

.protocol-model-route__model :deep(.v-chip__content) {
  overflow-wrap: anywhere;
  white-space: normal;
  line-height: 1.35;
}
</style>
