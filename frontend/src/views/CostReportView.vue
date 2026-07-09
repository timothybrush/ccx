<template>
  <div class="cost-report-view">
    <!-- Header -->
    <div class="d-flex align-center justify-space-between mb-4">
      <div class="d-flex align-center">
        <v-icon size="28" class="mr-2" color="primary">mdi-cash-multiple</v-icon>
        <span class="text-h5 font-weight-bold">成本报表</span>
      </div>
      <div class="d-flex ga-2">
        <v-btn
          variant="tonal"
          prepend-icon="mdi-refresh"
          :loading="loading"
          @click="fetchReport"
        >
          刷新
        </v-btn>
        <v-btn
          variant="tonal"
          prepend-icon="mdi-download"
          :disabled="rows.length === 0"
          @click="exportCSV"
        >
          导出 CSV
        </v-btn>
      </div>
    </div>

    <!-- Filter bar -->
    <v-card class="mb-4" variant="outlined">
      <v-card-text class="d-flex align-center ga-4 flex-wrap">
        <!-- groupBy selector -->
        <div class="d-flex align-center ga-1">
          <span class="text-caption text-medium-emphasis mr-1">分组维度</span>
          <v-chip
            v-for="opt in groupByOptions"
            :key="opt.value"
            :color="groupBy === opt.value ? 'primary' : undefined"
            :variant="groupBy === opt.value ? 'flat' : 'outlined'"
            size="small"
            @click="groupBy = opt.value; fetchReport()"
          >
            {{ opt.label }}
          </v-chip>
        </div>

        <!-- duration selector -->
        <div class="d-flex align-center ga-1">
          <span class="text-caption text-medium-emphasis mr-1">时间范围</span>
          <v-chip
            v-for="opt in durationOptions"
            :key="opt.value"
            :color="duration === opt.value ? 'primary' : undefined"
            :variant="duration === opt.value ? 'flat' : 'outlined'"
            size="small"
            @click="duration = opt.value; fetchReport()"
          >
            {{ opt.label }}
          </v-chip>
        </div>

        <!-- apiType selector -->
        <v-select
          v-model="apiType"
          :items="apiTypeOptions"
          item-title="label"
          item-value="value"
          variant="outlined"
          density="compact"
          hide-details
          style="max-width: 180px"
          @update:model-value="fetchReport"
        />
      </v-card-text>
    </v-card>

    <!-- Summary cards -->
    <v-row class="mb-4">
      <v-col cols="6" sm="3">
        <v-card variant="outlined" class="pa-3 text-center">
          <div class="text-caption text-medium-emphasis">总请求数</div>
          <div class="text-h6 font-weight-bold">{{ formatNumber(totalRequests) }}</div>
        </v-card>
      </v-col>
      <v-col cols="6" sm="3">
        <v-card variant="outlined" class="pa-3 text-center">
          <div class="text-caption text-medium-emphasis">成功率</div>
          <div class="text-h6 font-weight-bold">{{ successRate }}%</div>
        </v-card>
      </v-col>
      <v-col cols="6" sm="3">
        <v-card variant="outlined" class="pa-3 text-center">
          <div class="text-caption text-medium-emphasis">总输入 Token</div>
          <div class="text-h6 font-weight-bold">{{ formatTokens(totalInputTokens) }}</div>
        </v-card>
      </v-col>
      <v-col cols="6" sm="3">
        <v-card variant="outlined" class="pa-3 text-center">
          <div class="text-caption text-medium-emphasis">官方定价成本</div>
          <div class="text-h6 font-weight-bold">${{ totalListCostUSD.toFixed(4) }}</div>
        </v-card>
      </v-col>
    </v-row>

    <!-- Loading state -->
    <div v-if="loading && rows.length === 0" class="text-center py-12">
      <v-progress-circular indeterminate color="primary" size="48" />
    </div>

    <!-- Empty state -->
    <div v-else-if="!loading && rows.length === 0" class="text-center py-12 text-medium-emphasis">
      <v-icon size="64" class="mb-4" color="grey">mdi-cash-multiple</v-icon>
      <div class="text-body-1">暂无成本数据</div>
      <div class="text-caption mt-1">需要启用 SQLite 持久化存储并有请求记录</div>
    </div>

    <!-- Data table -->
    <v-card v-else variant="outlined">
      <v-table hover>
        <thead>
          <tr>
            <th class="text-left" style="min-width: 200px">{{ groupByLabel }}</th>
            <th class="text-right">请求数</th>
            <th class="text-right">成功率</th>
            <th class="text-right">输入 Token</th>
            <th class="text-right">输出 Token</th>
            <th class="text-right">缓存创建</th>
            <th class="text-right">缓存读取</th>
            <th class="text-right">官方成本 (USD)</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="row in rows" :key="row.groupKey">
            <td class="text-left">
              <v-chip size="small" variant="tonal" color="primary">{{ row.groupKey || '(空)' }}</v-chip>
            </td>
            <td class="text-right">{{ formatNumber(row.totalRequests) }}</td>
            <td class="text-right">
              <span :class="row.successCount / row.totalRequests >= 0.95 ? 'text-success' : 'text-warning'">
                {{ ((row.successCount / row.totalRequests) * 100).toFixed(1) }}%
              </span>
            </td>
            <td class="text-right">{{ formatTokens(row.inputTokens) }}</td>
            <td class="text-right">{{ formatTokens(row.outputTokens) }}</td>
            <td class="text-right">{{ formatTokens(row.cacheCreationTokens) }}</td>
            <td class="text-right">{{ formatTokens(row.cacheReadTokens) }}</td>
            <td class="text-right font-weight-bold">${{ row.listCostUSD.toFixed(6) }}</td>
          </tr>
        </tbody>
      </v-table>
    </v-card>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { api } from '@/services/api'
