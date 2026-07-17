import type { ChannelKind, ChannelProtocolRoute, ManagedAccountChannel } from '@/services/api'

const NATIVE_KIND_BY_SERVICE_TYPE: Record<string, ChannelKind> = {
  claude: 'messages',
  openai: 'chat',
  chat: 'chat',
  responses: 'responses',
  gemini: 'gemini',
  copilot: 'responses',
}

const DISPLAY_ORDER: ChannelKind[] = ['messages', 'chat', 'responses', 'gemini', 'images', 'vectors']

const nativeKindForRoute = (route: ChannelProtocolRoute): ChannelKind => {
  if (route.kind === 'images' || route.kind === 'vectors') return route.kind
  return NATIVE_KIND_BY_SERVICE_TYPE[route.serviceType.trim().toLowerCase()] ?? route.kind
}

/**
 * 将客户端入站路由折叠为上游原生协议，并附加 endpoint profile 的实际模型清单。
 * 保留 kind 作为 CCX 实际入站路由，upstreamKind 只供上游协议展示使用；
 * 同一上游协议存在多条转换路由时，优先保留 kind 与原生协议一致的渠道。
 */
export function buildNativeProtocolModelRoutes(
  routes: ChannelProtocolRoute[] | undefined,
  accountChannels: ManagedAccountChannel[] | undefined,
): ChannelProtocolRoute[] {
  const availabilityByChannel = new Map(
    (accountChannels ?? []).map(channel => [channel.channelUid, channel]),
  )
  const selected = new Map<ChannelKind, { route: ChannelProtocolRoute; native: boolean }>()

  for (const route of routes ?? []) {
    const upstreamKind = nativeKindForRoute(route)
    const native = route.kind === upstreamKind
    const existing = selected.get(upstreamKind)
    if (existing && (existing.native || !native)) continue

    const availability = route.channelUid ? availabilityByChannel.get(route.channelUid) : undefined
    selected.set(upstreamKind, {
      native,
      route: {
        ...route,
        upstreamKind,
        modelInventoryKnown: availability?.modelInventoryKnown,
        discoveredModels: availability?.discoveredModels,
        modelBindings: availability?.modelBindings,
        modelsUpdatedAt: availability?.modelsUpdatedAt,
      },
    })
  }

  return DISPLAY_ORDER.flatMap(kind => {
    const item = selected.get(kind)
    return item ? [item.route] : []
  })
}
