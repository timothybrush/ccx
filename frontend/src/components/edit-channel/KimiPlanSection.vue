<template>
  <div class="kimi-plan-section mb-5">
    <v-divider class="mb-4" />
    <div class="d-flex align-center justify-space-between ga-3 flex-wrap mb-2">
      <div class="d-flex align-center ga-2">
        <v-icon color="primary" size="small">mdi-chart-donut</v-icon>
        <span class="text-body-2 font-weight-medium">{{ t('kimiConsoleToken.title') }}</span>
      </div>
      <v-btn
        href="https://www.kimi.com/code/console"
        target="_blank"
        rel="noopener noreferrer"
        size="small"
        variant="text"
        color="primary"
      >
        <v-icon start size="small">mdi-open-in-new</v-icon>
        {{ t('kimiConsoleToken.openConsole') }}
      </v-btn>
    </div>
    <div class="text-caption text-medium-emphasis mb-3">{{ t('kimiConsoleToken.hint') }}</div>

    <v-progress-linear v-if="loading" indeterminate color="primary" class="mb-3" />
    <v-alert v-if="loadError" color="error" variant="tonal" density="compact" class="mb-3">
      {{ loadError }}
    </v-alert>

    <div v-for="credential in credentials" :key="credential.credentialUid" class="kimi-credential py-3">
      <div class="d-flex align-center justify-space-between ga-3 flex-wrap mb-3">
        <code class="text-caption">{{ credential.keyMask }}</code>
        <v-chip :color="credential.hasKimiConsoleToken ? 'info' : 'warning'" size="x-small" variant="tonal">
          {{ credential.hasKimiConsoleToken ? t('kimiConsoleToken.configured') : t('kimiConsoleToken.notConfigured') }}
        </v-chip>
      </div>

      <template v-if="credential.kimiCodeUsage">
        <div class="kimi-quota-grid mb-3">
          <div v-for="item in quotaItems(credential.kimiCodeUsage)" :key="item.label" class="kimi-quota-item">
            <div class="text-caption text-medium-emphasis">{{ item.label }}</div>
            <div class="text-body-2 font-weight-medium">{{ formatQuota(item.window) }}</div>
            <v-progress-linear
              :model-value="quotaUsedPercent(item.window)"
              :color="usageColor(quotaUsedPercent(item.window))"
              height="4"
              rounded
              class="my-2"
            />
            <div class="text-caption text-disabled">
              {{ t('kimiConsoleToken.resetAt') }} {{ formatDateTime(item.window.resetTime) }}
            </div>
          </div>
        </div>

        <div class="kimi-balance-grid mb-2">
          <div v-if="credential.kimiCodeUsage.subscriptionBalance">
            <div class="text-caption text-medium-emphasis">{{ t('kimiConsoleToken.subscriptionRemaining') }}</div>
            <div class="text-body-2 font-weight-medium">
              {{ formatRemainingRatio(credential.kimiCodeUsage.subscriptionBalance.amountUsedRatio) }}
            </div>
            <div class="text-caption text-disabled">
              {{ t('kimiConsoleToken.codeUsed') }}
              {{ formatUsedRatio(credential.kimiCodeUsage.subscriptionBalance.kimiCodeUsedRatio) }}
            </div>
          </div>
          <div v-if="credential.kimiCodeUsage.codeSevenDay?.enabled">
            <div class="text-caption text-medium-emphasis">{{ t('kimiConsoleToken.sevenDayRemaining') }}</div>
            <div class="text-body-2 font-weight-medium">
              {{ formatRemainingRatio(credential.kimiCodeUsage.codeSevenDay.ratio) }}
            </div>
            <div class="text-caption text-disabled">
              {{ t('kimiConsoleToken.resetAt') }} {{ formatDateTime(credential.kimiCodeUsage.codeSevenDay.resetTime) }}
            </div>
          </div>
          <div v-if="credential.kimiCodeUsage.subscriptionBalance?.expireTime">
            <div class="text-caption text-medium-emphasis">{{ t('kimiConsoleToken.expiresAt') }}</div>
            <div class="text-body-2 font-weight-medium">
              {{ formatDateTime(credential.kimiCodeUsage.subscriptionBalance.expireTime) }}
            </div>
          </div>
          <div
            v-for="(gift, index) in credential.kimiCodeUsage.giftBalances ?? []"
            :key="`${gift.type}-${gift.expireTime}-${index}`"
          >
            <div class="text-caption text-medium-emphasis">{{ t('kimiConsoleToken.giftBalance') }}</div>
            <div class="text-body-2 font-weight-medium">
              {{ formatRemainingRatio(gift.amountUsedRatio) }}
            </div>
            <div class="text-caption text-disabled">
              {{ t('kimiConsoleToken.codeUsed') }} {{ formatUsedRatio(gift.kimiCodeUsedRatio) }}
            </div>
            <div class="text-caption text-disabled">
              {{ t('kimiConsoleToken.expiresAt') }} {{ formatDateTime(gift.expireTime) }}
            </div>
          </div>
        </div>
        <div class="text-caption text-disabled mb-3">
          {{ t('kimiConsoleToken.validatedAt') }} {{ formatDateTime(credential.kimiCodeUsage.validatedAt) }}
        </div>
      </template>

      <div v-if="forms[credential.credentialUid]" class="d-flex flex-column ga-2">
        <v-text-field
          v-model="forms[credential.credentialUid].accessToken"
          :label="t('kimiConsoleToken.token')"
          :placeholder="t('kimiConsoleToken.tokenPlaceholder')"
          type="password"
          variant="outlined"
          density="compact"
          autocomplete="new-password"
          hide-details
        />
        <v-alert v-if="forms[credential.credentialUid].error" color="error" variant="tonal" density="compact">
          {{ forms[credential.credentialUid].error }}
        </v-alert>
        <div class="d-flex align-center justify-end ga-2 flex-wrap">
          <v-tooltip
            v-if="credential.hasKimiConsoleToken"
            :text="t('kimiConsoleToken.refresh')"
            location="top"
            :open-delay="150"
            content-class="key-tooltip"
          >
            <template #activator="{ props: tooltipProps }">
              <v-btn
                v-bind="tooltipProps"
                icon
                size="small"
                variant="text"
                color="secondary"
                :loading="forms[credential.credentialUid].refreshing"
                :aria-label="t('kimiConsoleToken.refresh')"
                @click="refreshToken(credential)"
              >
                <v-icon size="small">mdi-refresh</v-icon>
              </v-btn>
            </template>
          </v-tooltip>
          <v-tooltip
            v-if="credential.hasKimiConsoleToken"
            :text="t('kimiConsoleToken.clear')"
            location="top"
            :open-delay="150"
            content-class="key-tooltip"
          >
            <template #activator="{ props: tooltipProps }">
              <v-btn
                v-bind="tooltipProps"
                icon
                size="small"
                variant="text"
                color="error"
                :loading="forms[credential.credentialUid].clearing"
                :aria-label="t('kimiConsoleToken.clear')"
                @click="clearToken(credential)"
              >
                <v-icon size="small">mdi-link-off</v-icon>
              </v-btn>
            </template>
          </v-tooltip>
          <v-btn
            size="small"
            variant="tonal"
            color="primary"
            :loading="forms[credential.credentialUid].saving"
            :disabled="!forms[credential.credentialUid].accessToken.trim()"
            @click="saveToken(credential)"
          >
            <v-icon start size="small">mdi-check-decagram-outline</v-icon>
            {{ t('kimiConsoleToken.verifyAndSave') }}
          </v-btn>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from '../../i18n'
