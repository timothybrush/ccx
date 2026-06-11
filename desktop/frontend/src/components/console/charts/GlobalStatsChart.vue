<template>
  <div class="global-stats-chart">
    <!-- Summary cards -->
    <div v-if="summary" class="flex flex-wrap gap-2 mb-3">
      <div class="flex-1 min-w-[80px] p-2 rounded-lg text-center bg-secondary/30 dark:bg-secondary/20">
        <div class="text-xs text-muted-foreground font-medium mb-1">{{ t('chart.totalRequests') }}</div>
        <div class="text-sm font-semibold">{{ formatNumber(summary.totalRequests) }}</div>
      </div>
      <div class="flex-1 min-w-[80px] p-2 rounded-lg text-center bg-secondary/30 dark:bg-secondary/20">
        <div class="text-xs text-muted-foreground font-medium mb-1">{{ t('chart.successRate') }}</div>
        <div
          class="text-sm font-semibold"
          :class="{
            'text-accent': summary.avgSuccessRate >= 95,
            'text-warning': summary.avgSuccessRate >= 80 && summary.avgSuccessRate < 95,
            'text-destructive': summary.avgSuccessRate < 80,
          }"
        >
          {{ summary.avgSuccessRate.toFixed(1) }}%
        </div>
      </div>
      <div class="flex-1 min-w-[80px] p-2 rounded-lg text-center bg-secondary/30 dark:bg-secondary/20">
        <div class="text-xs text-muted-foreground font-medium mb-1">{{ t('chart.inputTokens') }}</div>
        <div class="text-sm font-semibold">{{ formatNumber(summary.totalInputTokens) }}</div>
      </div>
      <div class="flex-1 min-w-[80px] p-2 rounded-lg text-center bg-secondary/30 dark:bg-secondary/20">
        <div class="text-xs text-muted-foreground font-medium mb-1">{{ t('chart.outputTokens') }}</div>
        <div class="text-sm font-semibold">{{ formatNumber(summary.totalOutputTokens) }}</div>
      </div>
      <div
        v-if="summary.totalCacheReadTokens > 0 || summary.totalCacheCreationTokens > 0"
        class="flex-1 min-w-[80px] p-2 rounded-lg text-center bg-secondary/30 dark:bg-secondary/20"
      >
        <div class="text-xs text-muted-foreground font-medium mb-1">{{ t('chart.cacheRw') }}</div>
        <div class="text-sm font-semibold">
          {{ formatNumber(summary.totalCacheReadTokens) }} / {{ formatNumber(summary.totalCacheCreationTokens) }}
        </div>
      </div>
    </div>

    <!-- Empty state -->
    <div
      v-if="!hasData"
      class="flex flex-col items-center justify-center text-muted-foreground"
      :style="{ height: chartHeight + 'px' }"
    >
      <div class="text-2xl mb-2 opacity-40">&#x1F4C8;</div>
      <div class="text-xs">{{ t('chart.noData') }}</div>
    </div>

    <!-- Chart -->
    <div v-else>
      <VueApexCharts
        ref="chartRef"
        type="area"
        :height="chartHeight"
        :options="chartOptions"
        :series="chartSeries"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import VueApexCharts from 'vue3-apexcharts'
import type { ApexOptions } from 'apexcharts'
import { useTheme } from '@/composables/useTheme'
import { useI18n } from '@/i18n'
import type { GlobalHistoryDataPoint, GlobalStatsSummary, ModelHistoryDataPoint } from '@/services/admin-api'

const { t } = useI18n()
const props = withDefaults(
  defineProps<{
    data: GlobalHistoryDataPoint[]
    duration?: string
    summary?: GlobalStatsSummary | null
    modelDataPoints?: Record<string, ModelHistoryDataPoint[]>
    compact?: boolean
  }>(),
  {
    duration: '6h',
    summary: null,
    modelDataPoints: undefined,
    compact: false,
  },
)

const { theme } = useTheme()
const chartRef = ref<InstanceType<typeof VueApexCharts> | null>(null)

const isDark = computed(() => {
  if (theme.value === 'dark') return true
  if (theme.value === 'auto') return window.matchMedia('(prefers-color-scheme: dark)').matches
  return false
})

const chartHeight = computed(() => (props.compact ? 180 : 260))

// Colors - using the desktop theme palette
const COLORS = {
  primary: '#3b82f6',
  input: '#8b5cf6',
  output: '#f97316',
  cacheRead: '#10b981',
  cacheWrite: '#06b6d4',
}

const MODEL_COLORS = [
  '#3b82f6', '#10b981', '#f97316', '#8b5cf6', '#ef4444',
  '#06b6d4', '#ec4899', '#84cc16', '#f59e0b', '#6366f1',
]

const hasData = computed(() => {
  if (!props.data?.length) return false
  return props.data.some(dp => dp.requestCount > 0)
})

const hasCacheData = computed(() => {
  if (!props.data?.length) return false
  return props.data.some(dp => (dp.cacheReadTokens || 0) > 0 || (dp.cacheCreationTokens || 0) > 0)
})

