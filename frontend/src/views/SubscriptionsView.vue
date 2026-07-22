<template>
  <div class="subscriptions-view">
    <!-- Header -->
    <div class="d-flex align-center mb-6">
      <v-icon size="28" class="mr-2" color="primary">mdi-lightning-bolt</v-icon>
      <span class="text-h5 font-weight-bold">{{ t('subscription.title') }}</span>
    </div>

    <!-- Provider 卡片网格 -->
    <SubscriptionProviderGrid @select="handleProviderSelect" />

    <!-- 右侧详情面板（根据选中 Provider 展示不同表单） -->
    <v-expand-transition>
      <div v-if="selectedProvider" class="mt-6">
        <!-- GitHub Copilot 详情 -->
        <v-card v-if="selectedProvider === 'github-copilot'" variant="outlined" class="pa-4">
          <v-card-title class="text-h6 d-flex align-center">
            <v-icon color="primary" class="mr-2">mdi-github</v-icon>
            GitHub Copilot
          </v-card-title>
          <v-card-text>
            <p class="text-body-2 text-medium-emphasis mb-4">
              {{ t('subscription.copilotDescription') }}
            </p>
            <v-alert color="info" variant="tonal" density="compact" class="mb-4">
              {{ t('subscription.copilotComingSoon') }}
            </v-alert>
          </v-card-text>
        </v-card>

        <!-- new-api 详情 -->
        <v-card v-if="selectedProvider === 'new-api'" variant="outlined" class="pa-4">
          <v-card-title class="text-h6 d-flex align-center">
            <v-icon color="warning" class="mr-2">mdi-server-network</v-icon>
            {{ t('subscription.newApi.connect') }}
          </v-card-title>
          <v-card-text>
            <v-form @submit.prevent="handleNewApiSubmit">
              <div class="d-flex flex-column ga-3">
                <v-text-field
                  v-model="newApiForm.baseUrl"
                  :label="t('subscription.newApi.baseUrl')"
                  placeholder="https://your-newapi-instance.com"
                  variant="outlined"
                  density="compact"
                  required
                />
                <v-text-field
                  v-model="newApiForm.accessToken"
                  :label="t('subscription.newApi.accessToken')"
                  variant="outlined"
                  density="compact"
                  type="password"
                  required
                />
                <v-expansion-panels variant="accordion">
                  <v-expansion-panel>
                    <v-expansion-panel-title>
                      {{ t('subscription.newApi.advancedOptions') }}
                    </v-expansion-panel-title>
                    <v-expansion-panel-text>
                      <v-text-field
                        v-model="newApiForm.userId"
                        :label="t('subscription.newApi.userId')"
                        variant="outlined"
                        density="compact"
                        class="mb-2"
                      />
                      <v-select
                        v-model="newApiForm.authTokenMode"
                        :label="t('subscription.newApi.authTokenMode')"
                        :items="authTokenModeOptions"
                        variant="outlined"
                        density="compact"
                        class="mb-2"
                      />
                      <v-text-field
                        v-model="newApiForm.displayName"
                        :label="t('subscription.field.name')"
                        variant="outlined"
                        density="compact"
                      />
                    </v-expansion-panel-text>
                  </v-expansion-panel>
                </v-expansion-panels>
              </div>

              <v-alert
                v-if="newApiVerifyResult"
                color="success"
                variant="tonal"
                density="compact"
                class="mt-3"
              >
                <div class="text-body-2">
                  <div>{{ t('subscription.newApi.username') }}: {{ newApiVerifyResult.username }}</div>
                  <div>{{ t('subscription.newApi.quota') }}: {{ newApiVerifyResult.quota }}</div>
                  <div v-if="newApiVerifyResult.availableModels?.length">
                    {{ t('subscription.newApi.availableModels') }}: {{ newApiVerifyResult.availableModels.length }}
                  </div>
                </div>
              </v-alert>

              <v-alert
                v-if="newApiError"
                color="error"
                variant="tonal"
                density="compact"
                class="mt-3"
              >
                {{ newApiError }}
              </v-alert>
            </v-form>
          </v-card-text>
          <v-card-actions>
            <v-spacer />
            <v-btn variant="text" @click="selectedProvider = ''">{{ t('app.actions.cancel') }}</v-btn>
            <v-btn
              v-if="!newApiVerifyResult"
              color="primary"
              :loading="newApiVerifying"
              :disabled="!canNewApiVerify"
              @click="handleNewApiVerify"
            >
              {{ t('subscription.newApi.verify') }}
            </v-btn>
            <v-btn
              v-else
              color="primary"
              :loading="newApiProvisioning"
              @click="handleNewApiProvision"
            >
              {{ t('subscription.newApi.provision') }}
            </v-btn>
          </v-card-actions>
        </v-card>

        <!-- 手动添加详情 -->
        <v-card v-if="selectedProvider === 'manual'" variant="outlined" class="pa-4">
          <v-card-title class="text-h6 d-flex align-center">
            <v-icon color="secondary" class="mr-2">mdi-plus-circle</v-icon>
            {{ t('subscription.manualAdd') }}
          </v-card-title>
          <v-card-text>
            <v-form @submit.prevent="handleManualSubmit">
              <v-text-field
                v-model="manualForm.subscriptionUid"
                :label="t('subscription.field.uid')"
                variant="outlined"
                density="compact"
                class="mb-2"
                required
              />
              <v-text-field
                v-model="manualForm.displayName"
                :label="t('subscription.field.name')"
                variant="outlined"
                density="compact"
                class="mb-2"
                required
              />
              <v-text-field
                v-model="manualForm.provider"
                :label="t('subscription.field.provider')"
                variant="outlined"
                density="compact"
                class="mb-2"
              />
              <v-select
                v-model="manualForm.originType"
                :label="t('subscription.field.originType')"
                :items="originTypeOptions"
                variant="outlined"
                density="compact"
                class="mb-2"
              />
              <v-select
                v-model="manualForm.billingMode"
                :label="t('subscription.field.billingMode')"
                :items="billingModeOptions"
                variant="outlined"
                density="compact"
                class="mb-2"
              />
              <v-text-field
                v-model="manualForm.currency"
                :label="t('subscription.field.currency')"
                variant="outlined"
                density="compact"
                class="mb-2"
                placeholder="CNY / USD"
              />
              <v-text-field
                v-model.number="manualForm.balance"
                :label="t('subscription.field.balance')"
                variant="outlined"
                density="compact"
                type="number"
                class="mb-2"
              />
              <v-textarea
                v-model="manualForm.notes"
                :label="t('subscription.field.notes')"
                variant="outlined"
                density="compact"
                rows="2"
              />
            </v-form>
          </v-card-text>
          <v-card-actions>
            <v-spacer />
            <v-btn variant="text" @click="selectedProvider = ''">{{ t('app.actions.cancel') }}</v-btn>
            <v-btn color="primary" :loading="manualSaving" @click="handleManualSubmit">
              {{ t('app.actions.save') }}
            </v-btn>
          </v-card-actions>
        </v-card>
      </div>
    </v-expand-transition>

    <!-- 提示信息 -->
    <v-alert color="info" variant="tonal" density="compact" class="mt-6">
      <v-icon start>mdi-information</v-icon>
      {{ t('subscription.manageInChannels') }}
    </v-alert>

    <!-- Snackbar -->
    <v-snackbar v-model="snackbar.show" :color="snackbar.color" :timeout="3000">
      {{ snackbar.message }}
    </v-snackbar>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from '@/i18n'
