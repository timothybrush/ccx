<script setup lang="ts">
import { Button } from '@/components/ui/button'
import { Eye, EyeOff, X, Zap } from 'lucide-vue-next'
import { useLanguage } from '@/composables/useLanguage'

defineProps<{
  channelType: string
  isEditMode: boolean
  noVision: boolean
  saving: boolean
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'toggle-no-vision'): void
  (e: 'test-capability'): void
}>()

const { tf } = useLanguage()
</script>

<template>
  <div class="flex shrink-0 items-start justify-between gap-3 border-b border-border/60 bg-card/50 p-5 backdrop-blur-sm">
    <div class="min-w-0 space-y-1">
      <div class="text-[10px] font-bold uppercase tracking-[0.2em] text-primary/80">
        {{ channelType }} CHANNEL
      </div>
      <h3 class="text-xl font-bold tracking-tight">
        {{ isEditMode
          ? tf('channelEditor.title.edit', '编辑渠道')
          : tf('channelEditor.title.create', '添加渠道')
        }}
      </h3>
    </div>

    <div class="flex shrink-0 items-center gap-1.5">
      <template v-if="isEditMode">
        <Button
          variant="ghost"
          size="icon-sm"
          class="h-8 w-8 rounded-full text-muted-foreground transition-all hover:bg-primary/10 hover:text-primary"
          :title="noVision ? tf('channelEditor.compat.visionDisabled', '视觉已禁用') : tf('channelEditor.compat.visionEnabled', '视觉已启用')"
          @click="emit('toggle-no-vision')"
        >
          <EyeOff v-if="noVision" class="h-3.5 w-3.5 text-amber-500" />
          <Eye v-else class="h-3.5 w-3.5" />
        </Button>
        <Button
          v-if="channelType !== 'images'"
          variant="outline"
          size="sm"
          class="h-8 rounded-full border border-border/80 bg-background/50 px-3.5 shadow-sm hover:bg-accent"
          :disabled="saving"
          @click="emit('test-capability')"
        >
          <Zap class="mr-1 h-3.5 w-3.5 fill-amber-500/20 text-amber-500" />
          {{ tf('capability.startTest', '能力测试') }}
        </Button>
      </template>

      <Button
        variant="ghost"
        size="icon-sm"
        class="h-8 w-8 shrink-0 rounded-md transition-colors hover:bg-destructive/10 hover:text-destructive"
        @click="emit('close')"
      >
        <X class="h-4 w-4" />
      </Button>
    </div>
  </div>
</template>
