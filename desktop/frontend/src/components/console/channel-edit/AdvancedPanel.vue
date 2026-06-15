<script setup lang="ts">
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { useLanguage } from '@/composables/useLanguage'

interface FormData {
  fastMode: boolean
  textVerbosity: 'low' | 'medium' | 'high' | ''
  visionFallbackModel: string
  noVision: boolean
  historicalImageTurnLimit: number
  passbackReasoningContent: boolean
  passbackThinkingBlocks: boolean
  lowQuality: boolean
  injectDummyThoughtSignature: boolean
  stripThoughtSignature: boolean
  stripEmptyTextBlocks: boolean
  normalizeSystemRoleToTopLevel: boolean
  normalizeMetadataUserId: boolean
  stripBillingHeader: boolean
  normalizeNonstandardChatRoles: boolean
  autoBlacklistBalance: boolean
  codexNativeToolPassthrough: boolean
  codexToolCompat: boolean
  stripCodexClientTools: boolean
  stripImageGenerationTool: boolean
  reasoningParamStyle: string
  serviceType: string
  proxyUrl: string
  routePrefix: string
  rateLimitRpm: string | number
  rateLimitWindowMinutes: string | number
  rateLimitMaxConcurrent: string | number
  rateLimitAutoFromHeaders: boolean
  requestTimeoutMs: string | number
}

const props = defineProps<{
  form: FormData
  channelType: string
  textVerbosityOptions: Array<{ label: string; value: string }>
  supportsOpenAIAdvanced: boolean
  supportsOpenAIAdvancedOptions: boolean
  supportsChatRoleNormalization: boolean
  DEFAULT_SELECT_VALUE: string
}>()

const emit = defineEmits<{
  'update:form': [value: Partial<FormData>]
}>()

const { t, tf } = useLanguage()

function updateField<K extends keyof FormData>(key: K, value: FormData[K]) {
  emit('update:form', { [key]: value } as Partial<FormData>)
}

function toSelectValue(value: string): string {
  return value === '' ? props.DEFAULT_SELECT_VALUE : value
}

function fromSelectValue(value: string): string {
  return value === props.DEFAULT_SELECT_VALUE ? '' : value
}

</script>

