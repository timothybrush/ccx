import { ref } from 'vue'
import type { AgentPlatform, AgentProvider, AgentConfigStatus, ApplyAgentConfigRequest } from '@/types'
import {
  GetAgentConfigStatus,
  ApplyAgentConfig,
  RestoreAgentConfig,
} from '@bindings/github.com/BenedictKing/ccx/desktop/desktopservice'

const agentLabels: Record<AgentPlatform, string> = {
  claude: 'Claude Code',
  codex: 'Codex',
}

const claudeProviderLabels: Record<AgentProvider | 'custom', string> = {
  ccx: 'CCX',
  deepseek: 'DeepSeek',
  mimo: 'MiMo',
  openai: 'OpenAI',
  custom: '自定义',
}

const codexProviderLabels: Record<AgentProvider | 'custom', string> = {
  ccx: 'CCX 本地网关',
  openai: 'OpenAI 官方',
  deepseek: 'DeepSeek',
  mimo: 'MiMo',
  custom: '自定义',
}

const agentPlatforms: AgentPlatform[] = ['claude', 'codex']

// Module-level singletons
const agentStatuses = ref<Record<AgentPlatform, AgentConfigStatus | null>>({
  claude: null,
  codex: null,
})
const configLoading = ref(false)
const selectedClaudeProvider = ref<AgentProvider>('ccx')
const claudeProviderKeys = ref<Record<AgentProvider, string>>({
  ccx: '',
  deepseek: '',
  mimo: '',
  openai: '',
})
const claudeMiMoBaseUrl = ref('https://api.mimo.xiaomi.com/v1')
const selectedCodexProvider = ref<AgentProvider>('ccx')

const isClaudeProvider = (value?: string): value is AgentProvider => {
  return value === 'ccx' || value === 'deepseek' || value === 'mimo'
}

const claudeProviderLabel = (value?: string) => {
  if (!value) return '未识别'
  return claudeProviderLabels[value as AgentProvider | 'custom'] || value
}

const codexProviderLabel = (value?: string) => {
  if (!value) return '未识别'
  return codexProviderLabels[value as AgentProvider | 'custom'] || value
}

const claudeTargetBaseUrl = () => {
  switch (selectedClaudeProvider.value) {
    case 'ccx':
      return agentStatuses.value.claude?.targetBaseUrl || '当前 CCX 网关'
    case 'deepseek':
      return 'https://api.deepseek.com/anthropic'
    case 'mimo':
      return claudeMiMoBaseUrl.value || 'https://api.mimo.xiaomi.com/v1'
    default:
      return ''
  }
}

const agentStatusText = (item: AgentConfigStatus | null) => {
  if (!item) return '检测中'
  if (item.configured) return '已配置'
  if (item.needsUpdate) return '端口不匹配'
  return '未配置'
}

const agentStatusClass = (item: AgentConfigStatus | null) => {
  if (!item) return 'starting'
  if (item.configured) return 'running'
  if (item.needsUpdate) return 'starting'
  return 'stopped'
}

const loadAgentStatuses = async () => {
  configLoading.value = true
  try {
    const [claude, codex] = await Promise.all([
      GetAgentConfigStatus('claude') as Promise<AgentConfigStatus>,
      GetAgentConfigStatus('codex') as Promise<AgentConfigStatus>,
    ])
    agentStatuses.value = { claude, codex }
    if (isClaudeProvider(claude.provider)) {
      selectedClaudeProvider.value = claude.provider
    }
    if (claude.provider === 'mimo' && claude.currentBaseUrl) {
      claudeMiMoBaseUrl.value = claude.currentBaseUrl
    }
    if (codex.provider === 'openai') {
      selectedCodexProvider.value = 'openai'
    } else {
      selectedCodexProvider.value = 'ccx'
    }
  } catch (error) {
    // error is handled by caller
  } finally {
    configLoading.value = false
  }
}

const canApplyAgent = (platform: AgentPlatform, serviceRunning: boolean) => {
  if (configLoading.value) return false
  if (platform === 'codex') {
    if (selectedCodexProvider.value === 'openai') return true
    return serviceRunning
  }
  if (selectedClaudeProvider.value === 'ccx') return serviceRunning
  return claudeProviderKeys.value[selectedClaudeProvider.value].trim() !== ''
}

const applyAgent = async (platform: AgentPlatform) => {
  configLoading.value = true
  try {
    const request: ApplyAgentConfigRequest = { platform }
    if (platform === 'claude') {
      request.provider = selectedClaudeProvider.value
      if (selectedClaudeProvider.value !== 'ccx') {
        request.apiKey = claudeProviderKeys.value[selectedClaudeProvider.value].trim()
      }
      if (selectedClaudeProvider.value === 'mimo') {
        request.baseUrl = claudeMiMoBaseUrl.value.trim()
      }
    }
    if (platform === 'codex') {
      request.provider = selectedCodexProvider.value
    }
    await ApplyAgentConfig(request)
    if (platform === 'claude' && selectedClaudeProvider.value !== 'ccx') {
      claudeProviderKeys.value[selectedClaudeProvider.value] = ''
    }
  } finally {
    configLoading.value = false
  }
}

const restoreAgent = async (platform: AgentPlatform) => {
  configLoading.value = true
  try {
    await RestoreAgentConfig(platform)
  } finally {
    configLoading.value = false
  }
}

export function useAgentConfig() {
  return {
    agentStatuses,
    configLoading,
    selectedClaudeProvider,
    claudeProviderKeys,
    claudeMiMoBaseUrl,
    agentLabels,
    claudeProviderLabels,
    codexProviderLabels,
    agentPlatforms,
    isClaudeProvider,
    claudeProviderLabel,
    claudeTargetBaseUrl,
    agentStatusText,
    agentStatusClass,
    loadAgentStatuses,
    canApplyAgent,
    applyAgent,
    restoreAgent,
    selectedCodexProvider,
    codexProviderLabel,
  }
}
