import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({ apiKey: 'test-admin-key' })
}))

function providerTemplatesResponse(providers: unknown[]): Response {
  return {
    ok: true,
    json: vi.fn().mockResolvedValue({ providers })
  } as unknown as Response
}

describe('provider templates cache', () => {
  beforeEach(() => {
    vi.resetModules()
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('预取与实际读取应复用同一个请求', async () => {
    const providers = [{ providerId: 'deepseek', displayName: 'DeepSeek' }]
    const fetchMock = vi.fn().mockResolvedValue(providerTemplatesResponse(providers))
    vi.stubGlobal('fetch', fetchMock)

    const { getProviderTemplates, preloadProviderTemplates } = await import('./autopilot-api')
    const preload = preloadProviderTemplates()

    const [, first, second] = await Promise.all([preload, getProviderTemplates(), getProviderTemplates()])

    expect(fetchMock).toHaveBeenCalledTimes(1)
    expect(first).toEqual(providers)
    expect(second).toEqual(providers)
  })

  it('预取失败后实际读取应重新请求', async () => {
    const providers = [{ providerId: 'mimo', displayName: 'MiMo' }]
    const fetchMock = vi
      .fn()
      .mockRejectedValueOnce(new Error('temporary failure'))
      .mockResolvedValueOnce(providerTemplatesResponse(providers))
    vi.stubGlobal('fetch', fetchMock)

    const { getProviderTemplates, preloadProviderTemplates } = await import('./autopilot-api')

    await preloadProviderTemplates()
    await expect(getProviderTemplates()).resolves.toEqual(providers)
    expect(fetchMock).toHaveBeenCalledTimes(2)
  })
})