import { api } from '@/services/api'
import { useRuntimePresets } from '@/composables/useRuntimePresets'
import SubscriptionProviderGrid from '@/components/subscriptions/SubscriptionProviderGrid.vue'
import type { NewApiVerifyResponse, NewApiProvisionResponse } from '@/services/api-types'

const { t } = useI18n()
const { subscriptionPreset: preset, ensureLoaded: ensureRuntimePresetsLoaded } = useRuntimePresets()

const selectedProvider = ref('')
const snackbar = ref({ show: false, message: '', color: 'success' })

// new-api 表单
const newApiForm = ref({
  baseUrl: '',
  accessToken: '',
  userId: '',
  authTokenMode: 'bearer',
  displayName: '',
})
const newApiVerifying = ref(false)
const newApiProvisioning = ref(false)
const newApiVerifyResult = ref<NewApiVerifyResponse | null>(null)
const newApiError = ref('')

const authTokenModeOptions = computed(() => [
  { title: 'Bearer', value: 'bearer' },
  { title: 'Raw', value: 'raw' },
])

const canNewApiVerify = computed(() => {
  return !!newApiForm.value.baseUrl.trim() && !!newApiForm.value.accessToken.trim()
})

// 手动添加表单
const manualForm = ref({
  subscriptionUid: '',
  displayName: '',
  provider: '',
  originType: '',
  billingMode: '',
  currency: '',
  balance: 0,
  notes: '',
})
const manualSaving = ref(false)

