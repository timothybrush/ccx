<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Button } from '@/components/ui/button'
import { Alert } from '@/components/ui/alert'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Globe } from 'lucide-vue-next'
import { useStatus } from '@/composables/useStatus'
import { useLanguage } from '@/composables/useLanguage'
import { useConsoleChannels } from '@/composables/useConsoleChannels'
import {
  channelSelectionPath,
  consoleSelectionChannelType,
  consoleSelectionSection,
  isManagedChannelType,
  type ConsoleSelection,
} from '@/composables/useConsoleSelection'
import { OpenWebUIInBrowser } from '@bindings/github.com/BenedictKing/ccx/desktop/desktopservice'
import ChannelManager from '@/components/console/ChannelManager.vue'
import ConversationDashboard from '@/components/console/ConversationDashboard.vue'
import type { ManagedChannelType } from '@/utils/channel-type-api'

const props = withDefaults(defineProps<{
  selection?: ConsoleSelection
}>(), {
  selection: '/channels/messages',
})

const emit = defineEmits<{
  'update:selection': [selection: ConsoleSelection]
}>()

const { status } = useStatus()
const { t, tf } = useLanguage()
const { activeTab, refreshError } = useConsoleChannels()

// 子 Tab 定义
const protocolTabs: { value: ManagedChannelType; label: string }[] = [
  { value: 'messages', label: 'Claude' },
  { value: 'chat', label: 'OpenAI Chat' },
  { value: 'images', label: 'Images' },
  { value: 'responses', label: 'Codex' },
  { value: 'gemini', label: 'Gemini' },
]

// 管理控制台的顶级 tab：频道管理 vs 驾驶舱
const consoleTab = ref<'channels' | 'conversations'>(consoleSelectionSection(props.selection))


const applySelection = (selection: ConsoleSelection) => {
  const section = consoleSelectionSection(selection)
  consoleTab.value = section
  if (section === 'channels') {
    activeTab.value = consoleSelectionChannelType(selection)
  }
}

const updateConsoleTab = (value: string | number) => {
  const next = String(value) === 'conversations' ? 'conversations' : 'channels'
  consoleTab.value = next
  emit('update:selection', next === 'conversations' ? '/conversations' : channelSelectionPath(activeTab.value))
}

const updateProtocolTab = (value: string | number) => {
  const next = String(value)
  if (!isManagedChannelType(next)) return
  activeTab.value = next
  emit('update:selection', channelSelectionPath(next))
}


const openInBrowser = async () => {
  try {
    await OpenWebUIInBrowser()
  } catch (e) {
    console.warn('Failed to open WebUI in browser:', e)
  }
}

onMounted(() => {
  applySelection(props.selection)
})

watch(() => props.selection, (selection) => {
  applySelection(selection)
})

</script>

<template>
  <div class="h-full flex flex-col gap-4">
    <!-- 服务未运行提示 -->
    <Alert v-if="!status.running" variant="destructive" class="shrink-0">
      <p class="text-sm">
        {{ t('webui.notRunning') }}
      </p>
    </Alert>

    <!-- 加载状态 -->
    <div v-else-if="refreshError" class="shrink-0">
      <Alert variant="destructive">
        <p class="text-sm">{{ refreshError }}</p>
      </Alert>
    </div>

    <!-- 管理控制台主体 -->
    <div v-else class="flex-1 flex flex-col min-h-0">
      <!-- 顶部紧凑工具栏：顶级 Tab + 协议子 Tab + 操作按钮，一行融合 -->
      <div class="shrink-0 border-b border-border bg-card/50 px-2 py-1.5 flex items-center gap-2">
        <!-- 顶级 Tab：Channels / Conversations -->
        <Tabs
          :model-value="consoleTab"
          class="inline-flex"
          @update:model-value="updateConsoleTab"
        >
          <TabsList class="h-7 border border-border bg-secondary/40 p-0.5">
            <TabsTrigger value="channels" class="px-2.5 py-0.5 text-xs data-[state=active]:bg-primary data-[state=active]:text-primary-foreground">
              {{ tf('app.tabs.messages', 'Channels') }}
            </TabsTrigger>
            <TabsTrigger value="conversations" class="px-2.5 py-0.5 text-xs data-[state=active]:bg-primary data-[state=active]:text-primary-foreground">
              {{ tf('app.tabs.conversations', 'Cockpit') }}
            </TabsTrigger>
          </TabsList>
        </Tabs>

        <!-- 分隔线 -->
        <div v-if="consoleTab === 'channels'" class="w-px h-4 bg-border" />

        <!-- 协议子 Tab（仅 channels 面板显示） -->
        <Tabs
          v-if="consoleTab === 'channels'"
          :model-value="activeTab"
          class="inline-flex"
          @update:model-value="updateProtocolTab"
        >
          <TabsList class="h-7 border border-border bg-secondary/40 p-0.5">
            <TabsTrigger
              v-for="tab in protocolTabs"
              :key="tab.value"
              :value="tab.value"
              class="px-2 py-0.5 text-xs data-[state=active]:bg-primary data-[state=active]:text-primary-foreground"
            >
              {{ tab.label }}
            </TabsTrigger>
          </TabsList>
        </Tabs>

        <!-- 弹性占位 -->
        <div class="flex-1" />

        <!-- 右侧操作按钮 -->
        <Button variant="outline" size="sm" class="h-7 text-xs" @click="openInBrowser">
          <Globe class="h-3 w-3" />
          {{ t('webui.openInBrowser') }}
        </Button>
      </div>

      <!-- 内容区域 -->
      <div class="flex-1 min-h-0">
        <!-- 频道管理面板 -->
        <div v-show="consoleTab === 'channels'" class="h-full">
          <ScrollArea class="h-full">
            <div class="p-3">
              <ChannelManager :type="activeTab" />
            </div>
          </ScrollArea>
        </div>

        <!-- 驾驶舱面板 -->
        <div v-show="consoleTab === 'conversations'" class="h-full">
          <ScrollArea class="h-full">
            <div class="p-3">
              <ConversationDashboard />
            </div>
          </ScrollArea>
        </div>
      </div>

      
    </div>
  </div>
</template>

<style scoped>
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.15s ease;
}
.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
</style>
