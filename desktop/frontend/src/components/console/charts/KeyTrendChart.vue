<template>
  <div class="key-trend-chart">
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
    </div>

    <!-- Loading state -->
    <div v-if="loading" class="flex items-center justify-center" style="height: 200px">
      <div class="w-6 h-6 border-2 border-primary border-t-transparent rounded-full animate-spin" />
    </div>

    <!-- Empty state -->
    <div
      v-else-if="!hasData"
      class="flex flex-col items-center justify-center text-muted-foreground"
      style="height: 200px"
    >
      <div class="text-2xl mb-2 opacity-40">&#x1F517;</div>
      <div class="text-xs">暂无 Key 使用数据</div>
    </div>

    <!-- Chart -->
    <div v-else>
      <VueApexCharts
        ref="chartRef"
        type="area"
        height="280"
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
import type { KeyHistoryData, GlobalStatsSummary } from '@/services/admin-api'

const { t } = useI18n()
const props = withDefaults(
  defineProps<{
    data: KeyHistoryData[]
    channelName: string
    loading?: boolean
    duration?: string
    summary?: GlobalStatsSummary | null
  }>(),
  {
    loading: false,
    duration: '1h',
    summary: null,
  },
)

const { theme } = useTheme()
const chartRef = ref<InstanceType<typeof VueApexCharts> | null>(null)

const isDark = computed(() => {
  if (theme.value === 'dark') return true
  if (theme.value === 'auto') return window.matchMedia('(prefers-color-scheme: dark)').matches
  return false
})

const textColor = computed(() => (isDark.value ? '#94a3b8' : '#64748b'))
const gridBorder = computed(() => (isDark.value ? 'rgba(255,255,255,0.06)' : 'rgba(0,0,0,0.06)'))

// Key color palette - supports up to 10 keys
const KEY_COLORS = [
  '#3b82f6', '#f97316', '#10b981', '#8b5cf6', '#ec4899',
  '#eab308', '#06b6d4', '#f43f5e', '#84cc16', '#6366f1',
]

const hasData = computed(() => {
  if (!props.data?.length) return false
  return props.data.some(k => k.dataPoints && k.dataPoints.length > 0)
})

const xLabelFormat = computed(() =>
  props.duration === '7d' || props.duration === '30d' ? 'MM-dd HH:mm' : 'HH:mm',
)

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

  // Aggregate all keys by aligned timestamp
  const timeMap = new Map<number, { totalRequests: number; totalFailures: number }>()
  for (const keyData of props.data) {
    for (const dp of keyData.dataPoints || []) {
      const rawTs = new Date(dp.timestamp).getTime()
      const alignedTs = Math.floor(rawTs / interval) * interval
      const existing = timeMap.get(alignedTs) || { totalRequests: 0, totalFailures: 0 }
      existing.totalRequests += dp.requestCount
      existing.totalFailures += dp.failureCount
      timeMap.set(alignedTs, existing)
    }
  }

  const rates = Array.from(timeMap.entries())
    .map(([timestamp, d]) => ({
      timestamp,
      failureRate: d.totalRequests > 0 ? d.totalFailures / d.totalRequests : 0,
    }))
    .sort((a, b) => a.timestamp - b.timestamp)

  const MAX_INTERVAL = interval * 2

  return rates
    .filter(p => p.failureRate >= FAILURE_RATE_THRESHOLD)
    .map((point, index, arr) => {
      let pointInterval = interval
      if (arr.length > 1) {
        if (index > 0) {
          pointInterval = point.timestamp - arr[index - 1].timestamp
        } else if (index < arr.length - 1) {
          pointInterval = arr[index + 1].timestamp - point.timestamp
        }
      }
      pointInterval = Math.min(pointInterval, MAX_INTERVAL)

      return {
        x: point.timestamp - pointInterval / 2,
        x2: point.timestamp + pointInterval / 2,
        fillColor: '#ef4444',
        opacity: getFailureOpacity(point.failureRate),
        label: { text: '' },
      }
    })
})

const chartOptions = computed<ApexOptions>(() => {
  const keyCount = props.data?.length || 0
  const colors = props.data?.map((_, i) => KEY_COLORS[i % KEY_COLORS.length]) || KEY_COLORS

  return {
    chart: {
      toolbar: { show: false },
      zoom: { enabled: false },
      background: 'transparent',
      fontFamily: 'inherit',
      defaultLocale: 'en',
      stacked: true,
      animations: { enabled: true, speed: 400 },
    },
    theme: { mode: isDark.value ? 'dark' : 'light' },
    colors,
    fill: {
      type: 'gradient',
      gradient: { shadeIntensity: 1, opacityFrom: 0.4, opacityTo: 0.08, stops: [0, 90, 100] },
    },
    dataLabels: { enabled: false },
    stroke: { curve: 'smooth', width: 2 },
    grid: { borderColor: gridBorder.value, padding: { left: 10, right: 10 } },
    xaxis: {
      type: 'datetime',
      labels: {
        datetimeUTC: false,
        format: xLabelFormat.value,
        style: { fontSize: '11px', colors: textColor.value },
      },
      axisBorder: { show: false },
      axisTicks: { show: false },
    },
    yaxis: {
      labels: {
        formatter: (val: number) => Math.round(val).toString(),
        style: { fontSize: '11px', colors: textColor.value },
      },
      min: 0,
    },
    tooltip: {
      x: { format: 'MM-dd HH:mm' },
      y: { formatter: (val: number) => `${Math.round(val)} 次` },
      custom: keyCount > 1 ? buildTrafficTooltip : undefined,
    },
    annotations: { xaxis: failureAnnotations.value },
    legend: {
      show: keyCount > 1,
      position: 'top',
      horizontalAlign: 'right',
      fontSize: '11px',
      markers: { size: 4 },
      labels: { colors: textColor.value },
    },
  }
})

