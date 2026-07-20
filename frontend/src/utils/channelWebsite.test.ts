import { describe, expect, it } from 'vitest'

import {
  getChannelWebsiteLinks,
  getVolcenginePlanWebsiteLinks,
  VOLCENGINE_AGENT_PLAN_CONSOLE_URL,
  VOLCENGINE_CODING_PLAN_CONSOLE_URL,
} from './channelWebsite'

describe('channelWebsite', () => {
  it('根据火山套餐端点提供对应控制台', () => {
    const links = getVolcenginePlanWebsiteLinks({
      providerId: 'volcengine',
      baseUrl: 'https://ark.cn-beijing.volces.com/api/plan',
      baseUrls: [
        'https://ark.cn-beijing.volces.com/api/plan',
        'https://ark.cn-beijing.volces.com/api/coding',
      ],
      apiKeyConfigs: [],
    })

    expect(links).toEqual([
      { kind: 'agent_plan', url: VOLCENGINE_AGENT_PLAN_CONSOLE_URL },
      { kind: 'coding_plan', url: VOLCENGINE_CODING_PLAN_CONSOLE_URL },
    ])
  })

  it('识别 OpenAI 兼容入口的 v3 后缀', () => {
    expect(getVolcenginePlanWebsiteLinks({
      providerId: 'volcengine',
      baseUrl: 'https://ark.cn-beijing.volces.com/api/coding/v3',
      apiKeyConfigs: [],
    })).toEqual([{ kind: 'coding_plan', url: VOLCENGINE_CODING_PLAN_CONSOLE_URL }])
  })

  it('根据不同 Key 绑定的端点提供全部套餐入口', () => {
    expect(getVolcenginePlanWebsiteLinks({
      providerId: 'volcengine',
      baseUrl: 'https://ark.cn-beijing.volces.com/api/plan',
      apiKeyConfigs: [
        { key: 'ark-agent', baseUrl: 'https://ark.cn-beijing.volces.com/api/plan' },
        { key: 'ark-coding', baseUrl: 'https://ark.cn-beijing.volces.com/api/coding/v3' },
      ],
    })).toEqual([
      { kind: 'agent_plan', url: VOLCENGINE_AGENT_PLAN_CONSOLE_URL },
      { kind: 'coding_plan', url: VOLCENGINE_CODING_PLAN_CONSOLE_URL },
    ])
  })

  it('用户自定义官网优先于自动推导', () => {
    expect(getChannelWebsiteLinks({
      providerId: 'volcengine',
      website: 'https://example.com/account',
      baseUrl: 'https://ark.cn-beijing.volces.com/api/plan',
      apiKeyConfigs: [],
    })).toEqual([{ kind: 'custom', url: 'https://example.com/account' }])
  })

  it('普通渠道继续回退到上游域名', () => {
    expect(getChannelWebsiteLinks({
      providerId: 'other',
      baseUrl: 'https://api.example.com/v1',
      apiKeyConfigs: [],
    })).toEqual([{ kind: 'upstream', url: 'https://api.example.com' }])
  })
})
