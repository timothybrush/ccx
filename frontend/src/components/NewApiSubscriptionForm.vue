<template>
  <div class="newapi-subscription-form d-flex flex-column ga-4">
    <!-- Step 1: 验证 -->
    <v-form @submit.prevent="handleVerify">
      <div class="text-subtitle-2 mb-2 text-medium-emphasis">
        {{ t('subscription.newApi.step1Title') }}
      </div>
      <v-text-field
        v-model="verifyForm.baseUrl"
        :label="t('subscription.newApi.baseUrl')"
        placeholder="https://your-newapi-instance.com"
        variant="outlined"
        density="compact"
        class="mb-2"
        :disabled="verified"
        required
      />
      <v-text-field
        v-model="verifyForm.accessToken"
        :label="t('subscription.newApi.accessToken')"
        variant="outlined"
        density="compact"
        type="password"
        class="mb-2"
        :disabled="verified"
        required
      />
      <v-text-field
        v-model="verifyForm.userId"
        :label="t('subscription.newApi.userId')"
        variant="outlined"
        density="compact"
        class="mb-2"
        :disabled="verified"
      />
      <v-select
        v-model="verifyForm.authTokenMode"
        :label="t('subscription.newApi.authTokenMode')"
        :items="authTokenModeOptions"
        variant="outlined"
        density="compact"
        class="mb-2"
        :disabled="verified"
      />
      <v-text-field
        v-model="verifyForm.displayName"
        :label="t('subscription.field.name')"
        variant="outlined"
        density="compact"
        class="mb-2"
        :disabled="verified"
      />

      <v-btn
        v-if="!verified"
        color="primary"
        type="submit"
        :loading="verifying"
        :disabled="!canVerify"
        block
      >
        {{ t('subscription.newApi.verify') }}
      </v-btn>
      <v-btn
        v-else
        variant="tonal"
        block
        @click="resetVerification"
      >
        {{ t('subscription.newApi.reVerify') }}
      </v-btn>
    </v-form>

    <!-- 验证结果展示 -->
    <v-card v-if="verified && verifyResult" variant="outlined" class="pa-3">
      <div class="text-subtitle-2 mb-2">{{ t('subscription.newApi.accountPreview') }}</div>
      <div class="d-flex flex-column ga-1 text-body-2">
        <div>{{ t('subscription.newApi.username') }}: {{ verifyResult.username }}</div>
        <div>{{ t('subscription.newApi.quota') }}: {{ verifyResult.quota }}</div>
        <div>{{ t('subscription.newApi.usedQuota') }}: {{ verifyResult.usedQuota }}</div>
        <div>
          {{ t('subscription.newApi.availableModels') }}: {{ verifyResult.availableModels.length }}
        </div>
        <div v-if="groupItems.length">
          {{ t('subscription.newApi.groups') }}:
          <v-chip
            v-for="g in groupItems"
            :key="g.name"
            size="small"
            class="mr-1 mt-1"
            variant="tonal"
          >
            {{ g.name }} × {{ g.ratio }}
          </v-chip>
        </div>
      </div>
    </v-card>

    <!-- Step 2: 接入 -->
    <v-form v-if="verified" @submit.prevent="handleProvision">
      <v-divider class="my-2" />
      <div class="text-subtitle-2 mb-2 text-medium-emphasis">
        {{ t('subscription.newApi.step2Title') }}
      </div>
      <v-text-field
        v-model="provisionForm.subscriptionUid"
        :label="t('subscription.field.uid')"
        variant="outlined"
        density="compact"
        class="mb-2"
        required
      />
      <v-select
        v-model="provisionForm.channelKind"
        :label="t('subscription.newApi.channelKind')"
        :items="channelKindOptions"
        variant="outlined"
        density="compact"
        class="mb-2"
        required
      />
      <v-text-field
        v-model="provisionForm.channelName"
        :label="t('subscription.newApi.channelName')"
        variant="outlined"
        density="compact"
        class="mb-2"
      />
      <v-select
        v-if="groupItems.length"
        v-model="provisionForm.provisionGroup"
        :label="t('subscription.newApi.provisionGroup')"
        :items="groupSelectOptions"
        variant="outlined"
        density="compact"
        clearable
        class="mb-2"
      />
      <v-textarea
        v-model="provisionForm.notes"
        :label="t('subscription.field.notes')"
        variant="outlined"
        density="compact"
        rows="2"
        class="mb-2"
      />

      <v-btn
        color="primary"
        type="submit"
        :loading="provisioning"
        :disabled="!canProvision"
        block
      >
        {{ t('subscription.newApi.provision') }}
      </v-btn>
    </v-form>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useI18n } from '@/i18n'
