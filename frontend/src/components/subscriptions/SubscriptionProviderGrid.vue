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
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from '@/i18n'

const { t } = useI18n()
const emit = defineEmits<{ select: [provider: string] }>()
const selectedProvider = ref('')

function selectProvider(provider: string) {
  selectedProvider.value = provider
  emit('select', provider)
}
</script>

<style scoped>
.provider-card {
  min-width: 240px;
  max-width: 280px;
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
.cursor-pointer {
  cursor: pointer;
}
</style>