import type { CostReportRow } from '@/services/api-types'

const loading = ref(false)
const rows = ref<CostReportRow[]>([])
const groupBy = ref('user')
const duration = ref('7d')
const apiType = ref('messages')

const groupByOptions = [
  { label: '用户', value: 'user' },
  { label: '模型', value: 'model' },
  { label: 'Key', value: 'key' },
]

const durationOptions = [
  { label: '24h', value: '24h' },
  { label: '7d', value: '7d' },
  { label: '30d', value: '30d' },
  { label: '90d', value: '90d' },
  { label: '365d', value: '365d' },
]

const apiTypeOptions = [
  { label: 'Messages', value: 'messages' },
  { label: 'Responses', value: 'responses' },
  { label: 'Chat', value: 'chat' },
  { label: 'Gemini', value: 'gemini' },
  { label: 'Images', value: 'images' },
  { label: 'Vectors', value: 'vectors' },
]

const groupByLabel = computed(() => {
  return groupByOptions.find(o => o.value === groupBy.value)?.label || '分组'
})

const totalRequests = computed(() => rows.value.reduce((s, r) => s + r.totalRequests, 0))
const totalSuccess = computed(() => rows.value.reduce((s, r) => s + r.successCount, 0))
const totalInputTokens = computed(() => rows.value.reduce((s, r) => s + r.inputTokens, 0))
const totalListCostUSD = computed(() => rows.value.reduce((s, r) => s + r.listCostUSD, 0))
const successRate = computed(() => {
  if (totalRequests.value === 0) return '0.0'
  return ((totalSuccess.value / totalRequests.value) * 100).toFixed(1)
})

function formatNumber(n: number): string {
  if (n >= 1_000_000) return (n / 1_000_000).toFixed(1) + 'M'
  if (n >= 1_000) return (n / 1_000).toFixed(1) + 'K'
  return n.toString()
}

function formatTokens(n: number): string {
  if (n >= 1_000_000) return (n / 1_000_000).toFixed(2) + 'M'
  if (n >= 1_000) return (n / 1_000).toFixed(1) + 'K'
  return n.toString()
}

async function fetchReport() {
  loading.value = true
  try {
    const resp = await api.getCostReport(groupBy.value, duration.value, apiType.value)
    rows.value = resp.rows || []
  } catch (e) {
    console.error('[CostReport] 获取报表失败:', e)
    rows.value = []
  } finally {
    loading.value = false
  }
}

function exportCSV() {
  if (rows.value.length === 0) return

  const headers = [
    groupByLabel.value, '请求数', '成功数', '输入Token', '输出Token',
    '缓存创建Token', '缓存读取Token', '官方成本USD'
  ]
  const csvRows = rows.value.map(r => [
    r.groupKey, r.totalRequests, r.successCount,
    r.inputTokens, r.outputTokens, r.cacheCreationTokens,
    r.cacheReadTokens, r.listCostUSD.toFixed(6),
  ])

  const csv = [headers.join(','), ...csvRows.map(r => r.join(','))].join('\n')
  const blob = new Blob(['﻿' + csv], { type: 'text/csv;charset=utf-8;' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `cost-report-${groupBy.value}-${apiType.value}-${duration.value}.csv`
  a.click()
  URL.revokeObjectURL(url)
}

onMounted(() => {
  fetchReport()
})
</script>

<style scoped>
.cost-report-view {
  padding: 16px;
}
</style>
