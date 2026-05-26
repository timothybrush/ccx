import { Browser } from '@wailsio/runtime'

export const providerPromotionLinks: Record<string, string> = {
  compshare: 'https://www.compshare.cn/?ytag=GPU_YY_git_ccx',
}

export function openExternalLink(url: string) {
  return Browser.OpenURL(url)
}

export function openProviderPromotion(provider: string) {
  const url = providerPromotionLinks[provider]
  if (!url) return Promise.resolve()
  return openExternalLink(url)
}
