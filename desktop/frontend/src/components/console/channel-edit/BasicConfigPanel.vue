<script setup lang="ts">
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { useLanguage } from '@/composables/useLanguage'

interface FormData {
  name: string
  description: string
  serviceType: 'openai' | 'claude' | 'gemini' | 'responses' | ''
  baseUrl: string
  baseUrlsText: string
  website: string
}

interface Errors {
  name?: string
  serviceType?: string
  baseUrl?: string
}

const props = defineProps<{
  form: FormData
  errors: Errors
  serviceTypeOptions: Array<{ label: string; value: string }>
  expectedRequestUrls: any[]
}>()

const emit = defineEmits<{
  'update:form': [value: Partial<FormData>]
}>()

const { t, tf } = useLanguage()

function updateField<K extends keyof FormData>(key: K, value: FormData[K]) {
  emit('update:form', { [key]: value } as Partial<FormData>)
}
</script>

<template>
  <div class="grid gap-4 lg:grid-cols-2">
    <!-- 基础信息 -->
    <section class="space-y-4 rounded-xl border border-border/60 bg-card/40 p-5 shadow-xs">
      <h4 class="text-xs font-bold uppercase tracking-wider text-primary">{{ t('channelEditor.nav.basic') }}</h4>
      <div class="grid grid-cols-[2fr_1fr] gap-3">
        <div class="space-y-1.5">
          <Label class="text-xs font-semibold text-muted-foreground">
            {{ t('channelEditor.basic.name.label') }} <span class="text-destructive">*</span>
          </Label>
          <Input
            :model-value="form.name"
            class="h-9"
            :class="{ 'border-destructive': errors.name }"
            @update:model-value="(val) => updateField('name', val as string)"
          />
          <p v-if="errors.name" class="text-[10px] text-destructive">{{ errors.name }}</p>
        </div>
        <div class="space-y-1.5">
          <Label class="text-xs font-semibold text-muted-foreground">
            {{ t('channelEditor.basic.serviceType.label') }} <span class="text-destructive">*</span>
          </Label>
          <Select :model-value="form.serviceType" @update:model-value="updateField('serviceType', $event as any)">
            <SelectTrigger class="h-9" :class="{ 'border-destructive': errors.serviceType }">
              <SelectValue :placeholder="tf('channelEditor.basic.serviceType.placeholder', '选择服务类型')" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem v-for="opt in serviceTypeOptions" :key="opt.value" :value="opt.value">
                {{ opt.label }}
              </SelectItem>
            </SelectContent>
          </Select>
          <p v-if="errors.serviceType" class="text-[10px] text-destructive">{{ errors.serviceType }}</p>
        </div>
      </div>
      <div class="space-y-1.5">
        <Label class="text-xs font-semibold text-muted-foreground">{{ t('channelEditor.basic.description.label') }}</Label>
        <Textarea
          :model-value="form.description"
          rows="2"
          class="min-h-[74px] resize-none"
          :placeholder="t('channelEditor.basic.description.placeholder')"
          @update:model-value="(val) => updateField('description', val as string)"
        />
      </div>
    </section>

    <!-- 连接终点 -->
    <section class="space-y-4 rounded-xl border border-border/60 bg-card/40 p-5 shadow-xs">
      <h4 class="text-xs font-bold uppercase tracking-wider text-primary">{{ t('channelEditor.basic.baseUrl.label') }}</h4>
      <div class="space-y-1.5">
        <div class="flex items-center justify-between">
          <Label class="text-xs font-semibold text-muted-foreground">
            Base URL <span class="text-destructive">*</span>
          </Label>
          <span class="origin-right scale-95 text-[10px] text-muted-foreground/80">{{ tf('channelEditor.basic.multiLineFailover', '多行实现故障轮换') }}</span>
        </div>
        <Textarea
          :model-value="form.baseUrlsText"
          placeholder="https://api.example.com&#10;https://backup.example.com"
          class="min-h-[74px] w-full rounded-lg border border-input bg-background/50 px-3 py-2 font-mono text-xs leading-relaxed shadow-inner outline-none transition-all focus:border-primary focus:ring-2 focus:ring-primary/20"
          :class="{ 'border-destructive': errors.baseUrl }"
          @update:model-value="(val) => updateField('baseUrlsText', val as string)"
        />
        <div class="flex items-center gap-1.5 rounded-md border border-border/30 bg-accent/40 px-2 py-1 text-[10px] text-muted-foreground/70">
          <span class="inline-block size-1.5 animate-pulse rounded-full bg-emerald-500"></span>
          <span class="truncate font-mono">
            {{ tf('channelEditor.basic.expectedEndpoint', '预期终点:') }} {{ expectedRequestUrls[0]?.expectedUrl || 'N/A' }}
          </span>
        </div>
        <p v-if="errors.baseUrl" class="text-[10px] text-destructive">{{ errors.baseUrl }}</p>
      </div>
      <div class="space-y-1.5">
        <Label class="text-xs font-semibold text-muted-foreground">{{ t('channelEditor.basic.website.label') }}</Label>
        <Input
          :model-value="form.website"
          class="h-9"
          placeholder="https://example.com"
          @update:model-value="(val) => updateField('website', val as string)"
        />
      </div>
    </section>
  </div>
</template>
