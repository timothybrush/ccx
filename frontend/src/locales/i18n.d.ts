/**
 * 由 vue-i18n 自动消费的类型声明。
 * 从 zh-CN.json 推导所有 key，为 t() 提供自动补全和类型检查。
 */
import zhCN from './zh-CN.json'

type MessageSchema = typeof zhCN

declare module 'vue-i18n' {
  export type DefineLocaleMessage = MessageSchema
}
