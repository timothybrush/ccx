import { createI18n } from 'vue-i18n'
import en from '@/locales/en.json'
import zhCN from '@/locales/zh-CN.json'
import id from '@/locales/id.json'
import { getRuntimeLocale } from './core'

const i18n = createI18n({
  legacy: false,
  locale: getRuntimeLocale(),
  fallbackLocale: 'en',
  messages: { en, 'zh-CN': zhCN, id },
  missingWarn: false,
  fallbackWarn: false,
})

// 挂载到 globalThis 供非组件上下文的 translate() 使用
globalThis.__CCX_I18N__ = i18n

export default i18n
