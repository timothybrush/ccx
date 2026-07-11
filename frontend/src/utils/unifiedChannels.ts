import type {
  Channel,
  ChannelKind,
  ChannelMetrics,
  ChannelProtocolCapsule,
  ChannelProtocolRoute,
  ChannelRecentActivity,
  ChannelsResponse,
} from '@/services/api'

export type LlmChannelKind = 'messages' | 'chat' | 'responses' | 'gemini'

export const LLM_CHANNEL_KINDS: LlmChannelKind[] = ['messages', 'chat', 'responses', 'gemini']

const PROTOCOL_LABELS: Record<LlmChannelKind, string> = {
  messages: 'Claude',
  chat: 'Chat',
  responses: 'Codex',
  gemini: 'Gemini',
}

const PROVIDER_ROUTE_SUFFIXES: Record<LlmChannelKind, RegExp> = {
  messages: /-claude$/i,
  chat: /-chat$/i,
  responses: /-codex$/i,
  gemini: /-gemini$/i,
}

const PRIMARY_KIND_ORDER: LlmChannelKind[] = ['messages', 'chat', 'responses', 'gemini']

type RoutedChannel = Channel & {
  routeKind: ChannelKind
  routeIndex: number
  displayKey: string
}

type ChannelGroup = {
  key: string
  logicalName: string
  channels: Partial<Record<LlmChannelKind, RoutedChannel>>
}

export const isLlmChannelKind = (kind: string): kind is LlmChannelKind => {
  return kind === 'messages' || kind === 'chat' || kind === 'responses' || kind === 'gemini'
}

export const protocolLabelForKind = (kind: ChannelKind): string => {
  return isLlmChannelKind(kind) ? PROTOCOL_LABELS[kind] : kind
}

const stripRouteSuffix = (name: string, kind: LlmChannelKind): string => {
  return name.replace(PROVIDER_ROUTE_SUFFIXES[kind], '')
}

const apiKeyFingerprint = (channel: Channel): string => {
  const keys = channel.apiKeys ?? []
  if (!keys.length) return ''
  return keys.map(key => `${key.slice(0, 8)}:${key.slice(-6)}`).sort().join('|')
}

const logicalGroupKey = (kind: LlmChannelKind, channel: Channel): { key: string; name: string } => {
  const name = channel.autoManaged && channel.providerId
    ? stripRouteSuffix(channel.name, kind)
    : channel.name

  if (!channel.autoManaged || !channel.providerId) {
    return {
      key: `${kind}:${channel.index}:${channel.channelUid || channel.name}`,
      name,
    }
  }

  return {
    key: ['provider', channel.providerId, name, apiKeyFingerprint(channel)].join(':'),
    name,
  }
}

const annotateChannel = (kind: LlmChannelKind, channel: Channel): RoutedChannel => ({
  ...channel,
  routeKind: kind,
  routeIndex: channel.index,
  displayKey: `${kind}:${channel.index}:${channel.channelUid || channel.name}`,
})

const selectPrimary = (channels: Partial<Record<LlmChannelKind, RoutedChannel>>): RoutedChannel => {
  for (const kind of PRIMARY_KIND_ORDER) {
    const channel = channels[kind]
    if (channel) return channel
  }
  return Object.values(channels)[0] as RoutedChannel
}

const buildProtocolCapsules = (channels: Partial<Record<LlmChannelKind, RoutedChannel>>): ChannelProtocolCapsule[] => {
  return PRIMARY_KIND_ORDER.flatMap(kind => {
    const channel = channels[kind]
    if (!channel) return []
    return [{
      kind,
      label: PROTOCOL_LABELS[kind],
      serviceType: channel.serviceType,
      channelUid: channel.channelUid,
      index: channel.routeIndex,
      status: channel.status,
    }]
  })
}

const buildProtocolRoutes = (channels: Partial<Record<LlmChannelKind, RoutedChannel>>): ChannelProtocolRoute[] => {
  return PRIMARY_KIND_ORDER.flatMap(kind => {
    const channel = channels[kind]
    if (!channel) return []
    return [{
      kind,
      index: channel.routeIndex,
      name: channel.name,
      serviceType: channel.serviceType,
      channelUid: channel.channelUid,
    }]
  })
}

const buildDisplayChannel = (group: ChannelGroup, displayOrder: number): Channel => {
  const primary = selectPrimary(group.channels)

  return {
    ...primary,
    index: primary.routeIndex,
    name: group.logicalName,
    logicalName: group.logicalName,
    routeKind: primary.routeKind,
    routeIndex: primary.routeIndex,
    displayKey: `logical:${displayOrder}:${group.key}`,
    protocolCapsules: buildProtocolCapsules(group.channels),
    protocolRoutes: buildProtocolRoutes(group.channels),
  }
}

export const buildUnifiedChannelsData = (
  dataByKind: Record<LlmChannelKind, ChannelsResponse>
): ChannelsResponse => {
  const groups = new Map<string, ChannelGroup>()

  for (const kind of LLM_CHANNEL_KINDS) {
    for (const channel of dataByKind[kind].channels ?? []) {
      const routed = annotateChannel(kind, channel)
      const { key, name } = logicalGroupKey(kind, channel)
      const group = groups.get(key) ?? { key, logicalName: name, channels: {} }
      group.channels[kind] = routed
      groups.set(key, group)
    }
  }

  const channels = Array.from(groups.values()).map(buildDisplayChannel)
  return {
    channels,
    current: channels[0]?.index ?? -1,
  }
}

export const withRouteKindMetrics = (
  kind: LlmChannelKind,
  metrics: ChannelMetrics[]
): ChannelMetrics[] => metrics.map(metric => ({ ...metric, routeKind: kind }))

export const withRouteKindActivity = (
  kind: LlmChannelKind,
  activity: ChannelRecentActivity[] | undefined
): ChannelRecentActivity[] => (activity ?? []).map(item => ({ ...item, routeKind: kind }))
