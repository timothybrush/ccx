import type { ConversationInfo } from '@/services/api'

export type BoardColumnKey = 'working' | 'idle'

export interface SubagentSummary {
  total: number
  streaming: number
  active: number
  idle: number
}

export interface ConversationBoardItem {
  conversation: ConversationInfo
  subagents: ConversationInfo[]
  aggregateStatus: BoardColumnKey
  subagentSummary: SubagentSummary
}

export function buildConversationBoardItems(items: ConversationInfo[]): ConversationBoardItem[] {
  const conversations = deduplicateConversations(items)
  const byID = new Map(conversations.map(item => [item.id, item]))
  const childrenByParent = new Map<string, ConversationInfo[]>()
  const referencedChildIDs = new Set<string>()

  for (const item of conversations) {
    for (const childID of item.childConversationIds ?? []) {
      const child = byID.get(normalizeConversationID(childID))
      if (!child || child.id === item.id) continue
      referencedChildIDs.add(child.id)
      appendChild(childrenByParent, item.id, child)
    }
  }

  for (const item of conversations) {
    const parentID = normalizeConversationID(item.parentConversationId)
    if (!parentID || !byID.has(parentID)) continue
    appendChild(childrenByParent, parentID, item)
  }

  const roots = conversations.filter(item => !hasExistingParent(item, byID, referencedChildIDs))
  return roots.map(conversation => {
    const subagents = collectSubagents(conversation, childrenByParent)
    const subagentSummary = summarizeSubagents(subagents)
    return {
      conversation,
      subagents,
      subagentSummary,
      aggregateStatus: getAggregateStatus(conversation, subagents),
    }
  })
}

function deduplicateConversations(items: ConversationInfo[]): ConversationInfo[] {
  const seen = new Set<string>()
  const result: ConversationInfo[] = []

  for (const item of items) {
    const id = normalizeConversationID(item.id)
    if (!id || seen.has(id)) continue
    seen.add(id)
    result.push(id === item.id ? item : { ...item, id })
  }

  return result
}

function normalizeConversationID(id?: string): string {
  return String(id ?? '').trim()
}

function appendChild(childrenByParent: Map<string, ConversationInfo[]>, parentID: string, child: ConversationInfo) {
  const children = childrenByParent.get(parentID) ?? []
  if (!children.some(item => item.id === child.id)) children.push(child)
  childrenByParent.set(parentID, children)
}

function hasExistingParent(item: ConversationInfo, byID: Map<string, ConversationInfo>, referencedChildIDs: Set<string>): boolean {
  const parentID = normalizeConversationID(item.parentConversationId)
  if (parentID && byID.has(parentID)) return true
  return referencedChildIDs.has(item.id)
}

export function buildConversationColumnBuckets(items: ConversationInfo[]): Record<BoardColumnKey, ConversationBoardItem[]> {
  const buckets: Record<BoardColumnKey, ConversationBoardItem[]> = {
    working: [],
    idle: [],
  }

  for (const item of buildConversationBoardItems(items)) {
    buckets[item.aggregateStatus].push(item)
  }

  return buckets
}

export function filterConversationBoardItems(items: ConversationBoardItem[], kindFilter: string, searchQuery: string): ConversationBoardItem[] {
  const kind = kindFilter.trim()
  const query = searchQuery.trim().toLowerCase()

  return items.filter(item => {
    const related = [item.conversation, ...item.subagents]
    if (kind && !related.some(conversation => conversation.kind === kind)) return false
    if (!query) return true
    return related.some(matchesConversationSearch(query))
  })
}

export function getConversationBoardColumnKey(conversation: ConversationInfo, subagents: ConversationInfo[] = []): BoardColumnKey {
  if (conversation.status !== 'idle') return 'working'
  if (subagents.some(item => item.status !== 'idle')) return 'working'
  return 'idle'
}

function collectSubagents(root: ConversationInfo, childrenByParent: Map<string, ConversationInfo[]>): ConversationInfo[] {
  const result: ConversationInfo[] = []
  const pending = [...(childrenByParent.get(root.id) ?? [])]
  const seen = new Set<string>()

  while (pending.length > 0) {
    const child = pending.shift()
    if (!child || seen.has(child.id)) continue
    seen.add(child.id)
    result.push(child)
    pending.push(...(childrenByParent.get(child.id) ?? []))
  }

  return result.sort((a, b) => new Date(b.lastActiveAt).getTime() - new Date(a.lastActiveAt).getTime())
}

function summarizeSubagents(subagents: ConversationInfo[]): SubagentSummary {
  return subagents.reduce<SubagentSummary>((summary, item) => {
    summary.total += 1
    if (item.status === 'streaming') summary.streaming += 1
    else if (item.status === 'idle') summary.idle += 1
    else summary.active += 1
    return summary
  }, { total: 0, streaming: 0, active: 0, idle: 0 })
}

function matchesConversationSearch(query: string): (conversation: ConversationInfo) => boolean {
  return conversation =>
    (conversation.title || '').toLowerCase().includes(query) ||
    (conversation.userId || '').toLowerCase().includes(query) ||
    (conversation.rawUserId || '').toLowerCase().includes(query) ||
    (conversation.lastModel || '').toLowerCase().includes(query) ||
    (conversation.channelName || '').toLowerCase().includes(query)
}

function getAggregateStatus(conversation: ConversationInfo, subagents: ConversationInfo[]): BoardColumnKey {
  if (conversation.status !== 'idle') return 'working'
  if (subagents.some(item => item.status !== 'idle')) return 'working'
  return 'idle'
}
