import { computed, ref } from 'vue'
import { describe, expect, it, vi } from 'vitest'
import type { ApiService, Channel } from '../services/api'
import { useDisabledApiKeys } from './useDisabledApiKeys'

const disabledKey = 'ark-test-disabled-key'

const createChannel = () => ref<Channel | null>({
  index: 3,
  apiKeys: [],
  disabledApiKeys: [{
    key: disabledKey,
    reason: 'rate_limited',
    message: 'temporary limit',
    disabledAt: new Date().toISOString(),
  }],
  } as unknown as Channel)

const createOptions = (restoreApiKey: ApiService['restoreApiKey']) => {
  const channel = createChannel()
  const form = { apiKeys: [] as string[] }
  const state = useDisabledApiKeys({
    apiService: { restoreApiKey } as ApiService,
    channel: computed(() => channel.value),
    channelType: computed(() => 'messages' as const),
    emitError: vi.fn(),
    form,
  })
  return { form, state }
}

describe('useDisabledApiKeys', () => {
  it('restores the key and updates the visible blacklist immediately', async () => {
    const restoreApiKey = vi.fn().mockResolvedValue(undefined)
    const { form, state } = createOptions(restoreApiKey)

    const restore = state.restoreDisabledKey(disabledKey)
    expect(state.restoringKey.value).toBe(disabledKey)

    await restore

    expect(restoreApiKey).toHaveBeenCalledWith(3, disabledKey)
    expect(form.apiKeys).toEqual([disabledKey])
    expect(state.visibleDisabledKeys.value).toEqual([])
    expect(state.restoringKey.value).toBe('')
  })

  it('ignores a second key restore while the first request is pending', async () => {
    let resolveRestore!: () => void
    const restoreApiKey = vi.fn(() => new Promise<void>(resolve => {
      resolveRestore = resolve
    }))
    const { state } = createOptions(restoreApiKey)

    const firstRestore = state.restoreDisabledKey(disabledKey)
    const secondRestore = state.restoreDisabledKey(disabledKey)

    expect(restoreApiKey).toHaveBeenCalledTimes(1)
    resolveRestore()
    await Promise.all([firstRestore, secondRestore])
  })
})
