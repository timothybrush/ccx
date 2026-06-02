<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Button } from '@/components/ui/button'
import { Alert } from '@/components/ui/alert'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Globe, Sparkles, X } from 'lucide-vue-next'
import { useStatus } from '@/composables/useStatus'
import { useLanguage } from '@/composables/useLanguage'
import { useConsoleChannels } from '@/composables/useConsoleChannels'
import { OpenWebUIInBrowser } from '@bindings/github.com/BenedictKing/ccx/desktop/desktopservice'
import ChannelManager from '@/components/console/ChannelManager.vue'
import ChannelTab from '@/components/channel/ChannelTab.vue'
import ConversationDashboard from '@/components/console/ConversationDashboard.vue'
import type { ManagedChannelType } from '@/utils/channel-type-api'

const props = withDefaults(defineProps<{
  initialSection?: 'channels' | 'conversations'
}>(), {
  initialSection: 'channels',
})

const { status } = useStatus()
const { t, tf } = useLanguage()
const { activeTab, refreshChannels, refreshError } = useConsoleChannels()

// 子 Tab 定义
const protocolTabs: { value: ManagedChannelType; label: string }[] = [
  { value: 'messages', label: 'Messages' },
  { value: 'chat', label: 'Chat' },
  { value: 'responses', label: 'Responses' },
  { value: 'gemini', label: 'Gemini' },
  { value: 'images', label: 'Images' },
]

// 管理控制台的顶级 tab：频道管理 vs 会话管理
const consoleTab = ref<'channels' | 'conversations'>(props.initialSection)
const showQuickAdd = ref(false)

const isManagedChannelType = (value: string): value is ManagedChannelType => {
  return protocolTabs.some(tab => tab.value === value)
}

const handlePresetCreated = async (target: string) => {
  if (isManagedChannelType(target)) {
    activeTab.value = target
  }
  showQuickAdd.value = false
  await refreshChannels()
}

const openInBrowser = async () => {
  try {
    await OpenWebUIInBrowser()
  } catch (e) {
    console.warn('Failed to open WebUI in browser:', e)
  }
}

onMounted(() => {
  if (status.value.running) {
    refreshChannels()
  }
})

watch(() => props.initialSection, (section) => {
  consoleTab.value = section
})

watch(() => status.value.running, (running) => {
  if (running) refreshChannels()
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
      <!-- 顶级 Tab：频道管理 / 会话管理 -->
      <Tabs v-model="consoleTab" class="flex-1 flex flex-col min-h-0">
        <div v-if="initialSection !== 'channels'" class="flex items-center justify-between shrink-0 mb-4">
          <TabsList>
            <TabsTrigger value="channels">
              {{ tf('console.channelsTab', 'Channels') }}
            </TabsTrigger>
            <TabsTrigger value="conversations">
              {{ tf('console.conversationsTab', 'Conversations') }}
            </TabsTrigger>
          </TabsList>

          <div class="flex items-center gap-2">
            <Button variant="outline" size="sm" @click="openInBrowser">
              <Globe class="w-3.5 h-3.5 mr-1.5" />
              {{ t('webui.openInBrowser') }}
            </Button>
          </div>
        </div>

        <!-- 频道管理面板 -->
        <TabsContent value="channels" class="flex-1 min-h-0 mt-0">
          <div class="h-full flex flex-col">
            <div class="mb-3 flex items-center justify-between border border-border bg-card/70 px-3 py-2">
              <div>
                <div class="text-xs font-bold uppercase tracking-[0.18em] text-muted-foreground">
                  {{ tf('console.nativeChannelsEyebrow', 'Native channel orchestration') }}
                </div>
                <div class="text-sm font-semibold text-foreground">
                  {{ tf('console.nativeChannelsTitle', 'WebUI-aligned protocol workspace') }}
                </div>
              </div>
              <div class="flex items-center gap-2">
                <Button variant="outline" size="sm" @click="showQuickAdd = true">
                  <Sparkles class="h-3.5 w-3.5" />
                  {{ tf('console.quickAdd', 'Quick Add') }}
                </Button>
                <Button variant="outline" size="sm" @click="openInBrowser">
                  <Globe class="h-3.5 w-3.5" />
                  {{ t('webui.openInBrowser') }}
                </Button>
              </div>
            </div>
            <!-- 协议子 Tab -->
            <Tabs
              :model-value="activeTab"
              @update:model-value="(v: string | number) => { activeTab = String(v) as ManagedChannelType }"
              class="flex-1 flex flex-col min-h-0"
            >
              <TabsList class="shrink-0 mb-3 border border-border bg-card/80 p-1 rounded-none">
                <TabsTrigger
                  v-for="tab in protocolTabs"
                  :key="tab.value"
                  :value="tab.value"
                >
                  {{ tab.label }}
                </TabsTrigger>
              </TabsList>

              <div class="flex-1 min-h-0">
                <ScrollArea class="h-full">
                  <div class="pr-4">
                    <ChannelManager :type="activeTab" />
                  </div>
                </ScrollArea>
              </div>
            </Tabs>
          </div>
        </TabsContent>

        <!-- 会话管理面板 -->
        <TabsContent value="conversations" class="flex-1 min-h-0 mt-0">
          <ScrollArea class="h-full">
            <div class="pr-4">
              <ConversationDashboard />
            </div>
          </ScrollArea>
        </TabsContent>
      </Tabs>

      <Teleport to="body">
        <Transition name="fade">
          <div v-if="showQuickAdd" class="fixed inset-0 z-50 flex justify-end">
            <div class="absolute inset-0 bg-black/50 backdrop-blur-sm" @click="showQuickAdd = false" />
            <aside class="relative z-10 h-full w-[min(980px,92vw)] overflow-y-auto border-l border-border bg-background p-5 shadow-2xl">
              <div class="mb-4 flex items-center justify-between border-b border-border pb-3">
                <div>
                  <div class="text-xs font-bold uppercase tracking-[0.18em] text-primary">
                    {{ tf('console.quickAddEyebrow', 'Provider Presets') }}
                  </div>
                  <h3 class="text-lg font-bold text-foreground">{{ tf('console.quickAddTitle', 'Quick Add Channel') }}</h3>
                </div>
                <Button variant="ghost" size="icon-sm" @click="showQuickAdd = false">
                  <X class="h-4 w-4" />
                </Button>
              </div>
              <ChannelTab @created="handlePresetCreated" />
            </aside>
          </div>
        </Transition>
      </Teleport>
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
