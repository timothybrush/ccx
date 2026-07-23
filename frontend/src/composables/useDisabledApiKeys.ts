import { computed, ref, type ComputedRef } from 'vue'
import { ApiService, type Channel } from '../services/api'

type ChannelType = 'messages' | 'chat' | 'responses' | 'gemini' | 'images' | 'vectors'
type FormLike = {
  apiKeys: string[]
}

type DisabledApiKeyOptions = {
  apiService: ApiService
  channel: ComputedRef<Channel | null | undefined>
  channelType: ComputedRef<ChannelType>
  emitError: (message: string) => void
  form: FormLike
}

export function useDisabledApiKeys(options: DisabledApiKeyOptions) {
  const restoringKey = ref('')
  const localRestoredKeys = ref(new Set<string>())
  const restoringKeyModel = ref('')
  const localRestoredKeyModels = ref(new Set<string>())
  const suspendingKey = ref('')
  const localSuspendedKeys = ref(new Set<string>())
  const localResumedKeys = ref(new Set<string>())

  const keyModelKey = (apiKey: string, model: string) => `${apiKey}|${model}`

  const disabledKeys = computed(() => options.channel.value?.disabledApiKeys || [])
  const visibleDisabledKeys = computed(() =>
    (options.channel.value?.disabledApiKeys || []).filter(dk => !localRestoredKeys.value.has(dk.key))
  )

  const disabledKeyModels = computed(() => options.channel.value?.disabledKeyModels || [])
  const visibleDisabledKeyModels = computed(() =>
    (options.channel.value?.disabledKeyModels || []).filter(
      dm => !localRestoredKeyModels.value.has(keyModelKey(dm.key, dm.model))
    )
  )

  const resetRestoredKeys = () => {
    localRestoredKeys.value = new Set<string>()
    restoringKey.value = ''
    localRestoredKeyModels.value = new Set<string>()
    restoringKeyModel.value = ''
    localSuspendedKeys.value = new Set<string>()
    localResumedKeys.value = new Set<string>()
    suspendingKey.value = ''
  }

  const restoreDisabledKey = async (apiKey: string) => {
    const channel = options.channel.value
    if (!channel || restoringKey.value) return
    restoringKey.value = apiKey
    try {
      const channelId = channel.index
      switch (options.channelType.value) {
        case 'chat':
          await options.apiService.restoreChatApiKey(channelId, apiKey)
          break
        case 'images':
          await options.apiService.restoreImagesApiKey(channelId, apiKey)
          break
        case 'vectors':
          await options.apiService.restoreVectorsApiKey(channelId, apiKey)
          break
        case 'gemini':
          await options.apiService.restoreGeminiApiKey(channelId, apiKey)
          break
        case 'responses':
          await options.apiService.restoreResponsesApiKey(channelId, apiKey)
          break
        default:
          await options.apiService.restoreApiKey(channelId, apiKey)
      }
      localRestoredKeys.value = new Set([...localRestoredKeys.value, apiKey])
      if (!options.form.apiKeys.includes(apiKey)) {
        options.form.apiKeys = [...options.form.apiKeys, apiKey]
      }
    } catch (error) {
      options.emitError(error instanceof Error ? error.message : 'Restore failed')
    } finally {
      restoringKey.value = ''
    }
  }

  const restoreDisabledKeyModel = async (apiKey: string, model: string) => {
    const channel = options.channel.value
    const key = keyModelKey(apiKey, model)
    if (!channel || restoringKeyModel.value) return
    restoringKeyModel.value = key
    try {
      const channelId = channel.index
      switch (options.channelType.value) {
        case 'chat':
          await options.apiService.restoreChatKeyModel(channelId, apiKey, model)
          break
        case 'images':
          await options.apiService.restoreImagesKeyModel(channelId, apiKey, model)
          break
        case 'vectors':
          await options.apiService.restoreVectorsKeyModel(channelId, apiKey, model)
          break
        case 'gemini':
          await options.apiService.restoreGeminiKeyModel(channelId, apiKey, model)
          break
        case 'responses':
          await options.apiService.restoreResponsesKeyModel(channelId, apiKey, model)
          break
        default:
          await options.apiService.restoreKeyModel(channelId, apiKey, model)
      }
      localRestoredKeyModels.value = new Set([...localRestoredKeyModels.value, key])
    } catch (error) {
      options.emitError(error instanceof Error ? error.message : 'Restore failed')
    } finally {
      restoringKeyModel.value = ''
    }
  }

  const suspendKey = async (apiKey: string) => {
    const channel = options.channel.value
    if (!channel || suspendingKey.value) return
    suspendingKey.value = apiKey
    try {
      const channelId = channel.index
      switch (options.channelType.value) {
        case 'chat':
          await options.apiService.suspendChatApiKey(channelId, apiKey)
          break
        case 'images':
          await options.apiService.suspendImagesApiKey(channelId, apiKey)
          break
        case 'vectors':
          await options.apiService.suspendVectorsApiKey(channelId, apiKey)
          break
        case 'gemini':
          await options.apiService.suspendGeminiApiKey(channelId, apiKey)
          break
        case 'responses':
          await options.apiService.suspendResponsesApiKey(channelId, apiKey)
          break
        default:
          await options.apiService.suspendApiKey(channelId, apiKey)
      }
      localSuspendedKeys.value = new Set([...localSuspendedKeys.value, apiKey])
      localResumedKeys.value.delete(apiKey)
    } catch (error) {
      options.emitError(error instanceof Error ? error.message : 'Suspend failed')
    } finally {
      suspendingKey.value = ''
    }
  }

  const resumeKey = async (apiKey: string) => {
    const channel = options.channel.value
    if (!channel || suspendingKey.value) return
    suspendingKey.value = apiKey
    try {
      const channelId = channel.index
      switch (options.channelType.value) {
        case 'chat':
          await options.apiService.resumeChatApiKey(channelId, apiKey)
          break
        case 'images':
          await options.apiService.resumeImagesApiKey(channelId, apiKey)
          break
        case 'vectors':
          await options.apiService.resumeVectorsApiKey(channelId, apiKey)
          break
        case 'gemini':
          await options.apiService.resumeGeminiApiKey(channelId, apiKey)
          break
        case 'responses':
          await options.apiService.resumeResponsesApiKey(channelId, apiKey)
          break
        default:
          await options.apiService.resumeApiKey(channelId, apiKey)
      }
      localResumedKeys.value = new Set([...localResumedKeys.value, apiKey])
      localSuspendedKeys.value.delete(apiKey)
    } catch (error) {
      options.emitError(error instanceof Error ? error.message : 'Resume failed')
    } finally {
      suspendingKey.value = ''
    }
  }

  return {
    restoringKey,
    localRestoredKeys,
    disabledKeys,
    visibleDisabledKeys,
    resetRestoredKeys,
    restoreDisabledKey,
    restoringKeyModel,
    disabledKeyModels,
    visibleDisabledKeyModels,
    restoreDisabledKeyModel,
    suspendingKey,
    suspendKey,
    resumeKey,
    localSuspendedKeys,
    localResumedKeys,
  }
}
