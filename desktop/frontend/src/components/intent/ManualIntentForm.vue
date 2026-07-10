<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { FlaskConical } from 'lucide-vue-next'
import { Alert } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { useAdminApi } from '@/composables/useAdminApi'
import { useLanguage } from '@/composables/useLanguage'
import { MANUAL_INTENTS_PATH } from '@/services/admin-api'
import type {
  CreateIntentRequest,
  ManualIntentTaskClass,
  ManualIntentType,
  ManualRoutingIntent,
} from '@/services/admin-api'

const props = defineProps<{
  prefillChannelKind?: string
  prefillChannelUid?: string
  prefillModel?: string
}>()

const emit = defineEmits<{
  created: [intent: ManualRoutingIntent]
  close: []
}>()

const { t } = useLanguage()
const api = useAdminApi()

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
const ttlMinutes = ref<number | undefined>(120)
const maxRequests = ref<number | undefined>(undefined)
const maxEstimatedCost = ref<number | undefined>(undefined)
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

// ---- 下拉/多选选项 ----
const intentTypeItems = computed(() => [
  { label: t('manualIntent.intentType.model_trial'), value: 'model_trial' as const },
  { label: t('manualIntent.intentType.channel_trial'), value: 'channel_trial' as const },
  { label: t('manualIntent.intentType.endpoint_trial'), value: 'endpoint_trial' as const },
  { label: t('manualIntent.intentType.session_pin'), value: 'session_pin' as const },
])

const channelKindItems = ['messages', 'chat', 'responses', 'gemini', 'images', 'vectors']

const agentRoleItems = computed(() => [
  { label: t('manualIntent.agentRole.main'), value: 'main' },
  { label: t('manualIntent.agentRole.subagent'), value: 'subagent' },
])

const taskClassItems = computed<{ label: string; value: ManualIntentTaskClass }[]>(() => [
  { label: t('manualIntent.taskClass.supervisor'), value: 'supervisor' },
  { label: t('manualIntent.taskClass.worker'), value: 'worker' },
  { label: t('manualIntent.taskClass.lightweight'), value: 'lightweight' },
  { label: t('manualIntent.taskClass.vision'), value: 'vision' },
  { label: t('manualIntent.taskClass.long_context'), value: 'long_context' },
  { label: t('manualIntent.taskClass.image_generation'), value: 'image_generation' },
  { label: t('manualIntent.taskClass.embedding'), value: 'embedding' },
])

// 多选切换：Checkbox 组
function toggleAgentRole(value: string, checked: boolean) {
  if (checked) {
    if (!agentRoles.value.includes(value)) agentRoles.value = [...agentRoles.value, value]
  } else {
    agentRoles.value = agentRoles.value.filter(v => v !== value)
  }
}

function toggleTaskClass(value: ManualIntentTaskClass, checked: boolean) {
  if (checked) {
    if (!taskClasses.value.includes(value)) taskClasses.value = [...taskClasses.value, value]
  } else {
    taskClasses.value = taskClasses.value.filter(v => v !== value)
  }
}

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
    const intent = await api.post<ManualRoutingIntent>(MANUAL_INTENTS_PATH, payload)
    emit('created', intent)
  } catch (err) {
    submitError.value = err instanceof Error ? err.message : t('manualIntent.error.createFailed')
  } finally {
    submitting.value = false
  }
}

defineExpose({ handleSubmit, submitting })
</script>

