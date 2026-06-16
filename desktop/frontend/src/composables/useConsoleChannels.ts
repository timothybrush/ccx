import { ref, computed } from 'vue'
import { useAdminApi } from '@/composables/useAdminApi'
import { mergeChannelsWithLocalData } from '@/utils/channel-merge'
import { getChannelTypeApi, type ManagedChannelType } from '@/utils/channel-type-api'
import type {
  Channel,
  ChannelsResponse,
  ChannelMetrics,
  ChannelDashboardResponse,
  ChannelRecentActivity,
  SchedulerStatsResponse,
  PingResult,
} from '@/services/admin-api'

/**
 * 管理控制台频道状态层
 *
 * 使用 composable 单例模式（模块级 state），与 Desktop 现有架构一致。
 * 替代根 frontend 的 Pinia channel store，通过 HTTP 直接调后端 /api/*。
 */

type ChannelType = ManagedChannelType
type DashboardCache = {
  metrics: ChannelMetrics[]
  stats: SchedulerStatsResponse | undefined
  recentActivity: ChannelRecentActivity[] | undefined
}

const EMPTY_CACHE: DashboardCache = { metrics: [], stats: undefined, recentActivity: undefined }

// ===== 模块级单例状态 =====

const activeTab = ref<ChannelType>('messages')

// 五种协议的频道数据（独立缓存，切换不闪烁）
const channelsByType = ref<Record<ChannelType, ChannelsResponse>>({
  messages: { channels: [], current: -1 },
  chat: { channels: [], current: -1 },
  responses: { channels: [], current: -1 },
  gemini: { channels: [], current: -1 },
  images: { channels: [], current: -1 },
})

// 五种协议的 dashboard 缓存
const dashboardCache = ref<Record<ChannelType, DashboardCache>>({
  messages: { ...EMPTY_CACHE },
  chat: { ...EMPTY_CACHE },
  responses: { ...EMPTY_CACHE },
  gemini: { ...EMPTY_CACHE },
  images: { ...EMPTY_CACHE },
})

const isPingingAll = ref(false)
const refreshError = ref('')
let refreshLoopPromise: Promise<void> | null = null
let refreshRequested = false

function translate(key: string, fallback: string): string {
  const i18n = (globalThis as any).__CCX_I18N__
  const translated = i18n?.global?.t?.(key)
  return translated && translated !== key ? translated : fallback
}

// ===== 计算属性 =====

const currentChannelsData = computed(() => channelsByType.value[activeTab.value])
const currentDashboardMetrics = computed(() => dashboardCache.value[activeTab.value].metrics)
const currentDashboardStats = computed(() => dashboardCache.value[activeTab.value].stats)
const currentDashboardRecentActivity = computed(() => dashboardCache.value[activeTab.value].recentActivity)

const activeChannelCount = computed(() => {
  const chs = currentChannelsData.value.channels
  return chs.filter(ch => ch.status === 'active' || ch.status === undefined || ch.status === '').length
})

const failoverChannelCount = computed(() => {
  const chs = currentChannelsData.value.channels
  return chs.filter(ch => ch.status !== 'disabled').length
})

// ===== 刷新逻辑 =====

async function doRefresh(tab: ChannelType) {
  const api = useAdminApi()
  try {
    // 统一 dashboard 接口：GET /api/messages/channels/dashboard?type=<tab>
    const dashboard = await api.get<ChannelDashboardResponse>(
      `/api/messages/channels/dashboard?type=${tab}`
    )
    const existing = channelsByType.value[tab].channels
    channelsByType.value[tab] = {
      channels: mergeChannelsWithLocalData(dashboard.channels, existing),
      current: typeof dashboard.current === 'number' ? dashboard.current : channelsByType.value[tab].current,
    }
    dashboardCache.value[tab] = {
      metrics: dashboard.metrics,
      stats: dashboard.stats,
      recentActivity: dashboard.recentActivity,
    }
    refreshError.value = ''
  } catch (e) {
    const msg = e instanceof Error ? e.message : String(e)
    // 网络层 TypeError 包装为友好提示
    refreshError.value = msg.includes('Failed to fetch')
      ? translate('adminApi.error.networkUnavailable', '服务未运行或网络不可达，请检查后端是否已启动')
      : msg
  }
}

async function refreshChannels() {
  refreshRequested = true
  if (refreshLoopPromise) return refreshLoopPromise

  refreshLoopPromise = (async () => {
    try {
      while (refreshRequested) {
        refreshRequested = false
        await doRefresh(activeTab.value)
      }
    } finally {
      refreshLoopPromise = null
    }
  })()

  return refreshLoopPromise
}

