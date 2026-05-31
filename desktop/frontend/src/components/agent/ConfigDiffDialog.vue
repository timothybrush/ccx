<script setup lang="ts">
import { computed, ref, watch, onBeforeUnmount } from 'vue'
import { Button } from '@/components/ui/button'
import { AlertTriangle } from 'lucide-vue-next'
import type { ConfigDiffResult, DiffLine, FileDiff } from '@/types'
import { useLanguage } from '@/composables/useLanguage'

const CONTEXT_THRESHOLD = 4
const CONTEXT_KEEP = 2

const { t } = useLanguage()

type DisplayLine =
  | { kind: 'line'; line: DiffLine; origIndex: number }
  | { kind: 'collapsed'; id: string; lines: DiffLine[]; startOrigIndex: number }

const props = defineProps<{
  open: boolean
  mode: 'apply' | 'restore'
  platform: string
  result: ConfigDiffResult | null
  loading: boolean
  warning?: string
}>()

const emit = defineEmits<{
  confirm: []
  cancel: []
}>()

// Keyboard shortcuts: Esc 取消，Enter 确认
const handleKeydown = (e: KeyboardEvent) => {
  if (!props.open) return
  if (e.key === 'Escape') {
    e.preventDefault()
    emit('cancel')
  } else if (e.key === 'Enter' && !props.loading) {
    e.preventDefault()
    emit('confirm')
  }
}

watch(() => props.open, (isOpen) => {
  if (isOpen) {
    window.addEventListener('keydown', handleKeydown)
  } else {
    window.removeEventListener('keydown', handleKeydown)
  }
})

onBeforeUnmount(() => {
  window.removeEventListener('keydown', handleKeydown)
})

const title = computed(() =>
  props.mode === 'apply' ? t('agent.diffPreviewApply') : t('agent.diffPreviewRestore')
)

const confirmLabel = computed(() =>
  props.mode === 'apply' ? t('agent.diffConfirmApply') : t('agent.diffConfirmRestore')
)

const platformLabel = computed(() =>
  props.platform === 'claude' ? 'Claude Code' : props.platform === 'opencode' ? 'OpenCode' : 'Codex'
)

// 统计文件增删行数（用于替代易被误认为可点击按钮的状态标签）
const lineStats = (file: FileDiff) => {
  let added = 0
  let removed = 0
  for (const line of file.lines) {
    if (line.type === 'added') added++
    else if (line.type === 'removed') removed++
  }
  return { added, removed }
}

// --- Context folding ---

const collapsedSections = ref(new Set<string>())

function toggleCollapse(id: string) {
  if (collapsedSections.value.has(id)) {
    collapsedSections.value.delete(id)
  } else {
    collapsedSections.value.add(id)
  }
}

function isExpanded(id: string): boolean {
  return collapsedSections.value.has(id)
}

function collapseContextLines(file: FileDiff, fileIndex: number): DisplayLine[] {
  const result: DisplayLine[] = []
  let runStart = -1

  const flushRun = (end: number) => {
    const run = file.lines.slice(runStart, end)
    if (run.length <= CONTEXT_THRESHOLD) {
      run.forEach((line, i) => result.push({ kind: 'line', line, origIndex: runStart + i }))
    } else {
      const hidden = run.slice(CONTEXT_KEEP, run.length - CONTEXT_KEEP)
      const id = `${fileIndex}-c-${runStart}`
      // head
      for (let i = 0; i < CONTEXT_KEEP; i++) {
        result.push({ kind: 'line', line: run[i], origIndex: runStart + i })
      }
      // collapsed marker
      result.push({ kind: 'collapsed', id, lines: hidden, startOrigIndex: runStart + CONTEXT_KEEP })
      // tail
      for (let i = run.length - CONTEXT_KEEP; i < run.length; i++) {
        result.push({ kind: 'line', line: run[i], origIndex: runStart + i })
      }
    }
  }

  for (let i = 0; i < file.lines.length; i++) {
    if (file.lines[i].type === 'context') {
      if (runStart === -1) runStart = i
    } else {
      if (runStart !== -1) {
        flushRun(i)
        runStart = -1
      }
      result.push({ kind: 'line', line: file.lines[i], origIndex: i })
    }
  }
  if (runStart !== -1) flushRun(file.lines.length)

  return result
}

const processedFiles = computed(() => {
  if (!props.result) return []
  return props.result.files.map((file, fi) => ({
    file,
    displayLines: collapseContextLines(file, fi),
  }))
})
</script>

