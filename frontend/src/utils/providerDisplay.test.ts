import { describe, expect, it } from 'vitest'
import { isManagedProviderChannel, providerDisplayName } from './providerDisplay'

describe('providerDisplayName', () => {
  it('保留 provider 品牌的标准大小写', () => {
    expect(providerDisplayName('mimo')).toBe('MiMo')
    expect(providerDisplayName('volcengine')).toBe('火山方舟')
    expect(providerDisplayName('openai')).toBe('OpenAI')
  })

  it('为未知 provider 生成可读名称', () => {
    expect(providerDisplayName('example-provider')).toBe('Example Provider')
    expect(providerDisplayName()).toBe('')
  })

  it('通过 provider 和 account 身份识别聚合托管渠道', () => {
    expect(isManagedProviderChannel({ providerId: 'mimo', accountUid: 'acct-1' } as never)).toBe(true)
    expect(isManagedProviderChannel({ autoManaged: true } as never)).toBe(true)
    expect(isManagedProviderChannel({ providerId: 'mimo' } as never)).toBe(false)
  })
})