<template>
  <div class="space-y-6">
    <!-- 生成参数 -->
    <section v-if="supportsOpenAIAdvanced || channelType === 'responses' || channelType === 'chat'" class="space-y-3 rounded-xl border border-border/60 bg-card/40 p-5 shadow-xs">
      <h4 class="text-xs font-bold uppercase tracking-wider text-primary border-b border-border/40 pb-2">
        {{ tf('channelEditor.compat.generationParams', '生成参数') }}
      </h4>

      <div v-if="supportsOpenAIAdvanced" class="grid gap-4 md:grid-cols-2 bg-background/30 p-3 rounded-lg border border-border/40">
        <div class="flex items-center justify-between p-2 rounded-md hover:bg-accent/40 transition-colors">
          <div class="space-y-0.5">
            <Label class="text-xs font-semibold">{{ tf('channelEditor.compat.fastMode.label', '快速模式') }}</Label>
            <p class="text-[10px] text-muted-foreground">{{ tf('channelEditor.compat.fastMode.hint', '优先选取低延迟的轻量边缘路由链路') }}</p>
          </div>
          <Switch :model-value="form.fastMode" @update:model-value="updateField('fastMode', $event)" />
        </div>

        <div class="space-y-1 p-1">
          <Label class="text-[10px] font-bold text-muted-foreground uppercase">{{ tf('channelEditor.compat.textVerbosity.style', 'Text Verbosity Style') }}</Label>
          <Select :model-value="toSelectValue(form.textVerbosity)" @update:model-value="(val) => updateField('textVerbosity', fromSelectValue(val as string) as any)">
            <SelectTrigger class="h-9 w-full">
              <SelectValue :placeholder="tf('channelEditor.compat.selectDefault', '默认')" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem v-for="opt in textVerbosityOptions" :key="opt.value" :value="opt.value">
                {{ opt.label }}
              </SelectItem>
            </SelectContent>
          </Select>
        </div>
      </div>

      <!-- Vision 控制 -->
      <div v-if="['messages', 'chat'].includes(channelType)" class="p-4 rounded-xl border border-border/50 bg-background/40 space-y-3">
        <div class="text-[10px] font-bold uppercase tracking-wider text-primary/80 border-b border-border/30 pb-1">
          {{ tf('channelEditor.compat.vision.title', '视觉控制') }}
        </div>
        <div class="space-y-3">
          <div class="flex flex-row-reverse items-center justify-between gap-3">
            <Switch :model-value="form.noVision" @update:model-value="updateField('noVision', $event)" class="shrink-0" />
            <div class="min-w-0 space-y-0.5">
              <Label class="text-xs font-medium">{{ tf('channelEditor.compat.noVision.label', '跳过含图请求') }}</Label>
              <p class="text-[10px] text-muted-foreground">
                {{ tf('channelEditor.compat.noVision.hint', '启用后，包含图片的请求将跳过此渠道并 failover 到下一个渠道') }}
              </p>
            </div>
          </div>
          <div class="space-y-1">
            <Label class="text-[10px] font-bold text-muted-foreground">
              {{ tf('channelEditor.compat.historicalImageLimit.label', '历史图片轮次限制') }}
            </Label>
            <Input
              :model-value="form.historicalImageTurnLimit"
              type="number"
              min="0"
              class="h-8 text-xs"
              placeholder="0"
              @update:model-value="updateField('historicalImageTurnLimit', Number($event))"
            />
            <p class="text-[10px] leading-4 text-muted-foreground">
              {{ tf('channelEditor.compat.historicalImageLimit.hint', '0 = 继承全局；后端会对 >0 的值应用最低 3 约束') }}
            </p>
          </div>
        </div>
      </div>
    </section>

    <!-- 高级选项 -->
    <section class="space-y-6 rounded-xl border border-border/60 bg-card/40 p-4 shadow-xs">
      <h4 class="text-xs font-bold uppercase tracking-wider text-primary border-b border-border/40 pb-2">
        {{ tf('channelEditor.nav.advanced', '高级选项') }}
      </h4>

      <div class="space-y-2.5">
        <!-- Runtime 运行期策略 -->
        <div class="p-4 rounded-xl border border-border/50 bg-background/40 space-y-2.5">
        <div class="text-[10px] font-bold uppercase tracking-wider text-primary/80 border-b border-border/30 pb-1">
          {{ t('channelEditor.runtime.title') }}
        </div>
        <div class="space-y-2">
          <div class="flex items-center justify-between">
            <Label class="text-xs font-medium">{{ t('channelEditor.runtime.autoBlacklist.label') }}</Label>
            <Switch :model-value="form.autoBlacklistBalance" @update:model-value="updateField('autoBlacklistBalance', $event)" />
          </div>
          <div class="flex items-center justify-between">
            <div class="space-y-0.5">
              <Label class="text-xs font-medium">{{ t('channelEditor.runtime.autoLearnRateLimits.label') }}</Label>
              <p class="text-[10px] leading-4 text-muted-foreground">
                {{ t('channelEditor.runtime.autoLearnRateLimits.hint') }}
              </p>
            </div>
            <Switch :model-value="form.rateLimitAutoFromHeaders" @update:model-value="updateField('rateLimitAutoFromHeaders', $event)" />
          </div>
        </div>
      </div>

      <!-- 协议规范化 -->
      <div class="p-4 rounded-xl border border-border/50 bg-background/40 space-y-2.5">
      <div class="text-[10px] font-bold uppercase tracking-wider text-primary/80 border-b border-border/30 pb-1">
        {{ t('channelEditor.compat.title') }}
      </div>
      <div class="space-y-2">
        <div v-if="channelType === 'responses'" class="flex items-center justify-between">
          <Label class="text-xs font-medium">{{ t('channelEditor.compat.codexNativeTools.label') }}</Label>
          <Switch :model-value="form.codexNativeToolPassthrough" @update:model-value="updateField('codexNativeToolPassthrough', $event)" />
        </div>
        <div v-if="channelType === 'responses'" class="flex items-center justify-between">
          <Label class="text-xs font-medium">{{ t('channelEditor.compat.codexCompat.label') }}</Label>
          <Switch :model-value="form.codexToolCompat" @update:model-value="updateField('codexToolCompat', $event)" />
        </div>
        <div v-if="channelType === 'responses' || channelType === 'chat'" class="flex items-center justify-between">
          <Label class="text-xs font-medium">{{ t('channelEditor.compat.stripImageGen.label') }}</Label>
          <Switch :model-value="form.stripImageGenerationTool" @update:model-value="updateField('stripImageGenerationTool', $event)" />
        </div>
        <div v-if="channelType === 'messages'" class="flex items-center justify-between">
          <Label class="text-xs font-medium">{{ t('channelEditor.compat.normalizeSystem.label') }}</Label>
          <Switch :model-value="form.normalizeSystemRoleToTopLevel" @update:model-value="updateField('normalizeSystemRoleToTopLevel', $event)" />
        </div>
        <div v-if="['messages','responses'].includes(channelType)" class="flex items-center justify-between">
          <Label class="text-xs font-medium">{{ t('channelEditor.compat.normalizeUserId.label') }}</Label>
          <Switch :model-value="form.normalizeMetadataUserId" @update:model-value="updateField('normalizeMetadataUserId', $event)" />
        </div>
        <div v-if="channelType === 'messages'" class="flex items-center justify-between">
          <Label class="text-xs font-medium">{{ t('channelEditor.compat.stripBillingHeader.label') }}</Label>
          <Switch :model-value="form.stripBillingHeader" @update:model-value="updateField('stripBillingHeader', $event)" />
        </div>
        <div v-if="supportsChatRoleNormalization" class="flex items-center justify-between">
          <Label class="text-xs font-medium">{{ t('channelEditor.compat.normalizeRoles.label') }}</Label>
          <Switch :model-value="form.normalizeNonstandardChatRoles" @update:model-value="updateField('normalizeNonstandardChatRoles', $event)" />
        </div>
        <div v-if="supportsOpenAIAdvancedOptions" class="flex items-center justify-between">
          <div class="flex-1">
            <Label class="text-xs font-medium">{{ t('channelEditor.compat.reasoningStyle.label') }}</Label>
          </div>
          <Select
            :model-value="form.reasoningParamStyle || 'default'"
            @update:model-value="(val) => updateField('reasoningParamStyle', val === 'default' ? '' : String(val))"
          >
            <SelectTrigger class="h-8 w-[140px] text-xs">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="default">{{ t('channelEditor.compat.selectDefault') }}</SelectItem>
              <SelectItem value="reasoning_effort">reasoning_effort</SelectItem>
              <SelectItem value="developer_message">developer_message</SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div v-if="(channelType === 'gemini' || channelType === 'messages') && form.serviceType === 'gemini'" class="flex items-center justify-between">
          <Label class="text-xs font-medium">{{ t('channelEditor.compat.injectDummySignature.label') }}</Label>
          <Switch :model-value="form.injectDummyThoughtSignature" @update:model-value="updateField('injectDummyThoughtSignature', $event)" />
        </div>
        <div v-if="form.serviceType === 'gemini'" class="flex items-center justify-between">
          <Label class="text-xs font-medium">{{ t('channelEditor.compat.stripThoughtSignature.label') }}</Label>
          <Switch :model-value="form.stripThoughtSignature" @update:model-value="updateField('stripThoughtSignature', $event)" />
        </div>
        <div v-if="(channelType === 'messages' || channelType === 'chat' || channelType === 'responses') && form.serviceType === 'claude'" class="flex items-center justify-between">
          <Label class="text-xs font-medium">{{ t('channelEditor.compat.passbackReasoning.label') }}</Label>
          <Switch :model-value="form.passbackReasoningContent" @update:model-value="updateField('passbackReasoningContent', $event)" />
        </div>
        <div v-if="(channelType === 'messages' || channelType === 'chat' || channelType === 'responses') && form.serviceType === 'claude'" class="flex items-center justify-between">
          <Label class="text-xs font-medium">{{ t('channelEditor.compat.passbackThinking.label') }}</Label>
          <Switch :model-value="form.passbackThinkingBlocks" @update:model-value="updateField('passbackThinkingBlocks', $event)" />
        </div>
        <div v-if="channelType === 'messages' && form.serviceType === 'claude'" class="flex items-center justify-between">
          <Label class="text-xs font-medium">{{ t('channelEditor.compat.stripEmptyBlocks.label') }}</Label>
          <Switch :model-value="form.stripEmptyTextBlocks" @update:model-value="updateField('stripEmptyTextBlocks', $event)" />
        </div>
      </div>
    </div>
      </div>

      <!-- Transport 代理路由网络 -->
      <div class="p-4 rounded-xl border border-border/50 bg-background/40 space-y-3">
        <div class="text-[10px] font-bold uppercase tracking-wider text-primary/80 border-b border-border/30 pb-1">
          {{ t('channelEditor.transport.title') }}
        </div>
        <div class="grid grid-cols-3 gap-2">
          <div class="space-y-1">
            <Label class="text-[9px] font-bold text-muted-foreground">{{ t('channelEditor.transport.proxyUrl.label') }}</Label>
            <Input
              :model-value="form.proxyUrl"
              class="h-8 w-full font-mono text-xs"
              placeholder="socks5://..."
              @update:model-value="(val) => updateField('proxyUrl', val as string)"
            />
          </div>
          <div class="space-y-1">
            <Label class="text-[9px] font-bold text-muted-foreground">{{ t('channelEditor.transport.requestTimeout.label') }}</Label>
            <Input
              :model-value="form.requestTimeoutMs"
              type="number"
              class="h-8 w-full text-xs"
              placeholder="60000"
              @update:model-value="(val) => updateField('requestTimeoutMs', val)"
            />
          </div>
          <div class="space-y-1">
            <Label class="text-[9px] font-bold text-muted-foreground">{{ t('channelEditor.transport.routePrefix.label') }}</Label>
            <Input
              :model-value="form.routePrefix"
              class="h-8 w-full font-mono text-xs"
              placeholder="kimi"
              @update:model-value="(val) => updateField('routePrefix', val as string)"
            />
          </div>
        </div>
      </div>

      <!-- Rate Limit -->
      <div class="p-4 rounded-xl border border-border/50 bg-background/40 space-y-3">
        <div class="text-[10px] font-bold uppercase tracking-wider text-muted-foreground">
          {{ t('channelEditor.rateLimit.title') }}
        </div>
        <div class="grid grid-cols-3 gap-3">
          <div class="space-y-1">
            <Label class="text-[10px] font-medium text-muted-foreground/80">{{ t('channelEditor.rateLimit.rpm.label') }}</Label>
            <Input
              :model-value="form.rateLimitRpm"
              type="number"
              class="h-9 text-xs"
              :placeholder="t('channelEditor.rateLimit.rpm.placeholder')"
              @update:model-value="updateField('rateLimitRpm', $event)"
            />
          </div>
          <div class="space-y-1">
            <Label class="text-[10px] font-medium text-muted-foreground/80">{{ t('channelEditor.rateLimit.window.label') }}</Label>
            <Input
              :model-value="form.rateLimitWindowMinutes"
              type="number"
              class="h-9 text-xs"
              :placeholder="t('channelEditor.rateLimit.window.placeholder')"
              @update:model-value="updateField('rateLimitWindowMinutes', $event)"
            />
          </div>
          <div class="space-y-1">
            <Label class="text-[10px] font-medium text-muted-foreground/80">{{ t('channelEditor.rateLimit.maxConcurrent.label') }}</Label>
            <Input
              :model-value="form.rateLimitMaxConcurrent"
              type="number"
              class="h-9 text-xs"
              :placeholder="t('channelEditor.rateLimit.maxConcurrent.placeholder')"
              @update:model-value="updateField('rateLimitMaxConcurrent', $event)"
            />
          </div>
        </div>
      </div>

    </section>
  </div>
</template>
