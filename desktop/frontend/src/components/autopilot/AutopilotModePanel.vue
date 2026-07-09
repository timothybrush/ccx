<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { AlertOctagon } from 'lucide-vue-next'
import { Alert } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { useLanguage } from '@/composables/useLanguage'
import type { AutopilotMode, SmartRoutingConfig } from '@/services/admin-api'

const props = defineProps<{
  config: SmartRoutingConfig
  saving: boolean
}>()

const emit = defineEmits<{
  'update:config': [config: SmartRoutingConfig]
}>()

const { t } = useLanguage()

function cloneConfig(src: SmartRoutingConfig): SmartRoutingConfig {
  return {
    mode: src.mode,
    killSwitchActive: src.killSwitchActive,
    costPreference: src.costPreference,
  }
}

const localConfig = reactive<SmartRoutingConfig>(cloneConfig(props.config))

watch(
  () => props.config,
  (newCfg) => {
    localConfig.mode = newCfg.mode
    localConfig.killSwitchActive = newCfg.killSwitchActive
    localConfig.costPreference = newCfg.costPreference
  },
  { deep: true },
)

const modeOptions: AutopilotMode[] = ['off', 'shadow', 'assist', 'auto']

const costPreferenceItems = computed(() => [
  { value: 'quality_first', label: t('autopilot.costPreference.quality_first') },
  { value: 'balanced', label: t('autopilot.costPreference.balanced') },
  { value: 'cost_first', label: t('autopilot.costPreference.cost_first') },
  { value: 'custom', label: t('autopilot.costPreference.custom') },
])

const hasChanges = computed(() => {
  return (
    localConfig.mode !== props.config.mode ||
    localConfig.costPreference !== props.config.costPreference
  )
})

const confirmDialog = ref(false)
const pendingMode = ref<AutopilotMode | ''>('')

function onModeSelect(mode: AutopilotMode) {
  if (localConfig.killSwitchActive) return
  if (mode === localConfig.mode) return
  if (mode === 'assist' || mode === 'auto') {
    pendingMode.value = mode
    confirmDialog.value = true
    return
  }
  localConfig.mode = mode
}

function confirmModeChange() {
  if (pendingMode.value) {
    localConfig.mode = pendingMode.value
  }
  pendingMode.value = ''
  confirmDialog.value = false
}

function cancelModeChange() {
  pendingMode.value = ''
  confirmDialog.value = false
}

function saveConfig() {
  emit('update:config', cloneConfig(localConfig))
}

function resetConfig() {
  localConfig.mode = props.config.mode
  localConfig.killSwitchActive = props.config.killSwitchActive
  localConfig.costPreference = props.config.costPreference
}
</script>

<template>
  <div class="rounded-xl border border-border/60 bg-card/40 p-4">
    <h4 class="mb-3 text-sm font-bold">{{ t('autopilot.modePanel.title') }}</h4>

    <Alert v-if="localConfig.killSwitchActive" variant="destructive" class="mb-4">
      <AlertOctagon class="mr-2 inline size-4" />
      <p class="inline text-sm">{{ t('autopilot.modePanel.killSwitchActive') }}</p>
    </Alert>

    <div class="mb-4">
      <div class="mb-2 text-xs text-muted-foreground">{{ t('autopilot.modePanel.routingMode') }}</div>
      <div class="flex flex-wrap gap-2">
        <Button
          v-for="mode in modeOptions"
          :key="mode"
          size="sm"
          :variant="localConfig.mode === mode ? 'default' : 'outline'"
          :disabled="localConfig.killSwitchActive"
          @click="onModeSelect(mode)"
        >
          {{ t(`autopilot.mode.${mode}`) }}
        </Button>
      </div>
      <div class="mt-1 text-xs text-muted-foreground">
        {{ t(`autopilot.modeDesc.${localConfig.mode}`) }}
      </div>
    </div>

    <div class="mb-4">
      <div class="flex items-center gap-2">
        <Switch :model-value="localConfig.killSwitchActive" disabled />
        <span class="text-sm">{{ t('autopilot.modePanel.killSwitch') }}</span>
      </div>
      <div class="mt-1 text-xs text-muted-foreground">{{ t('autopilot.modePanel.killSwitchHint') }}</div>
    </div>

    <div class="mb-4">
      <div class="mb-2 text-xs text-muted-foreground">{{ t('autopilot.modePanel.costPreference') }}</div>
      <Select
        :model-value="localConfig.costPreference"
        :disabled="localConfig.killSwitchActive"
        @update:model-value="(v) => (localConfig.costPreference = v as string)"
      >
        <SelectTrigger class="h-9 w-full max-w-[280px]">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem v-for="opt in costPreferenceItems" :key="opt.value" :value="opt.value">
            {{ opt.label }}
          </SelectItem>
        </SelectContent>
      </Select>
      <div class="mt-1 text-xs text-muted-foreground">
        {{ t(`autopilot.costPreferenceDesc.${localConfig.costPreference}`) }}
      </div>
    </div>

    <div class="flex gap-2">
      <Button :disabled="!hasChanges || saving" @click="saveConfig">
        {{ t('autopilot.modePanel.save') }}
      </Button>
      <Button variant="ghost" :disabled="!hasChanges || saving" @click="resetConfig">
        {{ t('autopilot.modePanel.reset') }}
      </Button>
    </div>

    <Dialog v-model:open="confirmDialog">
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{{ t('autopilot.modePanel.confirmTitle') }}</DialogTitle>
        </DialogHeader>
        <p class="text-sm text-muted-foreground">
          {{ t('autopilot.modePanel.confirmMessage', { mode: pendingMode }) }}
        </p>
        <DialogFooter>
          <Button variant="ghost" @click="cancelModeChange">{{ t('app.actions.cancel') }}</Button>
          <Button variant="destructive" @click="confirmModeChange">{{ t('app.actions.confirm') }}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>