import { api } from '@/services/api'
import type {
  NewApiVerifyRequest,
  NewApiVerifyResponse,
  NewApiProvisionRequest,
  NewApiProvisionResponse,
} from '@/services/api-types'

const { t } = useI18n()

const emit = defineEmits<{
  created: [result: NewApiProvisionResponse]
  error: [message: string]
}>()

const verifying = ref(false)
const provisioning = ref(false)
const verified = ref(false)
const verifyResult = ref<NewApiVerifyResponse | null>(null)

const verifyForm = ref<NewApiVerifyRequest>({
  baseUrl: '',
  accessToken: '',
  userId: '',
  authTokenMode: 'bearer',
  displayName: '',
})

const provisionForm = ref<NewApiProvisionRequest>({
  subscriptionUid: '',
  displayName: '',
  baseUrl: '',
  accessToken: '',
  channelKind: 'messages',
  userId: '',
  authTokenMode: 'bearer',
  channelName: '',
  provisionGroup: '',
  notes: '',
})

const authTokenModeOptions = computed(() => [
  { title: 'Bearer', value: 'bearer' },
  { title: 'Raw', value: 'raw' },
])

const channelKindOptions = computed(() => [
  { title: 'messages', value: 'messages' },
  { title: 'chat', value: 'chat' },
  { title: 'responses', value: 'responses' },
  { title: 'gemini', value: 'gemini' },
])

const groupItems = computed(() => {
  if (!verifyResult.value) return []
  return Object.entries(verifyResult.value.groups || {}).map(([name, ratio]) => ({ name, ratio }))
})

const groupSelectOptions = computed(() =>
  groupItems.value.map((g) => ({ title: `${g.name} × ${g.ratio}`, value: g.name }))
)

const canVerify = computed(() => !!verifyForm.value.baseUrl.trim() && !!verifyForm.value.accessToken.trim())
const canProvision = computed(
  () => !!provisionForm.value.subscriptionUid.trim() && !!provisionForm.value.channelKind
)

function slugifyDisplayName(name: string): string {
  const base = name
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9一-龥]+/g, '-')
    .replace(/^-+|-+$/g, '')
  return base || 'newapi'
}

watch(
  () => verifyForm.value.displayName,
  (name) => {
    if (!verified.value && name) {
      provisionForm.value.subscriptionUid = `newapi-${slugifyDisplayName(name)}`
    }
  }
)

async function handleVerify() {
  if (!canVerify.value) return
  verifying.value = true
  try {
    const result = await api.verifyNewApiSubscription({
      baseUrl: verifyForm.value.baseUrl.trim(),
      accessToken: verifyForm.value.accessToken,
      userId: verifyForm.value.userId || undefined,
      authTokenMode: verifyForm.value.authTokenMode || undefined,
      displayName: verifyForm.value.displayName || undefined,
    })
    verifyResult.value = result
    verified.value = true

    // 预填第 2 步表单
    provisionForm.value.baseUrl = verifyForm.value.baseUrl.trim()
    provisionForm.value.accessToken = verifyForm.value.accessToken
    provisionForm.value.userId = verifyForm.value.userId || undefined
    provisionForm.value.authTokenMode = verifyForm.value.authTokenMode || undefined
    provisionForm.value.displayName = verifyForm.value.displayName || result.username
    if (!provisionForm.value.subscriptionUid.trim()) {
      provisionForm.value.subscriptionUid = `newapi-${slugifyDisplayName(
        verifyForm.value.displayName || result.username
      )}`
    }
  } catch (e) {
    const message = e instanceof Error ? e.message : 'Unknown error'
    emit('error', message)
  } finally {
    verifying.value = false
  }
}

function resetVerification() {
  verified.value = false
  verifyResult.value = null
}

async function handleProvision() {
  if (!canProvision.value) return
  provisioning.value = true
  try {
    const result = await api.provisionNewApiSubscription({
      subscriptionUid: provisionForm.value.subscriptionUid.trim(),
      displayName: provisionForm.value.displayName || provisionForm.value.subscriptionUid,
      baseUrl: provisionForm.value.baseUrl,
      accessToken: provisionForm.value.accessToken,
      channelKind: provisionForm.value.channelKind,
      userId: provisionForm.value.userId || undefined,
      authTokenMode: provisionForm.value.authTokenMode || undefined,
      channelName: provisionForm.value.channelName || undefined,
      provisionGroup: provisionForm.value.provisionGroup || undefined,
      notes: provisionForm.value.notes || undefined,
    })
    emit('created', result)
  } catch (e) {
    const message = e instanceof Error ? e.message : 'Unknown error'
    emit('error', message)
  } finally {
    provisioning.value = false
  }
}
</script>
