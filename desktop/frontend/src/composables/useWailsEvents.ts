import { onMounted, onBeforeUnmount, type Ref } from 'vue'
import { Events } from '@wailsio/runtime'
import type { TabValue } from '@/types'

export function useWailsEvents(
  activeTab: Ref<TabValue>,
  actionError: Ref<string>,
  syncStatus: () => Promise<void>,
) {
  let unsubscribeTab: (() => void) | undefined
  let unsubscribeTrayError: (() => void) | undefined

  onMounted(() => {
    unsubscribeTab = Events.On('desktop:show-tab', (event: { data: string }) => {
      const validTabs: TabValue[] = ['status', 'agent', 'channels', 'env', 'dashboard']
      if (validTabs.includes(event.data as TabValue)) {
        activeTab.value = event.data as TabValue
      }
    })
    unsubscribeTrayError = Events.On('desktop:tray-error', (event: { data: string }) => {
      actionError.value = event.data
      void syncStatus()
    })
  })

  onBeforeUnmount(() => {
    unsubscribeTab?.()
    unsubscribeTrayError?.()
  })
}
