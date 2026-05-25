<script setup lang="ts">
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { FolderOpen } from 'lucide-vue-next'
import { OpenDirectory } from '@bindings/github.com/BenedictKing/ccx/desktop/desktopservice'
import type { DesktopStatus } from '@/types'

defineProps<{
  status: DesktopStatus
}>()

const openDir = (path: string) => {
  OpenDirectory(path).catch(() => {})
}
</script>

<template>
  <Card>
    <CardHeader class="pb-3">
      <CardTitle class="text-sm font-medium text-muted-foreground">服务详情</CardTitle>
    </CardHeader>
    <CardContent class="space-y-3">
      <div v-for="item in [
        { label: '二进制', value: status.binaryPath || '未发现', action: status.binaryPath ? 'reveal' : null, actionPath: status.binaryPath },
        { label: '数据目录', value: status.dataDir || '未设置', action: status.dataDir ? 'open' : null, actionPath: status.dataDir },
        { label: 'PID', value: String(status.pid || '-'), action: null },
        { label: '健康状态', value: status.health?.status || 'unknown', action: null },
      ]" :key="item.label" class="grid grid-cols-[5rem_minmax(0,1fr)] items-center gap-3 text-sm">
        <span class="text-muted-foreground">{{ item.label }}</span>
        <div class="flex min-w-0 items-center justify-end gap-2">
          <code
            class="inline-block min-w-0 max-w-full rounded-md bg-secondary px-2 py-1 text-right text-xs"
            :class="item.action ? 'break-all' : 'whitespace-nowrap'"
          >{{ item.value }}</code>
          <Button
            v-if="item.action"
            variant="ghost"
            size="icon-sm"
            :title="item.action === 'reveal' ? '打开所在目录' : '打开目录'"
            class="shrink-0"
            @click="openDir(item.actionPath!)"
          >
            <FolderOpen class="w-3.5 h-3.5" />
          </Button>
        </div>
      </div>
    </CardContent>
  </Card>
</template>