// ===== 频道 CRUD 操作 =====

async function saveChannel(
  payload: Omit<Channel, 'index' | 'latency' | 'status'>,
  editingIndex: number | null,
) {
  const typeApi = getChannelTypeApi(activeTab.value)
  if (editingIndex !== null) {
    await typeApi.updateChannel(editingIndex, payload)
    await refreshChannels()
    return { success: true, messageKey: 'channelEditor.toast.updated' }
  }
  await typeApi.addChannel(payload)
  await refreshChannels()
  return { success: true, messageKey: 'channelEditor.toast.added' }
}

async function deleteChannel(channelId: number) {
  const typeApi = getChannelTypeApi(activeTab.value)
  await typeApi.deleteChannel(channelId)
  await refreshChannels()
}

async function pingChannel(channelId: number) {
  const typeApi = getChannelTypeApi(activeTab.value)
  const result = await typeApi.pingChannel(channelId) as PingResult
  const channel = channelsByType.value[activeTab.value].channels.find(c => c.index === channelId)
  if (channel) {
    channel.latency = result.latency
    channel.latencyTestTime = Date.now()
  }
  return result
}

async function pingAllChannels() {
  if (isPingingAll.value) return
  isPingingAll.value = true
  try {
    const typeApi = getChannelTypeApi(activeTab.value)
    const results = await typeApi.pingAll() as PingResult[]
    const now = Date.now()
    const channels = channelsByType.value[activeTab.value].channels
    for (const result of results) {
      const ch = channels.find(c => c.index === (result as any).id)
      if (ch) {
        ch.latency = result.latency
        ch.latencyTestTime = now
      }
    }
  } finally {
    isPingingAll.value = false
  }
}

async function reorderChannels(order: number[]) {
  const typeApi = getChannelTypeApi(activeTab.value)
  await typeApi.reorder(order)
  await refreshChannels()
}

async function setChannelStatus(channelId: number, status: 'active' | 'suspended' | 'disabled') {
  const typeApi = getChannelTypeApi(activeTab.value)
  await typeApi.setStatus(channelId, status)
  await refreshChannels()
}

async function resumeChannel(channelId: number) {
  const typeApi = getChannelTypeApi(activeTab.value)
  const result = await typeApi.resume(channelId)
  await typeApi.setStatus(channelId, 'active')
  await refreshChannels()
  return result
}

async function promoteChannel(channelId: number, durationSeconds: number) {
  const typeApi = getChannelTypeApi(activeTab.value)
  await typeApi.promote(channelId, durationSeconds)
  await refreshChannels()
}

// ===== Key 管理 =====

async function addApiKey(channelId: number, key: string) {
  const typeApi = getChannelTypeApi(activeTab.value)
  await typeApi.addApiKey(channelId, key)
  await refreshChannels()
}

async function removeApiKey(channelId: number, key: string) {
  const typeApi = getChannelTypeApi(activeTab.value)
  await typeApi.removeApiKey(channelId, key)
  await refreshChannels()
}

async function restoreApiKey(channelId: number, key: string) {
  const typeApi = getChannelTypeApi(activeTab.value)
  await typeApi.restoreApiKey(channelId, key)
  await refreshChannels()
}

async function moveApiKeyToTop(channelId: number, key: string) {
  const typeApi = getChannelTypeApi(activeTab.value)
  await typeApi.moveApiKeyToTop(channelId, key)
  await refreshChannels()
}

async function moveApiKeyToBottom(channelId: number, key: string) {
  const typeApi = getChannelTypeApi(activeTab.value)
  await typeApi.moveApiKeyToBottom(channelId, key)
  await refreshChannels()
}

export function useConsoleChannels() {
  return {
    // 状态
    activeTab,
    channelsByType,
    dashboardCache,
    isPingingAll,
    refreshError,

    // 当前 tab 的计算属性
    currentChannelsData,
    currentDashboardMetrics,
    currentDashboardStats,
    currentDashboardRecentActivity,
    activeChannelCount,
    failoverChannelCount,

    // 操作
    refreshChannels,
    saveChannel,
    deleteChannel,
    pingChannel,
    pingAllChannels,
    reorderChannels,
    setChannelStatus,
    resumeChannel,
    promoteChannel,

    // Key 管理
    addApiKey,
    removeApiKey,
    restoreApiKey,
    moveApiKeyToTop,
    moveApiKeyToBottom,

  }
}
