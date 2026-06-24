<template>
  <section class="space-y-3">
    <div class="server-note">
      <strong>设备强制校验只在服务端生效。</strong>
      <span>客户端只上报设备信息，是否拦截由后端发布后的配置决定。</span>
    </div>

    <div class="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
      <label
        v-for="flag in visibleFlags"
        :key="flag"
        class="flag-row"
        :class="{ 'server-only': isServerOnly(flag) }"
      >
        <span>
          <span class="flag-title">
            <strong>{{ flagLabel(flag) }}</strong>
            <em v-if="isServerOnly(flag)">仅服务端</em>
          </span>
          <small>{{ descriptions[flag] || '客户端功能开关' }}</small>
        </span>
        <input v-model="model[flag]" type="checkbox" />
      </label>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{
  model: Record<string, boolean>
  flags: string[]
}>()

const serverOnlyFlags = new Set(['strict_device_enforcement'])
const visibleFlags = computed(() => Array.from(new Set([...props.flags, 'strict_device_enforcement'])))

const descriptions: Record<string, string> = {
  advanced_provider_config: '允许客户端显示高级供应商设置。',
  install_assistant: '开启安装和首次配置向导。',
  new_user_tutorial: '给新用户显示首次使用引导。',
  model_selector: '允许用户选择可用模型。',
  diagnostic_export: '允许用户导出诊断信息。',
  announcements: '在客户端显示 Codex++ 公告。',
  force_update_prompt: '需要升级时提示用户更新。',
  strict_device_enforcement: '开启后，没有合法设备信息的请求会被服务端拒绝。'
}

const labels: Record<string, string> = {
  advanced_provider_config: '高级供应商设置',
  install_assistant: '安装向导',
  new_user_tutorial: '新用户引导',
  model_selector: '模型选择',
  diagnostic_export: '诊断导出',
  announcements: '公告',
  force_update_prompt: '强制更新提示',
  strict_device_enforcement: '设备强制校验'
}

function isServerOnly(flag: string) {
  return serverOnlyFlags.has(flag)
}

function flagLabel(flag: string) {
  return labels[flag] || flag
}
</script>

<style scoped>
.server-note {
  @apply flex flex-wrap items-center gap-2 rounded-md border border-amber-300 bg-amber-50 px-3 py-2 text-xs text-amber-800 dark:border-amber-800 dark:bg-amber-950/40 dark:text-amber-200;
}

.server-note strong {
  @apply font-semibold;
}

.flag-row {
  @apply flex items-center justify-between gap-4 rounded-md border border-gray-200 px-3 py-2 dark:border-dark-700;
}

.flag-row.server-only {
  @apply border-amber-300 bg-amber-50/60 dark:border-amber-800 dark:bg-amber-950/20;
}

.flag-row strong {
  @apply block text-sm text-gray-900 dark:text-white;
}

.flag-title {
  @apply flex flex-wrap items-center gap-2;
}

.flag-title em {
  @apply rounded-full bg-amber-100 px-2 py-0.5 text-[11px] not-italic text-amber-800 dark:bg-amber-900 dark:text-amber-100;
}

.flag-row small {
  @apply block text-xs text-gray-500;
}
</style>
