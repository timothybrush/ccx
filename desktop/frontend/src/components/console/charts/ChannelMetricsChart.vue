<template>
  <div class="channel-metrics-chart">
    <!-- Loading state -->
    <div v-if="loading" class="flex items-center justify-center" style="height: 150px">
      <div class="w-6 h-6 border-2 border-primary border-t-transparent rounded-full animate-spin" />
    </div>

    <!-- Empty state -->
    <div
      v-else-if="!hasData"
      class="flex flex-col items-center justify-center text-muted-foreground"
      style="height: 150px"
    >
      <div class="text-2xl mb-2 opacity-40">&#x1F4C9;</div>
      <div class="text-xs">暂无 {{ channelName }} 的指标数据</div>
    </div>

    <!-- Charts -->
    <div v-else class="grid grid-cols-1 md:grid-cols-2 gap-4">
      <!-- Request count area chart -->
      <div>
        <div class="text-xs text-muted-foreground font-medium mb-1">请求数</div>
        <VueApexCharts
          type="area"
          height="120"
          :options="requestCountOptions"
          :series="requestCountSeries"
        />
      </div>

      <!-- Success rate line chart -->
      <div>
        <div class="text-xs text-muted-foreground font-medium mb-1">{{ t('chart.successRate') }}</div>
        <VueApexCharts
          type="line"
          height="120"
          :options="successRateOptions"
          :series="successRateSeries"
        />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import VueApexCharts from 'vue3-apexcharts'
import type { ApexOptions } from 'apexcharts'
import { useTheme } from '@/composables/useTheme'
import { useI18n } from '@/i18n'
import type { HistoryDataPoint } from '@/services/admin-api'

const { t } = useI18n()
const props = withDefaults(
  defineProps<{
    data: HistoryDataPoint[]
    channelName: string
    loading?: boolean
    duration?: string
  }>(),
  {
    loading: false,
    duration: '6h',
  },
)

const { theme } = useTheme()

const isDark = computed(() => {
  if (theme.value === 'dark') return true
  if (theme.value === 'auto') return window.matchMedia('(prefers-color-scheme: dark)').matches
  return false
})

const chartColor = '#3b82f6'
const textColor = computed(() => (isDark.value ? '#94a3b8' : '#64748b'))
const gridBorder = computed(() => (isDark.value ? 'rgba(255,255,255,0.06)' : 'rgba(0,0,0,0.06)'))

const hasData = computed(() => {
  if (!props.data?.length) return false
  return props.data.some(dp => dp.requestCount > 0)
})

const xLabelFormat = computed(() =>
  props.duration === '7d' || props.duration === '30d' ? 'MM-dd HH:mm' : 'HH:mm',
)

const baseChartOptions = computed<ApexOptions>(() => ({
  chart: {
    toolbar: { show: false },
    zoom: { enabled: false },
    background: 'transparent',
    fontFamily: 'inherit',
    defaultLocale: 'en',
    sparkline: { enabled: false },
  },
  theme: { mode: isDark.value ? 'dark' : 'light' },
  grid: {
    borderColor: gridBorder.value,
    strokeDashArray: 3,
    padding: { left: 10, right: 10 },
  },
  xaxis: {
    type: 'datetime',
    labels: {
      datetimeUTC: false,
      format: xLabelFormat.value,
      style: { fontSize: '10px', colors: textColor.value },
    },
    axisBorder: { show: false },
    axisTicks: { show: false },
  },
  yaxis: { labels: { style: { fontSize: '10px', colors: textColor.value } } },
  tooltip: { x: { format: 'MM-dd HH:mm' } },
  legend: { show: false },
  stroke: { curve: 'smooth', width: 2 },
}))

const requestCountOptions = computed<ApexOptions>(() => ({
  ...baseChartOptions.value,
  colors: [chartColor],
  fill: {
    type: 'gradient',
    gradient: { shadeIntensity: 1, opacityFrom: 0.4, opacityTo: 0.1, stops: [0, 90, 100] },
  },
  yaxis: {
    min: 0,
    labels: {
      formatter: (val: number) => Math.round(val).toString(),
      style: { fontSize: '10px', colors: textColor.value },
    },
  },
  dataLabels: { enabled: false },
}))

const successRateOptions = computed<ApexOptions>(() => ({
  ...baseChartOptions.value,
  colors: ['#10b981'],
  yaxis: {
    min: 0,
    max: 100,
    labels: {
      formatter: (val: number) => `${val.toFixed(0)}%`,
      style: { fontSize: '10px', colors: textColor.value },
    },
  },
  dataLabels: { enabled: false },
  markers: { size: 2, hover: { size: 4 } },
}))

const requestCountSeries = computed(() => {
  if (!props.data?.length) return []
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

const successRateSeries = computed(() => {
  if (!props.data?.length) return []
  return [
    {
      name: t('chart.successRate'),
      data: props.data
        .filter(dp => dp.requestCount > 0)
        .map(dp => ({
          x: new Date(dp.timestamp).getTime(),
          y: dp.successRate,
        })),
    },
  ]
})
</script>
