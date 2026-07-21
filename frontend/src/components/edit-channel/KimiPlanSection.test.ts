// eslint-disable-next-line @typescript-eslint/ban-ts-comment
// @ts-nocheck
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

describe('Kimi 套餐面板接线', () => {
  it('主 Web 编辑页向 Kimi 套餐面板传递托管账号身份', () => {
    const editModal = readFileSync(resolve(__dirname, '../EditChannelModal.vue'), 'utf8')
    const apiKeySection = readFileSync(resolve(__dirname, 'ApiKeyManagementSection.vue'), 'utf8')

    expect(editModal).toContain(':account-uid="props.channel?.accountUid"')
    expect(editModal).toContain(':provider-id="props.channel?.providerId"')
    expect(apiKeySection).toContain("v-if=\"providerId === 'kimi' && accountUid\"")
    expect(apiKeySection).toContain(':account-uid="accountUid"')
    expect(apiKeySection).toContain("import KimiPlanSection from './KimiPlanSection.vue'")
  })
})
