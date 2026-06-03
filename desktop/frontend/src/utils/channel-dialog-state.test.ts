import { describe, expect, it } from 'vitest'

import {
  filterValidSupportedModelPatterns,
  isValidSupportedModelPattern,
  parseSupportedModelInput
} from './channel-dialog-state'

describe('isValidSupportedModelPattern', () => {
  it('支持精确、前缀、后缀、包含和排除规则', () => {
    expect(isValidSupportedModelPattern('gpt-4o')).toBe(true)
    expect(isValidSupportedModelPattern('gpt-4*')).toBe(true)
    expect(isValidSupportedModelPattern('*image')).toBe(true)
    expect(isValidSupportedModelPattern('*image*')).toBe(true)
    expect(isValidSupportedModelPattern('!*image*')).toBe(true)
  })

  it('拒绝非法中间通配和空规则', () => {
    expect(isValidSupportedModelPattern('foo*bar')).toBe(false)
    expect(isValidSupportedModelPattern('**')).toBe(false)
    expect(isValidSupportedModelPattern('')).toBe(false)
    expect(isValidSupportedModelPattern('   ')).toBe(false)
    expect(isValidSupportedModelPattern('!')).toBe(false)
    expect(isValidSupportedModelPattern('!!gpt-4*')).toBe(false)
  })

  it('拒绝包含中文顿号、逗号等非法字符的规则', () => {
    expect(isValidSupportedModelPattern('gpt-5*、ada*')).toBe(false)
    expect(isValidSupportedModelPattern('gpt-5*,ada*')).toBe(false)
    expect(isValidSupportedModelPattern('gpt 5')).toBe(false)
    expect(isValidSupportedModelPattern('模型')).toBe(false)
  })
})

describe('parseSupportedModelInput', () => {
  it('按中文顿号拆分多条规则', () => {
    expect(parseSupportedModelInput('GPT-5*、ada*')).toEqual(['GPT-5*', 'ada*'])
  })

  it('兼容逗号、分号、竖线和多余空白', () => {
    expect(parseSupportedModelInput('a, b ; c | d')).toEqual(['a', 'b', 'c', 'd'])
    expect(parseSupportedModelInput('  gpt-4*  ，  *image*  ')).toEqual(['gpt-4*', '*image*'])
  })

  it('过滤纯空白与空项', () => {
    expect(parseSupportedModelInput('、、 ,, ；')).toEqual([])
    expect(parseSupportedModelInput('')).toEqual([])
  })
})

describe('filterValidSupportedModelPatterns', () => {
  it('过滤非法规则并保留合法规则顺序', () => {
    expect(filterValidSupportedModelPatterns([' gpt-4* ', 'foo*bar', '!*image*'])).toEqual({
      validPatterns: ['gpt-4*', '!*image*'],
      hasInvalidPatterns: true
    })
  })

  it('全部合法时不标记错误', () => {
    expect(filterValidSupportedModelPatterns(['gpt-4*', '*image*'])).toEqual({
      validPatterns: ['gpt-4*', '*image*'],
      hasInvalidPatterns: false
    })
  })

  it('自动拆分含顿号的粘贴输入为多条合法规则', () => {
    expect(filterValidSupportedModelPatterns(['GPT-5*、ada*'])).toEqual({
      validPatterns: ['GPT-5*', 'ada*'],
      hasInvalidPatterns: false
    })
  })
})
