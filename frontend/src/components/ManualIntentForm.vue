<template>
  <div class="manual-intent-form d-flex flex-column ga-4">
    <!-- 意图类型 -->
    <v-select
      v-model="intentType"
      :items="intentTypeItems"
      item-title="title"
      item-value="value"
      :label="t('manualIntent.field.intentType')"
      variant="outlined"
      density="compact"
      hide-details
      prepend-inner-icon="mdi-flask-outline"
    />

    <!-- 渠道类型（必填） -->
    <v-select
      v-model="channelKind"
      :items="channelKindItems"
      item-title="title"
      item-value="value"
      :label="t('manualIntent.field.channelKind')"
      variant="outlined"
      density="compact"
      :error-messages="channelKindError ? [t('manualIntent.validation.channelKindRequired')] : []"
      prepend-inner-icon="mdi-transit-connection-variant"
    />

    <!-- 名称（可选） -->
    <v-text-field
      v-model="name"
      :label="t('manualIntent.field.name')"
      variant="outlined"
      density="compact"
      hide-details
      prepend-inner-icon="mdi-tag"
    />

    <!-- 目标渠道 / 上游模型 -->
    <div class="d-flex ga-3">
      <v-text-field
        v-model="channelUid"
        :label="t('manualIntent.field.channelUid')"
        variant="outlined"
        density="compact"
        hide-details
        class="flex-grow-1"
        prepend-inner-icon="mdi-server-network"
      />
      <v-text-field
        v-model="metricsKey"
        :label="t('manualIntent.field.metricsKey')"
        variant="outlined"
        density="compact"
        hide-details
        class="flex-grow-1"
        prepend-inner-icon="mdi-key-variant"
      />
    </div>

    <div class="d-flex ga-3">
      <v-text-field
        v-model="model"
        :label="t('manualIntent.field.model')"
        variant="outlined"
        density="compact"
        hide-details
        class="flex-grow-1"
        prepend-inner-icon="mdi-cube-outline"
      />
      <v-text-field
        v-model="mappedModel"
        :label="t('manualIntent.field.mappedModel')"
        variant="outlined"
        density="compact"
        hide-details
        class="flex-grow-1"
        prepend-inner-icon="mdi-swap-horizontal"
      />
    </div>

    <!-- session_pin 需要 sessionId -->
    <v-text-field
      v-if="intentType === 'session_pin'"
      v-model="sessionId"
      :label="t('manualIntent.field.sessionId')"
      variant="outlined"
      density="compact"
      :error-messages="sessionIdError ? [t('manualIntent.validation.sessionIdRequired')] : []"
      prepend-inner-icon="mdi-pin"
    />

    <!-- 作用范围：agentRoles / taskClasses -->
    <v-select
      v-model="agentRoles"
      :items="agentRoleItems"
      item-title="title"
      item-value="value"
      :label="t('manualIntent.field.agentRoles')"
      variant="outlined"
      density="compact"
      hide-details
      multiple
      chips
      closable-chips
      prepend-inner-icon="mdi-account-group"
    />

    <v-select
      v-model="taskClasses"
      :items="taskClassItems"
      item-title="title"
      item-value="value"
      :label="t('manualIntent.field.taskClasses')"
      variant="outlined"
      density="compact"
      hide-details
      multiple
      chips
      closable-chips
      prepend-inner-icon="mdi-shape-outline"
    />

    <!-- 流量比例滑块 -->
    <div>
      <div class="d-flex align-center justify-space-between mb-1">
        <span class="text-body-2">{{ t('manualIntent.field.trafficPercent') }}</span>
        <span class="text-body-2 font-weight-medium">{{ trafficPercent }}%</span>
      </div>
      <v-slider
        v-model="trafficPercent"
        :min="0"
        :max="100"
        :step="1"
        color="primary"
        hide-details
        thumb-label
      />
    </div>

    <!-- 安全边界：TTL / maxRequests / maxEstimatedCost -->
    <div class="d-flex ga-3">
      <v-text-field
        v-model.number="ttlMinutes"
        type="number"
        :label="t('manualIntent.field.ttlMinutes')"
        variant="outlined"
        density="compact"
        hide-details
        class="flex-grow-1"
        prepend-inner-icon="mdi-timer-outline"
      />
      <v-text-field
        v-model.number="maxRequests"
        type="number"
        :label="t('manualIntent.field.maxRequests')"
        variant="outlined"
        density="compact"
        hide-details
        class="flex-grow-1"
        prepend-inner-icon="mdi-counter"
      />
      <v-text-field
        v-model.number="maxEstimatedCost"
        type="number"
        :label="t('manualIntent.field.maxEstimatedCost')"
        variant="outlined"
        density="compact"
        hide-details
        class="flex-grow-1"
        prepend-inner-icon="mdi-currency-usd"
      />
    </div>

    <!-- 开关 -->
    <div class="d-flex ga-4">
      <v-switch
        v-model="fallbackOnFailure"
        :label="t('manualIntent.field.fallbackOnFailure')"
        color="primary"
        density="compact"
        hide-details
        inset
      />
      <v-switch
        v-model="requireHardConstraints"
        :label="t('manualIntent.field.requireHardConstraints')"
        color="primary"
        density="compact"
        hide-details
        inset
      />
    </div>

    <!-- 备注 -->
    <v-textarea
      v-model="reason"
      :label="t('manualIntent.field.reason')"
      variant="outlined"
      density="compact"
      hide-details
      rows="2"
      auto-grow
      prepend-inner-icon="mdi-note-text-outline"
    />

    <!-- 错误提示 -->
    <v-alert
      v-if="submitError"
      type="error"
      variant="tonal"
      density="compact"
      class="mb-0"
    >
      {{ submitError }}
    </v-alert>

    <!-- 操作按钮 -->
    <div class="d-flex justify-end ga-2">
      <v-btn variant="text" @click="$emit('close')">
        {{ t('app.actions.cancel') }}
      </v-btn>
      <v-btn
        color="primary"
        variant="flat"
        :loading="submitting"
        prepend-icon="mdi-flask-outline"
        @click="handleSubmit"
      >
        {{ t('manualIntent.actions.create') }}
      </v-btn>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from '../i18n'
