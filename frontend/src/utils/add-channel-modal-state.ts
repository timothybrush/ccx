import type { Channel } from '@/services/api'
import { deduplicateEquivalentBaseUrls, type ServiceType } from './baseUrlSemantics'

export type ChannelWatcherAction = 'load-edit-channel' | 'reset-new-form' | 'noop'

export function resolveChannelWatcherAction(params: {
  show: boolean
  newChannel: Channel | null | undefined
  oldChannel: Channel | null | undefined
}): ChannelWatcherAction {
  const { show, newChannel, oldChannel } = params

  if (!show) {
    return 'noop'
  }

  if (newChannel) {
    if (oldChannel && newChannel.index === oldChannel.index) {
      return 'noop'
    }
    return 'load-edit-channel'
  }

  if (oldChannel) {
    return 'noop'
  }

  return 'reset-new-form'
}

export function syncBaseUrlsFormState(rawText: string, serviceType: ServiceType): {
  baseUrl: string
  baseUrls: string[]
} {
  const rawUrls = rawText
    .split('\n')
    .map(s => s.trim())
    .filter(Boolean)
  const deduplicated = deduplicateEquivalentBaseUrls(rawUrls, serviceType)

  if (deduplicated.length === 0) {
    return { baseUrl: '', baseUrls: [] }
  }

  if (deduplicated.length === 1) {
    return { baseUrl: deduplicated[0], baseUrls: [] }
  }

  return {
    baseUrl: deduplicated[0],
    baseUrls: deduplicated
  }
}

// 模型名合法字符：字母、数字、点、下划线、连字符、冒号、斜杠，外加通配符 * 与排除前缀 !
// 显式排除中文顿号（、）、逗号、分号、竖线等分隔符与其他标点，避免被当作单条规则保留
const SUPPORTED_MODEL_TOKEN_CHARS = /^[A-Za-z0-9._:/*!-]+$/

// 模型规则的分隔符：空白、中文顿号、逗号（中英文）、分号（中英文）、竖线、换行
const SUPPORTED_MODEL_SEPARATORS = /[\s、,，;；|]+/

/**
 * 将用户手动输入的原始文本按合法分隔符拆分为独立规则。
 * 例如 `GPT-5*、ada*` -> ['GPT-5*', 'ada*']。
 */
export function parseSupportedModelInput(raw: string): string[] {
  return raw
    .split(SUPPORTED_MODEL_SEPARATORS)
    .map(s => s.trim())
    .filter(Boolean)
}

export function isValidSupportedModelPattern(pattern: string): boolean {
  const trimmed = pattern.trim()
  if (!trimmed) {
    return false
  }

  // 仅允许模型名合法字符集；含顿号等非法字符直接拒绝
  if (!SUPPORTED_MODEL_TOKEN_CHARS.test(trimmed)) {
    return false
  }

  if ((trimmed.match(/!/g) || []).length > 1) {
    return false
  }

  const normalized = trimmed.startsWith('!') ? trimmed.slice(1).trim() : trimmed
  if (!normalized || normalized.startsWith('!')) {
    return false
  }

  const starCount = (normalized.match(/\*/g) || []).length
  if (starCount === 0) {
    return true
  }
  if (normalized === '*') {
    return true
  }
  if (starCount === 1) {
    return normalized.startsWith('*') || normalized.endsWith('*')
  }
  if (starCount === 2) {
    return normalized.startsWith('*') && normalized.endsWith('*') && normalized.replace(/\*/g, '') !== ''
  }
  return false
}

export function filterValidSupportedModelPatterns(patterns: string[]): {
  validPatterns: string[]
  hasInvalidPatterns: boolean
} {
  // 先按分隔符拆分（兼容用户把多个规则粘进同一项的情况），再做合法性校验
  const normalized = patterns.flatMap(parseSupportedModelInput)

  const validPatterns = normalized.filter(isValidSupportedModelPattern)
  return {
    validPatterns,
    hasInvalidPatterns: validPatterns.length !== normalized.length
  }
}
