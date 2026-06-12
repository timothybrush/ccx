<script setup lang="ts">
import { computed, markRaw, ref } from 'vue'
import type { ChannelRecentActivity, ActivitySegment } from '@/services/admin-api'
import { expandSparseSegments } from '@/services/admin-api'

const props = defineProps<{
  activity?: ChannelRecentActivity
  maxRequests?: number
}>()

// Bar 模型：与 Web UI 一致的结构
type ActivityBar = { x: number; y: number; width: number; height: number; radius: number; g: number; v: 0 | 1 }

// 持久化缓存：避免每次重新分配 150 个对象
const persistentCache = ref<{ segments: ActivitySegment[], bars: ActivityBar[] } | null>(null)

// 计算活动柱状图数据
const activityBars = computed(() => {
  if (!props.activity) return []

  const existing = persistentCache.value
  const segments = expandSparseSegments(props.activity, existing?.segments)
  const numSegments = segments.length // 150

  if (numSegments === 0) return []

  // 每个 segment 一个 bar
  const barWidth = 150 / numSegments
  const barGap = barWidth * 0.2 // 20% 间隙
  const actualBarWidth = barWidth - barGap

  // 找到最大请求数用于归一化
  const currentMax = Math.max(...segments.map(s => s.requestCount), 1)
  const maxRequests = props.maxRequests || currentMax

  // 复用已有的 bars 数组
  let bars: ActivityBar[]
  if (existing && existing.bars.length === numSegments) {
    bars = existing.bars
  } else {
    bars = new Array(numSegments)
    for (let i = 0; i < numSegments; i++) {
      bars[i] = { x: 0, y: 0, width: 0, height: 0, radius: 0, g: 0, v: 0 }
    }
  }

  for (let i = 0; i < numSegments; i++) {
    const segment = segments[i]
    const requests = segment.requestCount

    // 无请求：v=0 跳过渲染
    if (requests <= 0) {
      bars[i].v = 0
      bars[i].height = 0
      continue
    }

    // 成功率映射到 7 档 gradient id
    const successCount = requests - segment.failureCount
    const successRate = (successCount / requests) * 100
    let g: number
    if (successRate < 5) g = 6
    else if (successRate < 20) g = 5
    else if (successRate < 40) g = 4
    else if (successRate < 60) g = 3
    else if (successRate < 80) g = 2
    else if (successRate < 95) g = 1
    else g = 0

    // 计算柱高（最小高度 2）
    const heightPercent = requests / maxRequests
    const height = Math.max(heightPercent * 85, 2)

    bars[i].v = 1
    bars[i].x = i * barWidth + barGap / 2
    bars[i].y = 100 - height
    bars[i].width = actualBarWidth
    bars[i].height = height
    bars[i].radius = Math.min(actualBarWidth / 2, 1.5)
    bars[i].g = g
  }

  // 更新持久化缓存（markRaw 防止响应式 Proxy）
  persistentCache.value = { segments, bars: markRaw(bars) }
  return bars
})
</script>

<template>
  <svg
    class="absolute inset-0 h-full w-full pointer-events-none"
    preserveAspectRatio="none"
    viewBox="0 0 150 100"
  >
    <template v-for="(bar, i) in activityBars" :key="i">
      <rect
        v-if="bar.v"
        :x="bar.x"
        :y="bar.y"
        :width="bar.width"
        :height="bar.height"
        :fill="`url(#ccx-act-g${bar.g})`"
        :rx="bar.radius"
        :ry="bar.radius"
        class="activity-bar"
      />
    </template>
  </svg>
</template>

<style scoped>
.activity-bar {
  transition: none;
}
</style>