import { ApiService } from '../../services/api'
import type { KimiCodeQuotaWindow, KimiCodeUsageSnapshot, ManagedAccountCredential } from '../../services/api-types'

interface Props {
  accountUid: string
}

interface CredentialForm {
  accessToken: string
  saving: boolean
  refreshing: boolean
  clearing: boolean
  error: string
}

const props = defineProps<Props>()
const { t } = useI18n()
const apiService = new ApiService()
const credentials = ref<ManagedAccountCredential[]>([])
const forms = ref<Record<string, CredentialForm>>({})
const loading = ref(false)
const loadError = ref('')
let loadRequest = 0

const loadCredentials = async () => {
  const request = ++loadRequest
  credentials.value = []
  loadError.value = ''
  if (!props.accountUid) return
  loading.value = true
  try {
    const response = await apiService.getManagedAccounts()
    if (request !== loadRequest) return
    const account = response.accounts.find(item => item.accountUid === props.accountUid)
    if (!account) {
      loadError.value = t('kimiConsoleToken.accountNotFound')
      return
    }
    credentials.value = account.credentials
    const nextForms: Record<string, CredentialForm> = {}
    for (const credential of account.credentials) {
      nextForms[credential.credentialUid] = forms.value[credential.credentialUid] ?? {
        accessToken: '',
        saving: false,
        refreshing: false,
        clearing: false,
        error: ''
      }
    }
    forms.value = nextForms
  } catch (error) {
    if (request === loadRequest) loadError.value = errorMessage(error)
  } finally {
    if (request === loadRequest) loading.value = false
  }
}