const sortedModels = computed(() => {
  if (!props.modelDataPoints) return []
  return Object.entries(props.modelDataPoints)
    .map(([name, points]) => ({
      name,
      points,
      total: points.reduce((s: number, p: ModelHistoryDataPoint) => s + p.requestCount, 0),
    }))
    .sort((a, b) => b.total - a.total)
})

const hasMultiModel = computed(() => sortedModels.value.length > 0)

// Failure rate background bands
const FAILURE_RATE_THRESHOLD = 0.1

const AGGREGATION_INTERVALS: Record<string, number> = {
  '1h': 60000,
  '6h': 300000,
  '24h': 900000,
  'today': 300000,
  '7d': 3600000,
  '30d': 14400000,
}

const getFailureOpacity = (failureRate: number): number => {
  const minOpacity = 0.08
  const maxOpacity = 0.65
  const normalizedRate = Math.min((failureRate - FAILURE_RATE_THRESHOLD) / (1 - FAILURE_RATE_THRESHOLD), 1)
  return minOpacity + normalizedRate * (maxOpacity - minOpacity)
}

const failureAnnotations = computed(() => {
  if (!props.data?.length) return []
  const interval = AGGREGATION_INTERVALS[props.duration] || 60000

  return props.data
    .filter(dp => dp.requestCount > 0 && dp.failureCount / dp.requestCount >= FAILURE_RATE_THRESHOLD)
    .map((dp, index, arr) => {
      const ts = new Date(dp.timestamp).getTime()
      let pointInterval = interval
      if (arr.length > 1) {
        if (index > 0) {
          pointInterval = ts - new Date(arr[index - 1].timestamp).getTime()
        } else if (index < arr.length - 1) {
          pointInterval = new Date(arr[index + 1].timestamp).getTime() - ts
        }
      }
      pointInterval = Math.min(pointInterval, interval * 2)
      const failureRate = dp.failureCount / dp.requestCount

      return {
        x: ts - pointInterval / 2,
        x2: ts + pointInterval / 2,
        fillColor: '#ef4444',
        opacity: getFailureOpacity(failureRate),
        label: { text: '' },
      }
    })
})

const chartOptions = computed<ApexOptions>(() => {
  const isMultiModel = hasMultiModel.value
  const trafficColors = isMultiModel
    ? sortedModels.value.map((_, i) => MODEL_COLORS[i % MODEL_COLORS.length])
    : [COLORS.primary]

  const textColor = isDark.value ? '#94a3b8' : '#64748b'
  const gridBorder = isDark.value ? 'rgba(255,255,255,0.06)' : 'rgba(0,0,0,0.06)'

  const xLabelFormat = props.duration === '7d' || props.duration === '30d' ? 'MM-dd HH:mm' : 'HH:mm'

  return {
    chart: {
      toolbar: { show: false },
      zoom: { enabled: false },
      background: 'transparent',
      fontFamily: 'inherit',
      defaultLocale: 'en',
      stacked: isMultiModel,
      animations: { enabled: true, speed: 400 },
    },
    theme: { mode: isDark.value ? 'dark' : 'light' },
    colors: hasCacheData.value
      ? [COLORS.input, COLORS.output, COLORS.cacheRead, COLORS.cacheWrite]
      : trafficColors,
    fill: {
      type: 'gradient',
      gradient: { shadeIntensity: 1, opacityFrom: 0.4, opacityTo: 0.08, stops: [0, 90, 100] },
    },
    dataLabels: { enabled: false },
    stroke: { curve: 'smooth', width: 2 },
    grid: { borderColor: gridBorder, padding: { left: 10, right: 10 } },
    xaxis: {
      type: 'datetime',
      labels: {
        datetimeUTC: false,
        format: xLabelFormat,
        style: { fontSize: '11px', colors: textColor },
      },
      axisBorder: { show: false },
      axisTicks: { show: false },
    },
    yaxis: {
      labels: {
        formatter: (val: number) => Math.round(val).toString(),
        style: { fontSize: '11px', colors: textColor },
      },
      min: 0,
    },
    tooltip: {
      x: { format: 'MM-dd HH:mm' },
      y: { formatter: (val: number) => `${Math.round(val)} 次` },
    },
    annotations: { xaxis: failureAnnotations.value },
    legend: {
      show: isMultiModel,
      position: 'top',
      horizontalAlign: 'right',
      fontSize: '11px',
      markers: { size: 4 },
      labels: { colors: textColor },
    },
  }
})

const chartSeries = computed(() => {
  if (!props.data?.length) return []

  const models = sortedModels.value
  if (models.length > 0) {
    return models.map(model => ({
      name: model.name,
      data: model.points.map((p: ModelHistoryDataPoint) => ({
        x: new Date(p.timestamp).getTime(),
        y: p.requestCount,
      })),
    }))
  }

  return [
    {
      name: '请求数',
      data: props.data.map(dp => ({
        x: new Date(dp.timestamp).getTime(),
        y: dp.requestCount,
      })),
    },
  ]
})

function formatNumber(num: number): string {
  if (num >= 1000000) return (num / 1000000).toFixed(1) + 'M'
  if (num >= 1000) return (num / 1000).toFixed(1) + 'K'
  return num.toFixed(0)
}

defineExpose({ chartRef })
</script>
