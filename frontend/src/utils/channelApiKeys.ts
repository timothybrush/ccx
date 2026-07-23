import type { DisabledKeyInfo, APIKeyConfig } from '../services/api-types'

export interface ChannelApiKeyRow {
  key: string
  activeIndex: number
  disabled?: DisabledKeyInfo
  /** undefined = 默认活跃，true = 显式启用，false = 手动暂停 */
  enabled?: boolean
}

export function buildChannelApiKeyRows(
  apiKeys: string[] | null | undefined = [],
  disabledKeys: DisabledKeyInfo[] | null | undefined = [],
  apiKeyConfigs?: APIKeyConfig[] | null,
): ChannelApiKeyRow[] {
  const activeKeys = apiKeys ?? []
  const disabledItems = disabledKeys ?? []
  const disabledByKey = new Map(disabledItems.filter(item => item.key).map(item => [item.key, item]))

  const enabledByKey = new Map<string, boolean | undefined>()
  for (const cfg of apiKeyConfigs ?? []) {
    enabledByKey.set(cfg.key, cfg.enabled)
  }

  const seen = new Set<string>()
  const rows: ChannelApiKeyRow[] = []

  activeKeys.forEach((key, activeIndex) => {
    if (!key || seen.has(key)) return
    seen.add(key)
    rows.push({
      key,
      activeIndex,
      disabled: disabledByKey.get(key),
      enabled: enabledByKey.get(key),
    })
  })

  for (const disabled of disabledItems) {
    if (!disabled.key || seen.has(disabled.key)) continue
    seen.add(disabled.key)
    rows.push({
      key: disabled.key,
      activeIndex: -1,
      disabled,
      enabled: enabledByKey.get(disabled.key),
    })
  }

  return rows
}

type ChannelKeyState = {
  apiKeys?: string[] | null
  disabledApiKeys?: DisabledKeyInfo[] | null
}

export function availableChannelApiKeyCount(channel: ChannelKeyState): number {
  return buildChannelApiKeyRows(channel.apiKeys, channel.disabledApiKeys).filter(row => !row.disabled).length
}

export function disabledChannelApiKeyCount(channel: ChannelKeyState): number {
  return buildChannelApiKeyRows(channel.apiKeys, channel.disabledApiKeys).filter(row => !!row.disabled).length
}