watch(
  () => props.accountUid,
  () => {
    void loadCredentials()
  },
  { immediate: true }
)

const saveToken = async (credential: ManagedAccountCredential) => {
  const form = forms.value[credential.credentialUid]
  if (!form?.accessToken.trim()) return
  form.saving = true
  form.error = ''
  try {
    const response = await apiService.setKimiConsoleToken(
      props.accountUid,
      credential.credentialUid,
      form.accessToken.trim()
    )
    credential.hasKimiConsoleToken = true
    credential.kimiCodeUsage = response.usage
    form.accessToken = ''
  } catch (error) {
    form.error = errorMessage(error)
  } finally {
    form.saving = false
  }
}

const refreshToken = async (credential: ManagedAccountCredential) => {
  const form = forms.value[credential.credentialUid]
  if (!form) return
  form.refreshing = true
  form.error = ''
  try {
    const response = await apiService.refreshKimiConsoleToken(props.accountUid, credential.credentialUid)
    credential.kimiCodeUsage = response.usage
  } catch (error) {
    form.error = errorMessage(error)
  } finally {
    form.refreshing = false
  }
}

const clearToken = async (credential: ManagedAccountCredential) => {
  if (!window.confirm(t('kimiConsoleToken.clearConfirm'))) return
  const form = forms.value[credential.credentialUid]
  if (!form) return
  form.clearing = true
  form.error = ''
  try {
    await apiService.clearKimiConsoleToken(props.accountUid, credential.credentialUid)
    credential.hasKimiConsoleToken = false
    credential.kimiCodeUsage = undefined
  } catch (error) {
    form.error = errorMessage(error)
  } finally {
    form.clearing = false
  }
}

const quotaItems = (usage: KimiCodeUsageSnapshot) => [
  { label: t('kimiConsoleToken.weeklyRemaining'), window: usage.weeklyUsage },
  ...(usage.rateLimits ?? []).map(limit => ({
    label: t('kimiConsoleToken.rateLimitRemaining', { window: formatDuration(limit.windowSeconds) }),
    window: limit.usage
  }))
]

const numberFormat = new Intl.NumberFormat()
const percentFormat = new Intl.NumberFormat(undefined, { maximumFractionDigits: 2 })
const dateTimeFormat = new Intl.DateTimeFormat(undefined, {
  year: 'numeric',
  month: '2-digit',
  day: '2-digit',
  hour: '2-digit',
  minute: '2-digit'
})

const formatQuota = (window: KimiCodeQuotaWindow) =>
  `${numberFormat.format(Math.max(0, window.remaining))} / ${numberFormat.format(Math.max(0, window.limit))}`

const quotaUsedPercent = (window: KimiCodeQuotaWindow) => {
  if (window.limit <= 0) return 0
  return Math.max(0, Math.min(100, (window.used / window.limit) * 100))
}

const usageColor = (percent: number) => {
  if (percent >= 90) return 'error'
  if (percent >= 70) return 'warning'
  return 'success'
}

const formatRemainingRatio = (usedRatio: number) => `${percentFormat.format(Math.max(0, (1 - usedRatio) * 100))}%`
const formatUsedRatio = (usedRatio: number) => `${percentFormat.format(Math.max(0, usedRatio * 100))}%`

const formatDateTime = (value?: string) => {
  if (!value) return '-'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '-' : dateTimeFormat.format(date)
}

const formatDuration = (seconds: number) => {
  if (seconds > 0 && seconds % 3600 === 0) {
    return t('kimiConsoleToken.durationHours', { value: seconds / 3600 })
  }
  if (seconds > 0 && seconds % 60 === 0) {
    return t('kimiConsoleToken.durationMinutes', { value: seconds / 60 })
  }
  return t('kimiConsoleToken.durationSeconds', { value: Math.max(0, seconds) })
}

const errorMessage = (error: unknown) => (error instanceof Error ? error.message : String(error))
</script>

<style scoped>
.kimi-credential + .kimi-credential {
  border-top: 1px solid rgba(var(--v-border-color), var(--v-border-opacity));
}

.kimi-quota-grid,
.kimi-balance-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  gap: 12px;
}

.kimi-quota-item {
  min-width: 0;
}

@media (max-width: 700px) {
  .kimi-quota-grid,
  .kimi-balance-grid {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
