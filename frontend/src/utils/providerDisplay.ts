const PROVIDER_BRAND_NAMES: Record<string, string> = {
  mimo: 'MiMo',
  openai: 'OpenAI',
  deepseek: 'DeepSeek',
  gemini: 'Gemini',
  anthropic: 'Anthropic',
}

export const providerDisplayName = (providerId?: string): string => {
  const normalized = providerId?.trim().toLowerCase() ?? ''
  if (!normalized) return ''
  if (PROVIDER_BRAND_NAMES[normalized]) return PROVIDER_BRAND_NAMES[normalized]

  return normalized
    .split(/[-_\s]+/)
    .filter(Boolean)
    .map(part => part.charAt(0).toUpperCase() + part.slice(1))
    .join(' ')
}
