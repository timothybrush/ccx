import { Browser } from '@wailsio/runtime'

export const providerConsoleLinks: Record<string, string> = {
  deepseek: 'https://platform.deepseek.com/usage',
  mimo: 'https://platform.xiaomimimo.com/console/balance',
  compshare: 'https://console.compshare.cn/light-gpu/model-subscription',
  runapi: 'https://runapi.co/console',
  unity2: 'https://unity2.ai/dashboard',
  'tencent-lkeap': 'https://console.cloud.tencent.com/lkeap/token-plan',
  kimi: 'https://platform.moonshot.cn/console/account',
  'volc-ark': 'https://console.volcengine.com/ark',
  qianfan: 'https://console.bce.baidu.com/qianfan/resource/subscribe',
  originrouter: 'https://easytransnote.com/ai/console/#key',
  glm: 'https://open.bigmodel.cn/coding-plan/personal/overview',
  minimax: 'https://platform.minimaxi.com/user-center/payment/balance',
  dashscope: 'https://bailian.console.aliyun.com/cn-beijing?tab=model#/api-key',
  'opencode-zen': 'https://opencode.ai/',
  'opencode-go': 'https://opencode.ai/',
  openrouter: 'https://openrouter.ai/keys',
  modelscope: 'https://modelscope.cn/my/myaccesstoken',
  openai: 'https://platform.openai.com',
  xfyun: 'https://console.xfyun.cn/',
}

export const providerPromotionLinks: Record<string, string> = {
  compshare: 'https://www.compshare.cn/?ytag=GPU_YY_git_ccx',
  runapi: 'https://runapi.co/register?aff=CqQO',
  unity2: 'https://unity2.ai/register?source=ccx',
}

export function openExternalLink(url: string) {
  return Browser.OpenURL(url)
}

export function openProviderConsole(provider: string) {
  const url = providerConsoleLinks[provider]
  if (!url) return Promise.resolve()
  return openExternalLink(url)
}

export function openProviderPromotion(provider: string) {
  const url = providerPromotionLinks[provider]
  if (!url) return Promise.resolve()
  return openExternalLink(url)
}