<template>
  <div class="flex flex-col gap-4">
    <!-- 意图类型 -->
    <div class="flex flex-col gap-1.5">
      <Label class="text-xs text-muted-foreground">{{ t('manualIntent.field.intentType') }}</Label>
      <Select
        :model-value="intentType"
        @update:model-value="(v) => (intentType = v as ManualIntentType)"
      >
        <SelectTrigger class="h-9 w-full">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem v-for="opt in intentTypeItems" :key="opt.value" :value="opt.value">
            {{ opt.label }}
          </SelectItem>
        </SelectContent>
      </Select>
    </div>

    <!-- 渠道类型（必填） -->
    <div class="flex flex-col gap-1.5">
      <Label class="text-xs text-muted-foreground">{{ t('manualIntent.field.channelKind') }}</Label>
      <Select
        :model-value="channelKind"
        @update:model-value="(v) => (channelKind = v as string)"
      >
        <SelectTrigger class="h-9 w-full" :class="channelKindError ? 'border-destructive' : ''">
          <SelectValue :placeholder="t('manualIntent.field.channelKind')" />
        </SelectTrigger>
        <SelectContent>
          <SelectItem v-for="opt in channelKindItems" :key="opt" :value="opt">{{ opt }}</SelectItem>
        </SelectContent>
      </Select>
      <p v-if="channelKindError" class="text-xs text-destructive">
        {{ t('manualIntent.validation.channelKindRequired') }}
      </p>
    </div>

    <!-- 名称 -->
    <div class="flex flex-col gap-1.5">
      <Label class="text-xs text-muted-foreground">{{ t('manualIntent.field.name') }}</Label>
      <Input v-model="name" class="h-9" />
    </div>

    <!-- 目标渠道 / metricsKey -->
    <div class="flex gap-3">
      <div class="flex flex-1 flex-col gap-1.5">
        <Label class="text-xs text-muted-foreground">{{ t('manualIntent.field.channelUid') }}</Label>
        <Input v-model="channelUid" class="h-9" />
      </div>
      <div class="flex flex-1 flex-col gap-1.5">
        <Label class="text-xs text-muted-foreground">{{ t('manualIntent.field.metricsKey') }}</Label>
        <Input v-model="metricsKey" class="h-9" />
      </div>
    </div>

    <!-- model / mappedModel -->
    <div class="flex gap-3">
      <div class="flex flex-1 flex-col gap-1.5">
        <Label class="text-xs text-muted-foreground">{{ t('manualIntent.field.model') }}</Label>
        <Input v-model="model" class="h-9" />
      </div>
      <div class="flex flex-1 flex-col gap-1.5">
        <Label class="text-xs text-muted-foreground">{{ t('manualIntent.field.mappedModel') }}</Label>
        <Input v-model="mappedModel" class="h-9" />
      </div>
    </div>

    <!-- session_pin 需要 sessionId -->
    <div v-if="intentType === 'session_pin'" class="flex flex-col gap-1.5">
      <Label class="text-xs text-muted-foreground">{{ t('manualIntent.field.sessionId') }}</Label>
      <Input v-model="sessionId" class="h-9" :class="sessionIdError ? 'border-destructive' : ''" />
      <p v-if="sessionIdError" class="text-xs text-destructive">
        {{ t('manualIntent.validation.sessionIdRequired') }}
      </p>
    </div>

    <!-- agentRoles 多选 -->
    <div class="flex flex-col gap-1.5">
      <Label class="text-xs text-muted-foreground">{{ t('manualIntent.field.agentRoles') }}</Label>
      <div class="flex flex-wrap gap-4">
        <label v-for="opt in agentRoleItems" :key="opt.value" class="flex items-center gap-2 text-sm">
          <Checkbox
            :model-value="agentRoles.includes(opt.value)"
            @update:model-value="(c) => toggleAgentRole(opt.value, c === true)"
          />
          {{ opt.label }}
        </label>
      </div>
    </div>

    <!-- taskClasses 多选 -->
    <div class="flex flex-col gap-1.5">
      <Label class="text-xs text-muted-foreground">{{ t('manualIntent.field.taskClasses') }}</Label>
      <div class="flex flex-wrap gap-4">
        <label v-for="opt in taskClassItems" :key="opt.value" class="flex items-center gap-2 text-sm">
          <Checkbox
            :model-value="taskClasses.includes(opt.value)"
            @update:model-value="(c) => toggleTaskClass(opt.value, c === true)"
          />
          {{ opt.label }}
        </label>
      </div>
    </div>

    <!-- 流量比例 -->
    <div class="flex flex-col gap-1.5">
      <Label class="text-xs text-muted-foreground">
        {{ t('manualIntent.field.trafficPercent') }} ({{ trafficPercent }}%)
      </Label>
      <Input v-model.number="trafficPercent" type="number" min="0" max="100" step="1" class="h-9" />
    </div>

    <!-- 安全边界 -->
    <div class="flex gap-3">
      <div class="flex flex-1 flex-col gap-1.5">
        <Label class="text-xs text-muted-foreground">{{ t('manualIntent.field.ttlMinutes') }}</Label>
        <Input v-model.number="ttlMinutes" type="number" min="0" class="h-9" />
      </div>
      <div class="flex flex-1 flex-col gap-1.5">
        <Label class="text-xs text-muted-foreground">{{ t('manualIntent.field.maxRequests') }}</Label>
        <Input v-model.number="maxRequests" type="number" min="0" class="h-9" />
      </div>
      <div class="flex flex-1 flex-col gap-1.5">
        <Label class="text-xs text-muted-foreground">{{ t('manualIntent.field.maxEstimatedCost') }}</Label>
        <Input v-model.number="maxEstimatedCost" type="number" min="0" step="0.01" class="h-9" />
      </div>
    </div>

    <!-- 开关 -->
    <div class="flex gap-6">
      <div class="flex items-center gap-2">
        <Switch :model-value="fallbackOnFailure" @update:model-value="(v) => (fallbackOnFailure = v)" />
        <span class="text-sm">{{ t('manualIntent.field.fallbackOnFailure') }}</span>
      </div>
      <div class="flex items-center gap-2">
        <Switch :model-value="requireHardConstraints" @update:model-value="(v) => (requireHardConstraints = v)" />
        <span class="text-sm">{{ t('manualIntent.field.requireHardConstraints') }}</span>
      </div>
    </div>

    <!-- 备注 -->
    <div class="flex flex-col gap-1.5">
      <Label class="text-xs text-muted-foreground">{{ t('manualIntent.field.reason') }}</Label>
      <Textarea v-model="reason" rows="2" />
    </div>

    <!-- 错误提示 -->
    <Alert v-if="submitError" variant="destructive">
      <p class="text-sm">{{ submitError }}</p>
    </Alert>

    <!-- 操作按钮 -->
    <div class="flex justify-end gap-2">
      <Button variant="ghost" @click="emit('close')">{{ t('manualIntent.actions.cancel') }}</Button>
      <Button :disabled="submitting" @click="handleSubmit">
        <FlaskConical class="mr-2 size-4" />
        {{ t('manualIntent.actions.create') }}
      </Button>
    </div>
  </div>
</template>