// Custom tooltip showing per-key success/failure breakdown
const buildTrafficTooltip = ({ seriesIndex, dataPointIndex, w }: any): string => {
  if (!props.data?.length) return ''

  const timestamp = w.globals.seriesX[seriesIndex][dataPointIndex]
  const date = new Date(timestamp)
  const timeStr = date.toLocaleString('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  })

  const interval = AGGREGATION_INTERVALS[props.duration] || 60000
  const alignedTimestamp = Math.floor(timestamp / interval) * interval

  interface KeyStat {
    displayName: string
    total: number
    failure: number
    color: string
  }

  const keyStats: KeyStat[] = []
  let grandTotal = 0
  let grandFailure = 0

  const escapeHtml = (str: string): string =>
    str.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;')

  props.data.forEach((keyData, keyIndex) => {
    const matchingPoints = (keyData.dataPoints || []).filter(p => {
      const dpTs = new Date(p.timestamp).getTime()
      return Math.floor(dpTs / interval) === Math.floor(alignedTimestamp / interval)
    })

    if (matchingPoints.length > 0) {
      const aggregated = matchingPoints.reduce(
        (acc, dp) => ({
          success: acc.success + dp.successCount,
          failure: acc.failure + dp.failureCount,
          total: acc.total + dp.requestCount,
        }),
        { success: 0, failure: 0, total: 0 },
      )

      if (aggregated.total > 0) {
        const displayName = keyData.model ? `${keyData.keyMask}/${keyData.model}` : keyData.keyMask
        keyStats.push({
          displayName: escapeHtml(displayName),
          total: aggregated.total,
          failure: aggregated.failure,
          color: KEY_COLORS[keyIndex % KEY_COLORS.length],
        })
        grandTotal += aggregated.total
        grandFailure += aggregated.failure
      }
    }
  })

  if (keyStats.length === 0) return ''

  const hasFailure = grandFailure > 0
  let html = `<div style="padding: 8px 12px; font-size: 13px; line-height: 1.6; font-family: var(--font-sans);">`
  html += `<div style="font-weight: 600; margin-bottom: 6px; color: ${hasFailure ? '#ef4444' : 'inherit'};">${timeStr}</div>`

  keyStats.forEach(stat => {
    const failureRate = stat.total > 0 ? ((stat.failure / stat.total) * 100).toFixed(0) : '0'
    html += `<div style="display: flex; align-items: center; margin: 4px 0;">`
    html += `<span style="width: 10px; height: 10px; border-radius: 50%; background: ${stat.color}; margin-right: 6px;"></span>`
    html += `<span style="flex: 1;">${stat.displayName}</span>`
    html += `<span style="margin-left: 12px; font-weight: 500;">${stat.total}</span>`
    if (stat.failure > 0) {
      html += `<span style="margin-left: 6px; color: #ef4444; font-size: 12px;">(${stat.failure} 失败, ${failureRate}%)</span>`
    }
    html += `</div>`
  })

  if (keyStats.length > 1) {
    const grandFailureRate = grandTotal > 0 ? ((grandFailure / grandTotal) * 100).toFixed(1) : '0'
    html += `<div style="border-top: 1px solid rgba(128,128,128,0.3); margin-top: 6px; padding-top: 6px; font-weight: 600;">`
    html += `<span>总计: ${grandTotal} 次</span>`
    if (hasFailure) {
      html += `<span style="color: #ef4444; margin-left: 8px;">${grandFailure} 失败 (${grandFailureRate}%)</span>`
    }
    html += `</div>`
  }

  html += `</div>`
  return html
}

const chartSeries = computed(() => {
  if (!props.data?.length) return []

  return props.data.map(keyData => {
    const displayName = keyData.model ? `${keyData.keyMask}/${keyData.model}` : keyData.keyMask
    return {
      name: displayName,
      data: keyData.dataPoints.map(dp => ({
        x: new Date(dp.timestamp).getTime(),
        y: dp.requestCount,
      })),
    }
  })
})

function formatNumber(num: number): string {
  if (num >= 1000000) return (num / 1000000).toFixed(1) + 'M'
  if (num >= 1000) return (num / 1000).toFixed(1) + 'K'
  return num.toFixed(0)
}

defineExpose({ chartRef })
</script>
