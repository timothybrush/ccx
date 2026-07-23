<template>
  <div class="subscription-provider-grid">
    <div class="text-h6 font-weight-bold mb-4">{{ t('subscription.quickAccess') }}</div>
    <div class="d-flex flex-wrap ga-4">
      <v-card
        class="provider-card pa-4 cursor-pointer"
        variant="outlined"
        :class="{ 'provider-card--active': selectedProvider === 'github-copilot' }"
        @click="selectProvider('github-copilot')"
      >
        <div class="d-flex align-center ga-3">
          <v-icon size="32" color="primary">mdi-github</v-icon>
          <div>
            <div class="text-subtitle-1 font-weight-bold">GitHub Copilot</div>
            <div class="text-caption text-medium-emphasis">{{ t('subscription.copilotDescription') }}</div>
          </div>
        </div>
      </v-card>

      <v-card
        class="provider-card pa-4 cursor-pointer"
        variant="outlined"
        :class="{ 'provider-card--active': selectedProvider === 'new-api' }"
        @click="selectProvider('new-api')"
      >
        <div class="d-flex align-center ga-3">
          <v-icon size="32" color="warning">mdi-server-network</v-icon>
          <div>
            <div class="text-subtitle-1 font-weight-bold">new-api</div>
            <div class="text-caption text-medium-emphasis">{{ t('subscription.newApiDescription') }}</div>
          </div>
        </div>
      </v-card>
    </div>

    <!-- 内置服务商 + 赞助商（与「快速添加」同源清单，卡片网格展示） -->
    <div class="text-h6 font-weight-bold mt-8 mb-4">{{ t('subscription.builtinProviders') }}</div>
    <div v-if="builtinLoading" class="d-flex align-center ga-2 text-medium-emphasis">
      <v-progress-circular indeterminate size="20" width="2" />
      <span class="text-body-2">{{ t('subscription.loadingProviders') }}</span>
    </div>
    <div v-else class="d-flex flex-wrap ga-4">
      <v-card
        v-for="provider in builtinProviders"
        :key="provider.providerId"
        class="provider-card pa-4 d-flex flex-column"
        variant="outlined"
      >
        <div class="d-flex align-center ga-3 mb-2">
          <v-icon size="32" color="secondary">mdi-domain</v-icon>
          <div class="text-subtitle-1 font-weight-bold">{{ provider.displayName }}</div>
        </div>
        <div class="text-caption text-medium-emphasis mb-3 provider-card__desc">
          {{ provider.description }}
        </div>
        <div v-if="capabilityKinds(provider).length" class="d-flex flex-wrap ga-1 mb-3">
          <v-chip
            v-for="kind in capabilityKinds(provider)"
            :key="kind"
            size="x-small"
            variant="tonal"
            color="primary"
          >
            {{ kindLabel(kind) }}
          </v-chip>
        </div>
        <v-spacer />
        <div class="d-flex align-center ga-2 mt-auto">
          <v-btn
            size="small"
            color="primary"
            variant="flat"
            @click="emit('add', provider.providerId)"
          >
            {{ t('subscription.addProvider') }}
          </v-btn>
          <v-btn
            v-if="providerConsoleLinks[provider.providerId]"
            size="small"
            variant="text"
            append-icon="mdi-open-in-new"
            @click="openProviderConsole(provider.providerId)"
          >
            {{ t('subscription.visitConsole') }}
          </v-btn>
          <v-btn
            v-if="providerPromotionLinks[provider.providerId]"
            size="small"
            variant="text"
            color="secondary"
            append-icon="mdi-open-in-new"
            @click="openProviderPromotion(provider.providerId)"
          >
            {{ t('subscription.visitSite') }}
          </v-btn>
        </div>
      </v-card>

      <!-- 赞助商渠道（仅展示 + 推广外链，无内置模板） -->
      <v-card
        v-for="sponsor in sponsors"
        :key="sponsor.providerId"
        class="provider-card pa-4 d-flex flex-column provider-card--sponsor"
        variant="outlined"
      >
        <div class="d-flex align-center ga-3 mb-2">
          <img :src="sponsor.logo" :alt="sponsor.displayName" class="sponsor-logo flex-shrink-0" />
          <div class="text-subtitle-1 font-weight-bold">{{ sponsor.displayName }}</div>
          <v-chip size="x-small" color="deep-purple" variant="tonal" class="ml-auto">
            {{ t('subscription.sponsorBadge') }}
          </v-chip>
        </div>
        <div class="text-caption text-medium-emphasis mb-3 provider-card__desc">
          {{ t(`subscription.sponsors.${sponsor.providerId}.description`) }}
        </div>
        <v-spacer />
        <div class="d-flex align-center ga-2 mt-auto">
          <v-btn
            size="small"
            color="deep-purple"
            variant="flat"
            append-icon="mdi-open-in-new"
            @click="openProviderPromotion(sponsor.providerId)"
          >
            {{ t('subscription.visitSite') }}
          </v-btn>
          <v-btn
            v-if="providerConsoleLinks[sponsor.providerId]"
            size="small"
            variant="text"
            append-icon="mdi-open-in-new"
            @click="openProviderConsole(sponsor.providerId)"
          >
            {{ t('subscription.visitConsole') }}
          </v-btn>
        </div>
      </v-card>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from '@/i18n'
