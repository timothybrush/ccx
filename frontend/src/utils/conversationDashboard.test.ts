import { describe, expect, it } from 'vitest'
import type { ConversationInfo } from '@/services/api'
import { buildConversationBoardItems, buildConversationColumnBuckets, getConversationBoardColumnKey } from './conversationDashboard'

function conversation(overrides: Partial<ConversationInfo>): ConversationInfo {
  return {
    id: overrides.id ?? 'conv-1',
    kind: overrides.kind ?? 'messages',
    userId: overrides.userId ?? 'user',
    createdAt: '2026-01-01T00:00:00Z',
    lastActiveAt: '2026-01-01T00:00:00Z',
    requestCount: 1,
    models: ['model'],
    currentChannel: 0,
    channelName: 'primary',
    status: overrides.status ?? 'active',
    lastModel: 'model',
    lastRequestId: '',
    ...overrides,
  }
}

describe('conversation dashboard columns', () => {
  it('marks a streaming conversation as working', () => {
    const item = conversation({ status: 'streaming', hasSubagents: true })

    expect(getConversationBoardColumnKey(item)).toBe('working')
  })

  it('groups child subagents under one root conversation', () => {
    const items = buildConversationBoardItems([
      conversation({ id: 'root', title: 'Root', status: 'idle', childConversationIds: ['sub-1', 'sub-2'] }),
      conversation({ id: 'sub-1', title: 'Sub One', parentConversationId: 'root', status: 'streaming' }),
      conversation({ id: 'sub-2', title: 'Sub Two', parentConversationId: 'root', status: 'active' }),
    ])

    expect(items).toHaveLength(1)
    expect(items[0].conversation.id).toBe('root')
    expect(items[0].aggregateStatus).toBe('working')
    expect(items[0].subagents.map(item => item.id)).toEqual(['sub-1', 'sub-2'])
    expect(items[0].subagentSummary).toEqual({ total: 2, streaming: 1, active: 1, idle: 0 })
  })

  it('skips empty IDs and keeps the first duplicate conversation', () => {
    const items = buildConversationBoardItems([
      conversation({ id: '', title: 'Empty ID' }),
      conversation({ id: 'dup', title: 'First Duplicate' }),
      conversation({ id: 'dup', title: 'Second Duplicate' }),
      conversation({ id: ' valid ', title: 'Trimmed ID' }),
    ])

    expect(items.map(item => item.conversation.id)).toEqual(['dup', 'valid'])
    expect(items.map(item => item.conversation.title)).toEqual(['First Duplicate', 'Trimmed ID'])
  })

  it('buckets root conversations into working and idle columns', () => {
    const buckets = buildConversationColumnBuckets([
      conversation({ id: 'streaming', status: 'streaming' }),
      conversation({ id: 'subagents', hasSubagents: true, status: 'active' }),
      conversation({ id: 'idle', status: 'idle' }),
    ])

    expect(buckets.working.map(item => item.conversation.id)).toEqual(['streaming', 'subagents'])
    expect(buckets.idle.map(item => item.conversation.id)).toEqual(['idle'])
  })
})
