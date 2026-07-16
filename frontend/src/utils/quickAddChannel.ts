import type { Channel, ChannelDiscoveryKind, ChannelKind } from '@/services/api-types'
import { isZhipuApiKey, parseQuickInput } from './quickInputParser'

interface QuickAddProviderCandidate {
  baseUrl: string
}

interface QuickAddProviderRoute {
  candidates?: QuickAddProviderCandidate[]
}

export interface QuickAddProviderTemplate {
  providerId: string
  aliases?: string[]
  candidates?: QuickAddProviderCandidate[]
  routes?: QuickAddProviderRoute[]
}

const DISCOVERABLE_CHANNEL_KINDS = new Set<ChannelDiscoveryKind>(['messages', 'chat', 'responses', 'gemini'])

const DEFAULT_SERVICE_TYPES: Record<ChannelKind, Channel['serviceType']> = {
  messages: 'claude',
  chat: 'openai',
  responses: 'responses',
  gemini: 'gemini',
  images: 'openai',
  vectors: 'openai'
}

export function buildQuickAddChannelName(baseUrl: string, suffix: string): string {
  try {
    const hostname = new URL(baseUrl).hostname.replace(/^www\./i, '').replace(/\./g, '-')
    return `${hostname || 'channel'}-${suffix}`
  } catch {
    return `channel-${suffix}`
  }
}

export function supportsQuickAddProtocolDiscovery(kind: ChannelKind): kind is ChannelDiscoveryKind {
  return DISCOVERABLE_CHANNEL_KINDS.has(kind as ChannelDiscoveryKind)
}

export function defaultQuickAddServiceType(kind: ChannelKind): Channel['serviceType'] {
  return DEFAULT_SERVICE_TYPES[kind]
}

export function recognizeQuickAddBaseUrl(rawUrl: string, kind: ChannelKind): string {
  return parseQuickInput(rawUrl, defaultQuickAddServiceType(kind)).detectedBaseUrl
}

export function normalizeQuickAddBaseUrls(rawUrls: string[], kind: ChannelKind): string[] {
  return parseQuickInput(rawUrls.join('\n'), defaultQuickAddServiceType(kind)).detectedBaseUrls
}

function effectiveURLPort(url: URL): string {
  if (url.port) return url.port
  if (url.protocol === 'https:') return '443'
  if (url.protocol === 'http:') return '80'
  return ''
}

function inferProviderFromBaseUrl(providers: QuickAddProviderTemplate[], rawBaseUrl: string): string {
  let target: URL
  try {
    target = new URL(rawBaseUrl.trim().replace(/#$/, ''))
  } catch {
    return ''
  }

  const targetPath = target.pathname.replace(/\/+$/, '')
  let bestProviderId = ''
  let bestPathLength = -1
  for (const provider of providers) {
    const candidates = [
      ...(provider.candidates ?? []),
      ...(provider.routes ?? []).flatMap(route => route.candidates ?? [])
    ]
    for (const candidate of candidates) {
      let candidateUrl: URL
      try {
        candidateUrl = new URL(candidate.baseUrl.trim().replace(/#$/, ''))
      } catch {
        continue
      }
      if (
        target.hostname.toLowerCase() !== candidateUrl.hostname.toLowerCase() ||
        effectiveURLPort(target) !== effectiveURLPort(candidateUrl)
      ) {
        continue
      }
      const candidatePath = candidateUrl.pathname.replace(/\/+$/, '')
      if (candidatePath && targetPath !== candidatePath && !targetPath.startsWith(`${candidatePath}/`)) continue
      if (candidatePath.length > bestPathLength) {
        bestProviderId = provider.providerId
        bestPathLength = candidatePath.length
      }
    }
  }
  return bestProviderId
}

/**
 * 仅在所有非空输入都指向同一个已知 provider 时自动识别。
 * 有非模板 URL 时不会再根据 Key 样式推断，避免把自定义渠道错标为已知 Provider。
 */
export function inferQuickAddProviderId(
  providers: QuickAddProviderTemplate[],
  rawBaseUrls: string[],
  rawApiKeys: string[]
): string {
  const baseUrls = rawBaseUrls.map(value => value.trim()).filter(Boolean)
  if (baseUrls.length > 0) {
    let providerId = ''
    for (const baseUrl of baseUrls) {
      const inferred = inferProviderFromBaseUrl(providers, baseUrl)
      if (!inferred || (providerId && providerId !== inferred)) return ''
      providerId = inferred
    }
    return providerId
  }

  const apiKeys = rawApiKeys.map(value => value.trim()).filter(Boolean)
  if (apiKeys.length > 0 && apiKeys.every(isZhipuApiKey) && providers.some(provider => provider.providerId === 'glm')) {
    return 'glm'
  }
  return ''
}

export function normalizeDiscoveredChannelKind(kind: string): ChannelDiscoveryKind | null {
  return DISCOVERABLE_CHANNEL_KINDS.has(kind as ChannelDiscoveryKind) ? (kind as ChannelDiscoveryKind) : null
}
