<script setup lang="ts">
import { Textarea } from '@/components/ui/textarea'
import { AlertCircle, CheckCircle2, ClipboardPaste } from 'lucide-vue-next'
import { useLanguage } from '@/composables/useLanguage'

defineProps<{
  quickInput: string
  detectedBaseUrls: string[]
  detectedApiKeys: string[]
}>()

const emit = defineEmits<{
  (e: 'update:quick-input', value: string): void
  (e: 'quick-paste', text: string): void
}>()

const { tf } = useLanguage()
</script>

<template>
  <section class="space-y-3 rounded-xl border border-primary/20 bg-primary/5 p-4">
    <div>
      <h4 class="flex items-center gap-2 text-xs font-semibold uppercase tracking-wider text-primary">
        <ClipboardPaste class="h-3.5 w-3.5" />
        {{ tf('addChannel.quickMode', '快速粘贴') }}
      </h4>
      <p class="mt-1 text-xs text-muted-foreground">
        {{ tf('addChannel.quickHint', '粘贴 Base URL、API Key 或完整配置片段，自动识别并填入表单。') }}
      </p>
    </div>

    <Textarea
      :model-value="quickInput"
      rows="10"
      class="!field-sizing-none min-h-[14rem] font-mono text-xs"
      placeholder="https://api.example.com/v1&#10;sk-..."
      @update:model-value="(val) => emit('update:quick-input', val as string)"
      @paste="emit('quick-paste', $event.clipboardData?.getData('text/plain') || '')"
    />

    <div class="grid gap-2 md:grid-cols-2">
      <div class="rounded-lg border border-border bg-background/70 p-2 text-xs">
        <div class="mb-1 flex items-center gap-1.5 font-semibold">
          <CheckCircle2 v-if="detectedBaseUrls.length" class="h-3.5 w-3.5 text-emerald-500" />
          <AlertCircle v-else class="h-3.5 w-3.5 text-muted-foreground" />
          Base URLs
        </div>
        <p class="truncate text-muted-foreground">
          {{ detectedBaseUrls.length ? detectedBaseUrls.join(' · ') : tf('addChannel.noneDetected', '未识别') }}
        </p>
      </div>

      <div class="rounded-lg border border-border bg-background/70 p-2 text-xs">
        <div class="mb-1 flex items-center gap-1.5 font-semibold">
          <CheckCircle2 v-if="detectedApiKeys.length" class="h-3.5 w-3.5 text-emerald-500" />
          <AlertCircle v-else class="h-3.5 w-3.5 text-muted-foreground" />
          {{ tf('channelEditor.auth.keys.label', 'API Keys') }}
        </div>
        <p class="text-muted-foreground">
          {{ detectedApiKeys.length ? `${detectedApiKeys.length} ${tf('channelCard.configuredKeys', 'active keys')}` : tf('addChannel.noneDetected', '未识别') }}
        </p>
      </div>
    </div>
  </section>
</template>
