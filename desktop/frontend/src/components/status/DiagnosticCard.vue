<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import { AlertTriangle, XCircle, HardDrive, Network, Clock, ShieldAlert, X } from 'lucide-vue-next'

const props = defineProps<{
  error: string
}>()

const emit = defineEmits<{
  dismiss: []
}>()

const dismissed = ref(false)

watch(
  () => props.error,
  (val) => {
    dismissed.value = false
    if (!val) dismissed.value = false
  },
)

const visible = computed(() => props.error && !dismissed.value)

type ErrorKind = 'binary' | 'port' | 'health' | 'permission' | 'generic'

interface DiagnosticInfo {
  kind: ErrorKind
  icon: typeof AlertTriangle
  title: string
  suggestions: string[]
  color: string
}

const patterns: { re: RegExp; kind: ErrorKind }[] = [
  { re: /未找到.*二进制|binary.*not.*found/i, kind: 'binary' },
  { re: /端口.*冲突|端口.*占用|no.*available.*port/i, kind: 'port' },
  { re: /connection.*refused|连接.*拒绝/i, kind: 'port' },
  { re: /health.*超时|等待.*health|health.*timeout/i, kind: 'health' },
  { re: /permission.*denied|权限|access.*denied|不允许/i, kind: 'permission' },
]

const kindDefaults: Record<ErrorKind, Omit<DiagnosticInfo, 'kind'>> = {
  binary: {
    icon: HardDrive,
    title: '二进制文件未找到',
    color: 'text-amber-400',
    suggestions: [
      '确认 CCX 二进制已构建: cd backend-go && make build',
      '检查 Desktop 数据目录中是否存在 ccx-go / ccx-go.exe',
      '首次使用需先构建后端，或从 Release 页面下载预编译版本',
    ],
  },
  port: {
    icon: Network,
    title: '端口冲突',
    color: 'text-orange-400',
    suggestions: [
      '检查是否有其他 CCX 实例已在运行',
      '修改 .env 中 PORT 字段使用其他端口',
      '使用 lsof -i :3688 (macOS/Linux) 或 netstat -ano | findstr :3688 (Windows) 检查端口占用',
    ],
  },
  health: {
    icon: Clock,
    title: '健康检查超时',
    color: 'text-amber-400',
    suggestions: [
      '查看日志面板中是否有启动错误信息',
      '检查 .env 配置是否有语法错误',
      '确认上游渠道配置正确，首次启动可能需要较长时间',
      '尝试手动重启服务',
    ],
  },
  permission: {
    icon: ShieldAlert,
    title: '权限不足',
    color: 'text-rose-400',
    suggestions: [
      '检查数据目录是否有写入权限',
      'macOS/Linux: 确认二进制文件有执行权限 (chmod +x)',
      'Windows: 尝试以管理员身份运行',
    ],
  },
  generic: {
    icon: XCircle,
    title: '启动失败',
    color: 'text-rose-400',
    suggestions: [
      '查看下方日志面板获取详细错误信息',
      '尝试重启服务',
    ],
  },
}

const diagnostic = computed<DiagnosticInfo>(() => {
  const msg = props.error
  for (const { re, kind } of patterns) {
    if (re.test(msg)) {
      return { kind, ...kindDefaults[kind] }
    }
  }
  return { kind: 'generic', ...kindDefaults.generic }
})
</script>

<template>
  <div
    v-if="visible"
    class="rounded-lg border border-rose-500/20 bg-rose-500/5 backdrop-blur-sm px-4 py-3"
  >
    <div class="flex items-start gap-3">
      <component :is="diagnostic.icon" :class="['h-5 w-5 mt-0.5 shrink-0', diagnostic.color]" />
      <div class="flex-1 min-w-0 space-y-2">
        <div class="flex items-center justify-between gap-2">
          <h4 :class="['text-sm font-semibold', diagnostic.color]">{{ diagnostic.title }}</h4>
          <button
            class="text-slate-500 hover:text-slate-300 transition-colors shrink-0"
            @click="dismissed = true; emit('dismiss')"
          >
            <X class="h-4 w-4" />
          </button>
        </div>
        <p class="text-xs text-slate-400 font-mono break-all leading-relaxed">{{ error }}</p>
        <ul class="space-y-1 pt-1">
          <li
            v-for="(suggestion, i) in diagnostic.suggestions"
            :key="i"
            class="text-xs text-slate-400 flex items-start gap-1.5"
          >
            <span class="text-slate-600 mt-px">-</span>
            <span>{{ suggestion }}</span>
          </li>
        </ul>
      </div>
    </div>
  </div>
</template>
