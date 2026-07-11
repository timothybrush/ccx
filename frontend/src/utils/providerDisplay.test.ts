import { describe, expect, it } from 'vitest'
import { providerDisplayName } from './providerDisplay'

describe('providerDisplayName', () => {
  it('保留 provider 品牌的标准大小写', () => {
    expect(providerDisplayName('mimo')).toBe('MiMo')
    expect(providerDisplayName('openai')).toBe('OpenAI')
  })

  it('为未知 provider 生成可读名称', () => {
    expect(providerDisplayName('example-provider')).toBe('Example Provider')
    expect(providerDisplayName()).toBe('')
  })
})