<template>
  <Teleport to="body">
    <Transition name="fade">
      <div
        v-if="open"
        class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm"
        @click.self="emit('cancel')"
      >
        <div
          class="w-[min(720px,90vw)] max-h-[85vh] overflow-hidden rounded-2xl border border-border bg-card shadow-2xl flex flex-col"
        >
          <!-- Header -->
          <div class="border-b border-border px-6 py-4 shrink-0">
            <div class="flex items-baseline justify-between">
              <h2 class="text-lg font-semibold text-foreground">{{ title }}</h2>
              <div class="text-xs text-muted-foreground">{{ platformLabel }}</div>
            </div>
          </div>

          <!-- Warning banner -->
          <div v-if="warning" class="mx-6 mt-3 flex items-start gap-2 rounded-lg border border-yellow-500/30 bg-yellow-500/10 px-3 py-2 text-xs text-yellow-700 dark:text-yellow-300">
            <AlertTriangle class="h-4 w-4 shrink-0 mt-0.5" />
            <p>{{ warning }}</p>
          </div>

          <!-- Content -->
          <div class="flex-1 overflow-y-auto px-6 py-4 space-y-4 min-h-0">
            <!-- Loading state -->
            <div v-if="loading" class="flex items-center justify-center py-12">
              <div class="text-sm text-muted-foreground">{{ t('agent.diffComputing') }}</div>
            </div>

            <!-- No changes -->
            <div v-else-if="!result || result.files.length === 0" class="flex items-center justify-center py-12">
              <div class="text-sm text-muted-foreground">{{ t('agent.diffNoChanges') }}</div>
            </div>

            <!-- Diff blocks -->
            <template v-else>
              <div
                v-for="{ file, displayLines } in processedFiles"
                :key="file.path"
                class="rounded-lg border border-border overflow-hidden"
              >
                <!-- File header -->
                <div class="flex items-center justify-between gap-3 px-4 py-2 bg-secondary/50 border-b border-border">
                  <code class="min-w-0 flex-1 text-xs text-foreground break-all">{{ file.path }}</code>
                  <div class="flex shrink-0 items-center gap-2 font-mono text-[11px] tabular-nums whitespace-nowrap">
                    <span v-if="lineStats(file).added > 0" class="text-emerald-600 dark:text-emerald-400">+{{ lineStats(file).added }}</span>
                    <span v-if="lineStats(file).removed > 0" class="text-red-600 dark:text-red-400">-{{ lineStats(file).removed }}</span>
                  </div>
                </div>

                <!-- Diff lines -->
                <div class="overflow-x-auto">
                  <table class="w-full text-xs font-mono">
                    <tbody>
                      <template v-for="item in displayLines" :key="item.kind === 'collapsed' ? item.id : item.origIndex">
                        <!-- Normal diff line -->
                        <tr
                          v-if="item.kind === 'line'"
                          :class="{
                            'bg-emerald-500/[0.07]': item.line.type === 'added',
                            'bg-red-500/[0.07]': item.line.type === 'removed',
                          }"
                        >
                          <td class="w-8 text-right pr-2 py-0.5 select-none text-muted/50 align-top">
                            {{ item.origIndex + 1 }}
                          </td>
                          <td class="w-4 text-center py-0.5 select-none align-top"
                            :class="{
                              'text-emerald-700 dark:text-emerald-400': item.line.type === 'added',
                              'text-red-700 dark:text-red-400': item.line.type === 'removed',
                              'text-muted/50': item.line.type === 'context',
                            }"
                          >
                            {{ item.line.type === 'added' ? '+' : item.line.type === 'removed' ? '-' : ' ' }}
                          </td>
                          <td class="py-0.5 pr-4 whitespace-pre-wrap break-all"
                            :class="{
                              'text-emerald-700/80 dark:text-emerald-300/80': item.line.type === 'added',
                              'text-red-700/80 dark:text-red-300/80': item.line.type === 'removed',
                              'text-muted/60': item.line.type === 'context',
                            }"
                          >
                            {{ item.line.content || ' ' }}
                          </td>
                        </tr>

                        <!-- Collapsed marker (not yet expanded) -->
                        <tr v-else-if="!isExpanded(item.id)" class="group">
                          <td colspan="3" class="py-1 px-4">
                            <button
                              class="w-full flex items-center justify-center gap-1.5 py-1 rounded text-muted-foreground hover:text-foreground hover:bg-secondary transition-colors cursor-pointer"
                              @click="toggleCollapse(item.id)"
                            >
                              <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                                <path stroke-linecap="round" stroke-linejoin="round" d="M4 8h16M4 16h16" />
                              </svg>
                              <span>{{ t('agent.diffExpandContext', { count: String(item.lines.length) }) }}</span>
                            </button>
                          </td>
                        </tr>

                        <!-- Expanded hidden lines -->
                        <template v-else>
                          <tr class="group">
                            <td colspan="3" class="py-1 px-4">
                              <button
                                class="w-full flex items-center justify-center gap-1.5 py-1 rounded text-muted-foreground hover:text-foreground hover:bg-secondary transition-colors cursor-pointer"
                                @click="toggleCollapse(item.id)"
                              >
                                <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                                  <path stroke-linecap="round" stroke-linejoin="round" d="M5 15l7-7 7 7" />
                                </svg>
                                <span>{{ t('agent.diffCollapseContext', { count: String(item.lines.length) }) }}</span>
                              </button>
                            </td>
                          </tr>
                          <tr
                            v-for="(hidden, hi) in item.lines"
                            :key="`${item.id}-${hi}`"
                          >
                            <td class="w-8 text-right pr-2 py-0.5 select-none text-muted/50 align-top">
                              {{ item.startOrigIndex + hi + 1 }}
                            </td>
                            <td class="w-4 text-center py-0.5 select-none text-muted/50 align-top"> </td>
                            <td class="py-0.5 pr-4 whitespace-pre-wrap break-all text-muted/60">
                              {{ hidden.content || ' ' }}
                            </td>
                          </tr>
                        </template>
                      </template>
                    </tbody>
                  </table>
                </div>
              </div>
            </template>
          </div>

          <!-- Footer -->
          <div class="flex justify-end gap-2 border-t border-border px-6 py-4 shrink-0">
            <Button variant="ghost" size="sm" @click="emit('cancel')">
              {{ t('agent.diffCancel') }} <span class="ml-1.5 text-xs opacity-60">Esc</span>
            </Button>
            <Button size="sm" :disabled="loading" @click="emit('confirm')">
              {{ confirmLabel }} <span class="ml-1.5 text-xs opacity-60">Enter</span>
            </Button>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.18s ease;
}
.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
</style>
