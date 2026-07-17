import { describe, expect, it } from 'vitest'

import type { ChannelProtocolRoute, ManagedAccountChannel } from '@/services/api'
import { buildNativeProtocolModelRoutes } from './channelModelAvailability'

describe('buildNativeProtocolModelRoutes', () => {
  it('火山只展示原生 Messages 与 Chat，并读取各渠道发现模型', () => {
    const routes: ChannelProtocolRoute[] = [
      { kind: 'messages', index: 7, channelUid: 'ch-messages', name: 'volcengine-claude', serviceType: 'claude' },
      { kind: 'chat', index: 6, channelUid: 'ch-chat', name: 'volcengine-chat', serviceType: 'openai' },
      { kind: 'responses', index: 4, channelUid: 'ch-responses', name: 'volcengine-codex', serviceType: 'openai' },
      { kind: 'gemini', index: 0, channelUid: 'ch-gemini', name: 'volcengine-gemini', serviceType: 'openai' },
    ]
    const channels: ManagedAccountChannel[] = [
      {
        kind: 'messages', channelUid: 'ch-messages', name: 'volcengine-claude', serviceType: 'claude', status: 'active',
        modelInventoryKnown: true,
        discoveredModels: ['glm-5.2', 'deepseek-v4-pro'],
      },
      {
        kind: 'chat', channelUid: 'ch-chat', name: 'volcengine-chat', serviceType: 'openai', status: 'active',
        modelInventoryKnown: true,
        discoveredModels: ['glm-5.2'],
      },
    ]

    const result = buildNativeProtocolModelRoutes(routes, channels)

    expect(result.map(route => route.kind)).toEqual(['messages', 'chat'])
    expect(result.map(route => route.upstreamKind)).toEqual(['messages', 'chat'])
    expect(result[0].discoveredModels).toEqual(['glm-5.2', 'deepseek-v4-pro'])
    expect(result[0].modelInventoryKnown).toBe(true)
    expect(result[1].discoveredModels).toEqual(['glm-5.2'])
  })

  it('缺少同名客户端路由时仍按 serviceType 显示真实上游协议', () => {
    const result = buildNativeProtocolModelRoutes([
      { kind: 'responses', index: 1, channelUid: 'ch-only', name: 'chat-through-responses', serviceType: 'openai' },
    ], [])

    expect(result).toHaveLength(1)
    expect(result[0].kind).toBe('responses')
    expect(result[0].upstreamKind).toBe('chat')
  })

  it('兼容旧 chat 别名，并将 Copilot 归为 Responses 上游', () => {
    const result = buildNativeProtocolModelRoutes([
      { kind: 'responses', index: 1, channelUid: 'ch-chat', name: 'legacy-chat', serviceType: ' CHAT ' },
      { kind: 'gemini', index: 2, channelUid: 'ch-copilot', name: 'copilot-through-gemini', serviceType: 'copilot' },
    ], [])

    expect(result.map(route => route.kind)).toEqual(['responses', 'gemini'])
    expect(result.map(route => route.upstreamKind)).toEqual(['chat', 'responses'])
  })
})