import { getProviderTemplates, type ProviderTemplate } from '@/services/autopilot-api'
import {
  providerConsoleLinks,
  providerPromotionLinks,
  openProviderConsole,
  openProviderPromotion,
} from '@/utils/provider-links'
import runapiLogo from '@/assets/runapi.svg'

const { t } = useI18n()
const emit = defineEmits<{
  select: [provider: string]
  add: [providerId: string]
}>()
const selectedProvider = ref('')

// 内置服务商模板（与「快速添加」同源，仅用于展示 + 发起添加）
const builtinProviders = ref<ProviderTemplate[]>([])
const builtinLoading = ref(false)

// 赞助商渠道：无内置模板，仅展示 + 推广外链
const sponsors = [{ providerId: 'runapi', displayName: 'RunAPI', logo: runapiLogo }]

function selectProvider(provider: string) {
  selectedProvider.value = provider
  emit('select', provider)
}

// 能力标签：由 routes 的渠道协议推导，去重保持顺序
function capabilityKinds(provider: ProviderTemplate): string[] {
  const kinds = new Set<string>()
  for (const route of provider.routes ?? []) {
    if (route.channelKind) kinds.add(route.channelKind)
  }
  if (kinds.size === 0 && provider.channelKind) kinds.add(provider.channelKind)
  return [...kinds]
}

function kindLabel(kind: string): string {
  const key = `autopilot.diagnose.kind.${kind}`
  const label = t(key)
  return label === key ? kind : label
}

onMounted(async () => {
  builtinLoading.value = true
  try {
    builtinProviders.value = await getProviderTemplates()
  } catch (err) {
    console.error('[Subscription-Providers] 加载内置服务商失败:', err)
    builtinProviders.value = []
  } finally {
    builtinLoading.value = false
  }
})
</script>

<style scoped>
.provider-card {
  min-width: 240px;
  max-width: 300px;
  flex: 1;
  transition: all 0.2s ease;
}
.provider-card:hover {
  border-color: rgb(var(--v-theme-primary));
  background-color: rgba(var(--v-theme-primary), 0.04);
}
.provider-card--active {
  border-color: rgb(var(--v-theme-primary));
  background-color: rgba(var(--v-theme-primary), 0.08);
}
.provider-card--sponsor:hover {
  border-color: rgb(var(--v-theme-deep-purple, var(--v-theme-secondary)));
}
.provider-card__desc {
  display: -webkit-box;
  -webkit-line-clamp: 3;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
.cursor-pointer {
  cursor: pointer;
}
.sponsor-logo {
  width: 32px;
  height: 32px;
  border-radius: 6px;
  object-fit: cover;
  display: block;
}
</style>