const originTypeOptions = computed(() =>
  preset.value.originTypes.map((o) => ({
    title: o.value,
    value: o.value,
  })),
)

const billingModeOptions = computed(() =>
  preset.value.billingModes.map((m) => ({
    title: m,
    value: m,
  })),
)

function handleProviderSelect(provider: string) {
  selectedProvider.value = provider
  // 重置表单
  newApiForm.value = { baseUrl: '', accessToken: '', userId: '', authTokenMode: 'bearer', displayName: '' }
  newApiVerifyResult.value = null
  newApiError.value = ''
  manualForm.value = { subscriptionUid: '', displayName: '', provider: '', originType: '', billingMode: '', currency: '', balance: 0, notes: '' }
}

async function handleNewApiVerify() {
  if (!canNewApiVerify.value) return
  newApiVerifying.value = true
  newApiError.value = ''
  try {
    const result = await api.verifyNewApiSubscription({
      baseUrl: newApiForm.value.baseUrl.trim(),
      accessToken: newApiForm.value.accessToken,
      userId: newApiForm.value.userId || undefined,
      authTokenMode: newApiForm.value.authTokenMode || undefined,
      displayName: newApiForm.value.displayName || undefined,
    })
    newApiVerifyResult.value = result
  } catch (e) {
    newApiError.value = e instanceof Error ? e.message : 'Unknown error'
  } finally {
    newApiVerifying.value = false
  }
}

async function handleNewApiProvision() {
  if (!newApiVerifyResult.value) return
  newApiProvisioning.value = true
  newApiError.value = ''
  try {
    const displayName = newApiForm.value.displayName || newApiVerifyResult.value.username || 'new-api'
    const result = await api.provisionNewApiSubscription({
      subscriptionUid: `newapi-${Date.now()}`,
      displayName,
      baseUrl: newApiForm.value.baseUrl.trim(),
      accessToken: newApiForm.value.accessToken,
      userId: newApiForm.value.userId || undefined,
      authTokenMode: newApiForm.value.authTokenMode || undefined,
      channelKind: 'messages',
      provisionAllEligibleGroups: true,
      maxGroupMultiplier: 1.0,
    })
    showSnackbar(t('subscription.newApi.provisionSuccess'), 'success')
    selectedProvider.value = ''
  } catch (e) {
    newApiError.value = e instanceof Error ? e.message : 'Unknown error'
    showSnackbar(newApiError.value, 'error')
  } finally {
    newApiProvisioning.value = false
  }
}

async function handleManualSubmit() {
  if (!manualForm.value.subscriptionUid.trim() || !manualForm.value.displayName.trim()) return
  manualSaving.value = true
  try {
    await api.createSubscription({
      subscriptionUid: manualForm.value.subscriptionUid.trim(),
      displayName: manualForm.value.displayName.trim(),
      provider: manualForm.value.provider || undefined,
      originType: manualForm.value.originType || undefined,
      billingMode: manualForm.value.billingMode || undefined,
      currency: manualForm.value.currency || undefined,
      balance: manualForm.value.balance,
      notes: manualForm.value.notes || undefined,
      source: 'manual',
      rechargeMultiplier: 1,
    })
    showSnackbar(t('subscription.add') + ' - OK', 'success')
    selectedProvider.value = ''
  } catch (e) {
    showSnackbar(e instanceof Error ? e.message : 'Unknown error', 'error')
  } finally {
    manualSaving.value = false
  }
}

function showSnackbar(message: string, color: string) {
  snackbar.value = { show: true, message, color }
}

onMounted(() => {
  ensureRuntimePresetsLoaded()
})
</script>

<style scoped>
.subscriptions-view {
  padding: 16px;
}
</style>
