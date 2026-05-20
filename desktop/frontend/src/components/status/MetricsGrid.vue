<script setup lang="ts">
import { computed, ref, watch, onMounted, onUnmounted } from 'vue'
import type { DesktopStatus } from '@/types'
import { Server, Clock, GitBranch, ArrowUpRight } from 'lucide-vue-next'

const props = defineProps<{
  status: DesktopStatus
}>()

// 本地微秒级仿真模拟时钟，实现毫秒级自增长进化
const localUptime = ref(0)
let localTimer: ReturnType<typeof setInterval> | undefined

// 监听远程 uptime，定时与其物理同步校准，避免前端漂移
watch(() => props.status.health?.uptime, (newVal) => {
  if (typeof newVal === 'number') {
    localUptime.value = newVal
  }
}, { immediate: true })

// 100ms 物理高频仿真自增时钟
const startLocalTimer = () => {
  localTimer = setInterval(() => {
    // 只有在网关 running 时才进行高频累增仿真，精度保持一位小数
    if (props.status.running && props.status.health?.uptime) {
      localUptime.value = Math.round((localUptime.value + 0.1) * 10) / 10
    }
  }, 100)
}

onMounted(() => {
  startLocalTimer()
})

onUnmounted(() => {
  if (localTimer) {
    clearInterval(localTimer)
  }
})

// 辅助格式化函数：最多保留 1 位小数，若是整数则不显示小数点
const formatDecimal = (num: number) => {
  const val = Math.round(num * 10) / 10
  return val % 1 === 0 ? String(val) : val.toFixed(1)
}

// 自动演进计时器设计
const uptimeDisplay = computed(() => {
  const t = localUptime.value
  if (!props.status.running || !t) return '——'

  // 1. 进化阶段 A（小于 1 分钟）：显示为秒，如 12.3s，由于 100ms 计时，数值将在眼前丝滑流动累加
  if (t < 60) {
    return `${formatDecimal(t)}s`
  }

  // 2. 进化阶段 B（小于 1 小时 / 3600 秒）：显示为分钟，如 12.4m
  if (t < 3600) {
    return `${formatDecimal(t / 60)}m`
  }

  // 3. 进化阶段 C（小于 1 天 / 86400 秒）：显示为小时，如 4.2h
  if (t < 86400) {
    return `${formatDecimal(t / 3600)}h`
  }

  // 4. 进化阶段 D（大于 1 天）：进化为天，如 3.1d
  return `${formatDecimal(t / 86400)}d`
})
</script>

<template>
  <div class="grid grid-cols-2 lg:grid-cols-4 gap-4 select-none">
    <!-- 1. 网关运行端口 -->
    <div class="bg-glass bg-glass-hover rounded-xl p-4.5 border border-white/[0.03] flex flex-col justify-between group">
      <div class="flex items-center justify-between">
        <span class="text-[11px] font-bold tracking-wider text-slate-500 uppercase">网关端口</span>
        <div class="p-1.5 rounded-lg bg-blue-500/10 border border-blue-500/15 group-hover:bg-blue-500/20 transition-colors">
          <Server class="w-3.5 h-3.5 text-blue-400" />
        </div>
      </div>
      <div class="mt-4 flex items-baseline gap-1.5 min-w-0">
        <span class="text-xl font-bold font-mono tracking-tight text-slate-100 truncate">
          {{ status.port || '——' }}
        </span>
        <span class="text-[9px] font-bold text-emerald-400 bg-emerald-500/10 px-1 py-0.2 rounded border border-emerald-500/15 uppercase tracking-wide shrink-0" v-if="status.running">
          Active
        </span>
      </div>
    </div>

    <!-- 2. 网关运行时长 (自适应秒、分、时、天自动流式演进) -->
    <div class="bg-glass bg-glass-hover rounded-xl p-4.5 border border-white/[0.03] flex flex-col justify-between group">
      <div class="flex items-center justify-between">
        <span class="text-[11px] font-bold tracking-wider text-slate-500 uppercase">运行时长</span>
        <div class="p-1.5 rounded-lg bg-emerald-500/10 border border-emerald-500/15 group-hover:bg-emerald-500/20 transition-colors">
          <Clock class="w-3.5 h-3.5 text-emerald-400" />
        </div>
      </div>
      <div class="mt-4 flex items-baseline gap-1.5 min-w-0">
        <span class="text-xl font-bold font-mono tracking-tight text-slate-100 truncate">
          {{ uptimeDisplay }}
        </span>
        <span class="text-[9px] text-emerald-500/80 font-bold tracking-wide uppercase shrink-0" v-if="status.running">
          Evolving
        </span>
      </div>
    </div>

    <!-- 3. 上游渠道数 -->
    <div class="bg-glass bg-glass-hover rounded-xl p-4.5 border border-white/[0.03] flex flex-col justify-between group">
      <div class="flex items-center justify-between">
        <span class="text-[11px] font-bold tracking-wider text-slate-500 uppercase">调度信道</span>
        <div class="p-1.5 rounded-lg bg-indigo-500/10 border border-indigo-500/15 group-hover:bg-indigo-500/20 transition-colors">
          <GitBranch class="w-3.5 h-3.5 text-indigo-400" />
        </div>
      </div>
      <div class="mt-4 flex items-baseline gap-1.5 min-w-0">
        <span class="text-xl font-bold font-mono tracking-tight text-slate-100 truncate">
          {{ status.health?.config?.upstreamCount || 0 }}
        </span>
        <span class="text-[9px] text-indigo-400 bg-indigo-500/10 px-1 py-0.2 rounded border border-indigo-500/15 uppercase tracking-wide shrink-0">
          Channels
        </span>
      </div>
    </div>

    <!-- 4. 网关版本 -->
    <div class="bg-glass bg-glass-hover rounded-xl p-4.5 border border-white/[0.03] flex flex-col justify-between group">
      <div class="flex items-center justify-between">
        <span class="text-[11px] font-bold tracking-wider text-slate-500 uppercase">网关版本</span>
        <div class="p-1.5 rounded-lg bg-slate-500/10 border border-slate-500/15 group-hover:bg-slate-500/20 transition-colors">
          <ArrowUpRight class="w-3.5 h-3.5 text-slate-400" />
        </div>
      </div>
      <div class="mt-4 flex items-baseline gap-1.5 min-w-0">
        <span class="text-xl font-bold font-mono tracking-tight text-slate-100 truncate">
          {{ status.health?.version?.version || 'v0.0.0' }}
        </span>
        <span class="text-[9px] text-slate-500 font-medium shrink-0">
          stable
        </span>
      </div>
    </div>
  </div>
</template>
