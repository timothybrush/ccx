<script setup lang="ts">
import MetricsGrid from '@/components/status/MetricsGrid.vue'
import ServiceActions from '@/components/status/ServiceActions.vue'
import ServiceDetails from '@/components/status/ServiceDetails.vue'
import DiagnosticCard from '@/components/status/DiagnosticCard.vue'
import LogViewer from '@/components/status/LogViewer.vue'
import { useStatus } from '@/composables/useStatus'

const { status, loading, actionError, startService, stopService, restartService, openInBrowser, refresh } = useStatus()

const emit = defineEmits<{
  switchToDashboard: []
}>()
</script>

<template>
  <div class="space-y-4">
    <MetricsGrid :status="status" />
    <ServiceActions
      :status="status"
      :loading="loading"
      @start="startService"
      @stop="stopService"
      @restart="restartService"
      @open-web-u-i="emit('switchToDashboard')"
      @open-browser="openInBrowser"
      @refresh="refresh"
    />
    <DiagnosticCard
      v-if="actionError"
      :error="actionError"
      @dismiss="actionError = ''"
    />
    <DiagnosticCard
      v-else-if="status.lastError"
      :error="status.lastError"
    />
    <ServiceDetails :status="status" />
    <LogViewer :logs="status.logs" />
  </div>
</template>
