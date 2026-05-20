<script setup lang="ts">
import { onMounted } from 'vue'
import AgentCard from '@/components/agent/AgentCard.vue'
import { useStatus } from '@/composables/useStatus'
import { useAgentConfig } from '@/composables/useAgentConfig'
import type { AgentPlatform } from '@/types'

const { status, actionError } = useStatus()
const {
  agentStatuses,
  configLoading,
  selectedClaudeProvider,
  claudeProviderKeys,
  claudeMiMoBaseUrl,
  agentLabels,
  agentPlatforms,
  claudeProviderLabel,
  claudeTargetBaseUrl,
  agentStatusText,
  agentStatusClass,
  loadAgentStatuses,
  canApplyAgent,
  applyAgent,
  restoreAgent,
  selectedCodexProvider,
  codexProviderLabels,
  codexProviderLabel,
} = useAgentConfig()

onMounted(() => {
  loadAgentStatuses()
})

const handleApply = async (platform: AgentPlatform) => {
  actionError.value = ''
  try {
    await applyAgent(platform)
    await loadAgentStatuses()
  } catch (error) {
    actionError.value = error instanceof Error ? error.message : String(error)
  }
}

const handleRestore = async (platform: AgentPlatform) => {
  actionError.value = ''
  try {
    await restoreAgent(platform)
    await loadAgentStatuses()
  } catch (error) {
    actionError.value = error instanceof Error ? error.message : String(error)
  }
}
</script>

<template>
  <div class="space-y-4">
    <div class="grid grid-cols-1 lg:grid-cols-2 gap-4">
      <AgentCard
        v-for="platform in agentPlatforms"
        :key="platform"
        :platform="platform"
        :agent-status="agentStatuses[platform]"
        :config-loading="configLoading"
        :service-running="status.running"
        :agent-label="agentLabels[platform]"
        :agent-status-text="agentStatusText(agentStatuses[platform])"
        :agent-status-class="agentStatusClass(agentStatuses[platform])"
        :can-apply="canApplyAgent(platform, status.running)"
        :selected-claude-provider="selectedClaudeProvider"
        :claude-provider-keys="claudeProviderKeys"
        :claude-mi-m-o-base-url="claudeMiMoBaseUrl"
        :claude-provider-label="claudeProviderLabel"
        :claude-target-base-url="claudeTargetBaseUrl"
        :selected-codex-provider="selectedCodexProvider"
        :codex-provider-labels="codexProviderLabels"
        :codex-provider-label="codexProviderLabel"
        @apply="handleApply(platform)"
        @restore="handleRestore(platform)"
        @update:selected-claude-provider="selectedClaudeProvider = $event"
        @update:claude-provider-keys="claudeProviderKeys = $event"
        @update:mi-m-o-base-url="claudeMiMoBaseUrl = $event"
        @update:selected-codex-provider="selectedCodexProvider = $event"
      />
    </div>
    <p v-if="actionError" class="text-sm text-destructive-foreground">{{ actionError }}</p>
  </div>
</template>
