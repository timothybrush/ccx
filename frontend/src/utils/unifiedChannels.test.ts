import { describe, expect, it } from 'vitest'

import type { Channel, ChannelsResponse } from '@/services/api'
import { buildUnifiedChannelsData, type LlmChannelKind } from './unifiedChannels'

const channel = (
  name: string,
  accountUid: string,
  index: number,
  apiKeys: string[],
  overrides: Partial<Channel> = {},
): Channel => ({
  name,
  accountUid,
  channelUid: `ch-${index}`,
  providerId: 'mimo',
  autoManaged: true,
  index,
  serviceType: name.endsWith('-claude') ? 'claude' : 'openai',
  baseUrl: 'https://example.com',
  apiKeys,
  ...overrides,
})

const response = (channels: Channel[]): ChannelsResponse => ({ channels, current: -1 })

describe('buildUnifiedChannelsData account grouping', () => {
  it('优先按 accountUid 聚合多协议渠道，不依赖 Key 指纹', () => {
    const data: Record<LlmChannelKind, ChannelsResponse> = {
      messages: response([channel('mimo-main-claude', 'acct-main', 0, ['sk-a'])]),
      chat: response([channel('mimo-main-chat', 'acct-main', 1, ['sk-a', 'sk-b'])]),
      responses: response([channel('mimo-main-codex', 'acct-main', 2, ['sk-b'])]),
      gemini: response([channel('mimo-main-gemini', 'acct-main', 3, ['sk-a'])]),
    }

    const result = buildUnifiedChannelsData(data)
    expect(result.channels).toHaveLength(1)
    expect(result.channels[0].accountUid).toBe('acct-main')
    expect(result.channels[0].protocolCapsules?.map(item => item.kind)).toEqual([
      'messages',
      'chat',
      'responses',
      'gemini',
    ])
  })

  it('相同 provider 和名称下不同 accountUid 不应合并', () => {
    const data: Record<LlmChannelKind, ChannelsResponse> = {
      messages: response([
        channel('mimo-main-claude', 'acct-a', 0, ['sk-a']),
        channel('mimo-main-claude', 'acct-b', 1, ['sk-b']),
      ]),
      chat: response([]),
      responses: response([]),
      gemini: response([]),
    }

    expect(buildUnifiedChannelsData(data).channels).toHaveLength(2)
  })

  it('新增单协议渠道置顶后保持多协议账号的既有相对顺序', () => {
    const data: Record<LlmChannelKind, ChannelsResponse> = {
      messages: response([
        channel('localhost-37zq4d', 'acct-local', 0, ['sk-local'], { providerId: '', priority: 0 }),
        channel('volcengine-claude', 'acct-volcengine', 1, ['sk-volcengine'], { providerId: 'volcengine', priority: 1 }),
        channel('mimo-claude', 'acct-mimo', 2, ['sk-mimo'], { priority: 2 }),
      ]),
      chat: response([
        channel('volcengine-chat', 'acct-volcengine', 0, ['sk-volcengine'], { providerId: 'volcengine', priority: 0 }),
        channel('mimo-chat', 'acct-mimo', 1, ['sk-mimo'], { priority: 1 }),
        channel('desktop-deepseek-chat', 'acct-deepseek', 34, ['sk-deepseek'], {
          autoManaged: false,
          providerId: '',
          priority: 1,
        }),
      ]),
      responses: response([
        channel('volcengine-codex', 'acct-volcengine', 0, ['sk-volcengine'], { providerId: 'volcengine', priority: 0 }),
        channel('mimo-codex', 'acct-mimo', 1, ['sk-mimo'], { priority: 1 }),
        channel('aixoras-xanqfm', 'acct-aixoras', 2, ['sk-aixoras'], {
          autoManaged: false,
          providerId: '',
          priority: 1,
        }),
      ]),
      gemini: response([
        channel('volcengine-gemini', 'acct-volcengine', 0, ['sk-volcengine'], { providerId: 'volcengine', priority: 0 }),
        channel('mimo-gemini', 'acct-mimo', 1, ['sk-mimo'], { priority: 1 }),
      ]),
    }

    const channels = buildUnifiedChannelsData(data).channels
    const sorted = [...channels].sort((a, b) => (a.priority ?? a.index) - (b.priority ?? b.index))

    expect(sorted.slice(0, 5).map(item => item.name)).toEqual([
      'localhost-37zq4d',
      'volcengine',
      'mimo',
      'desktop-deepseek-chat',
      'aixoras-xanqfm',
    ])
    expect(channels.find(item => item.name === 'mimo')?.priority).toBe(1)
  })

  it('在列表头部插入渠道时保持既有逻辑渠道的展示 key 稳定', () => {
    const original: Record<LlmChannelKind, ChannelsResponse> = {
      messages: response([
        channel('volcengine-claude', 'acct-volcengine', 0, ['sk-volcengine'], { providerId: 'volcengine' }),
        channel('mimo-claude', 'acct-mimo', 1, ['sk-mimo']),
      ]),
      chat: response([]),
      responses: response([]),
      gemini: response([]),
    }
    const withNewChannel: Record<LlmChannelKind, ChannelsResponse> = {
      ...original,
      messages: response([
        channel('localhost-37zq4d', 'acct-local', 0, ['sk-local'], { providerId: '' }),
        ...original.messages.channels,
      ]),
    }

    const originalMimo = buildUnifiedChannelsData(original).channels.find(item => item.name === 'mimo')
    const nextMimo = buildUnifiedChannelsData(withNewChannel).channels.find(item => item.name === 'mimo')

    expect(nextMimo?.displayKey).toBe(originalMimo?.displayKey)
  })
})
