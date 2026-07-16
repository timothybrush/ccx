import { describe, expect, it } from 'vitest'

import {
  buildQuickAddChannelName,
  defaultQuickAddServiceType,
  inferQuickAddProviderId,
  normalizeQuickAddBaseUrls,
  normalizeDiscoveredChannelKind,
  recognizeQuickAddBaseUrl,
  supportsQuickAddProtocolDiscovery
} from './quickAddChannel'

const providers = [
  {
    providerId: 'glm',
    candidates: [{ baseUrl: 'https://open.bigmodel.cn/api/anthropic' }],
    routes: [
      { candidates: [{ baseUrl: 'https://open.bigmodel.cn/api/anthropic' }] },
      { candidates: [{ baseUrl: 'https://open.bigmodel.cn/api/paas/v4#' }] }
    ]
  },
  {
    providerId: 'deepseek',
    candidates: [{ baseUrl: 'https://api.deepseek.com/anthropic' }],
    routes: [{ candidates: [{ baseUrl: 'https://api.deepseek.com' }] }]
  }
]

describe('buildQuickAddChannelName', () => {
  it('省略域名前导 www 并保留其余主机名', () => {
    expect(buildQuickAddChannelName('https://www.fastaitoken.com/v1', 'ivpp0p')).toBe('fastaitoken-com-ivpp0p')
  })

  it('不误删主机名中间的 www', () => {
    expect(buildQuickAddChannelName('https://api.www-example.com', 'abc123')).toBe('api-www-example-com-abc123')
  })

  it('无效地址回退到通用名称', () => {
    expect(buildQuickAddChannelName('not a url', 'abc123')).toBe('channel-abc123')
  })
})

describe('quick add protocol discovery', () => {
  it('与标准模式一致地清理后台路径和协议端点', () => {
    expect(recognizeQuickAddBaseUrl('https://www.fastaitoken.com/keys', 'messages')).toBe('https://www.fastaitoken.com')
    expect(recognizeQuickAddBaseUrl('https://www.fastaitoken.com/usage', 'messages')).toBe(
      'https://www.fastaitoken.com'
    )
    expect(recognizeQuickAddBaseUrl('https://relay.example.com/v1/responses', 'messages')).toBe(
      'https://relay.example.com'
    )
  })

  it('规范化并去重多个 Base URL', () => {
    expect(
      normalizeQuickAddBaseUrls(['https://api.example.com/keys', 'https://api.example.com', 'not-a-url'], 'responses')
    ).toEqual(['https://api.example.com'])
  })

  it('仅对四类 LLM 协议执行发现', () => {
    expect(supportsQuickAddProtocolDiscovery('messages')).toBe(true)
    expect(supportsQuickAddProtocolDiscovery('responses')).toBe(true)
    expect(supportsQuickAddProtocolDiscovery('images')).toBe(false)
    expect(supportsQuickAddProtocolDiscovery('vectors')).toBe(false)
  })

  it('按渠道类型提供探测所需的默认 serviceType', () => {
    expect(defaultQuickAddServiceType('messages')).toBe('claude')
    expect(defaultQuickAddServiceType('responses')).toBe('responses')
  })

  it('只接受发现接口支持的协议类型', () => {
    expect(normalizeDiscoveredChannelKind('responses')).toBe('responses')
    expect(normalizeDiscoveredChannelKind('images')).toBeNull()
    expect(normalizeDiscoveredChannelKind('')).toBeNull()
  })
})

describe('inferQuickAddProviderId', () => {
  const zhipuKey = '0123456789abcdef0123456789abcdef.ABCDEFGHIJKLMNO1'

  it('识别智谱两个官方协议根及其完整端点', () => {
    expect(inferQuickAddProviderId(providers, ['https://open.bigmodel.cn/api/anthropic'], ['sk-any'])).toBe('glm')
    expect(
      inferQuickAddProviderId(providers, ['https://open.bigmodel.cn/api/paas/v4/chat/completions'], ['sk-any'])
    ).toBe('glm')
  })

  it('没有 Base URL 时按 id.secret Key 识别智谱', () => {
    expect(inferQuickAddProviderId(providers, [''], [zhipuKey])).toBe('glm')
  })

  it('第三方 URL 优先，不能仅凭智谱样式 Key 标为官方', () => {
    expect(inferQuickAddProviderId(providers, ['https://relay.example/v1'], [zhipuKey])).toBe('')
  })

  it('混合官方和第三方 URL 时保持自定义模式', () => {
    expect(
      inferQuickAddProviderId(
        providers,
        ['https://open.bigmodel.cn/api/paas/v4', 'https://relay.example/v1'],
        [zhipuKey]
      )
    ).toBe('')
  })

  it('不会根据共享的 sk- Key 猜测 provider', () => {
    expect(inferQuickAddProviderId(providers, [''], ['sk-abcdefghijklmnopqrstuvwxyz123456'])).toBe('')
  })
})