import { api } from '../services/api'
import type {
  CreateIntentRequest,
  ManualIntentType,
  ManualIntentTaskClass,
  ManualRoutingIntent,
} from '../services/api-types'

interface Props {
  prefillChannelKind?: string
  prefillChannelUid?: string
  prefillModel?: string
}

const props = defineProps<Props>()

const emit = defineEmits<{
  created: [intent: ManualRoutingIntent]
  close: []
}>()

const { t } = useI18n()

// ---- 表单状态 ----
const intentType = ref<ManualIntentType>('channel_trial')
const channelKind = ref<string>(props.prefillChannelKind ?? '')
const name = ref('')
const channelUid = ref<string>(props.prefillChannelUid ?? '')
const metricsKey = ref('')
const model = ref<string>(props.prefillModel ?? '')
const mappedModel = ref('')
const sessionId = ref('')
const agentRoles = ref<string[]>([])
const taskClasses = ref<ManualIntentTaskClass[]>([])
const trafficPercent = ref(100)
const ttlMinutes = ref<number>(120)
const maxRequests = ref<number | null>(null)
const maxEstimatedCost = ref<number | null>(null)
const fallbackOnFailure = ref(true)
const requireHardConstraints = ref(true)
const reason = ref('')

const submitting = ref(false)
const submitError = ref('')
const channelKindError = ref(false)
const sessionIdError = ref(false)

// 父组件预填变化时同步
watch(() => props.prefillChannelKind, v => { if (v) channelKind.value = v })
watch(() => props.prefillChannelUid, v => { if (v !== undefined) channelUid.value = v })
watch(() => props.prefillModel, v => { if (v !== undefined) model.value = v })

// ---- 下拉选项 ----
const intentTypeItems = computed(() => [
  { title: t('manualIntent.intentType.model_trial'), value: 'model_trial' as const },
  { title: t('manualIntent.intentType.channel_trial'), value: 'channel_trial' as const },
  { title: t('manualIntent.intentType.endpoint_trial'), value: 'endpoint_trial' as const },
  { title: t('manualIntent.intentType.session_pin'), value: 'session_pin' as const },
])

const channelKindItems = [
  { title: 'messages', value: 'messages' },
  { title: 'chat', value: 'chat' },
  { title: 'responses', value: 'responses' },
  { title: 'gemini', value: 'gemini' },
  { title: 'images', value: 'images' },
  { title: 'vectors', value: 'vectors' },
]

const agentRoleItems = computed(() => [
  { title: t('manualIntent.agentRole.main'), value: 'main' },
  { title: t('manualIntent.agentRole.subagent'), value: 'subagent' },
])

const taskClassItems = computed(() => [
  { title: t('manualIntent.taskClass.supervisor'), value: 'supervisor' as const },
  { title: t('manualIntent.taskClass.worker'), value: 'worker' as const },
  { title: t('manualIntent.taskClass.lightweight'), value: 'lightweight' as const },
  { title: t('manualIntent.taskClass.vision'), value: 'vision' as const },
  { title: t('manualIntent.taskClass.long_context'), value: 'long_context' as const },
  { title: t('manualIntent.taskClass.image_generation'), value: 'image_generation' as const },
  { title: t('manualIntent.taskClass.embedding'), value: 'embedding' as const },
])

// ---- 提交 ----
function validate(): boolean {
  channelKindError.value = !channelKind.value
  sessionIdError.value = intentType.value === 'session_pin' && !sessionId.value.trim()
  return !channelKindError.value && !sessionIdError.value
}

async function handleSubmit() {
  submitError.value = ''
  if (!validate()) return

  const payload: CreateIntentRequest = {
    intentType: intentType.value,
    channelKind: channelKind.value,
    trafficPercent: trafficPercent.value,
    fallbackOnFailure: fallbackOnFailure.value,
    requireHardConstraints: requireHardConstraints.value,
  }
  if (name.value.trim()) payload.name = name.value.trim()
  if (channelUid.value.trim()) payload.channelUid = channelUid.value.trim()
  if (metricsKey.value.trim()) payload.metricsKey = metricsKey.value.trim()
  if (model.value.trim()) payload.model = model.value.trim()
  if (mappedModel.value.trim()) payload.mappedModel = mappedModel.value.trim()
  if (sessionId.value.trim()) payload.sessionId = sessionId.value.trim()
  if (agentRoles.value.length) payload.agentRoles = agentRoles.value
  if (taskClasses.value.length) payload.taskClasses = taskClasses.value
  if (ttlMinutes.value && ttlMinutes.value > 0) payload.ttlMinutes = ttlMinutes.value
  if (maxRequests.value && maxRequests.value > 0) payload.maxRequests = maxRequests.value
  if (maxEstimatedCost.value && maxEstimatedCost.value > 0) payload.maxEstimatedCost = maxEstimatedCost.value
  if (reason.value.trim()) payload.reason = reason.value.trim()

  submitting.value = true
  try {
    const intent = await api.createManualIntent(payload)
    emit('created', intent)
  } catch (err) {
    submitError.value = err instanceof Error ? err.message : t('manualIntent.error.createFailed')
  } finally {
    submitting.value = false
  }
}

defineExpose({ handleSubmit, submitting })
</script>

<style scoped>
.manual-intent-form {
  min-height: 0;
}
</style>
