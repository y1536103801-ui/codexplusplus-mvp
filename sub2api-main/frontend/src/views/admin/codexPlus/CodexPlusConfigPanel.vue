<template>
  <section class="space-y-4">
    <div class="grid gap-4 md:grid-cols-4">
      <label class="space-y-1">
        <span class="text-xs font-medium text-gray-500">发布范围</span>
        <select v-model="draft.publish_scope" class="input">
          <option value="draft">草稿</option>
          <option value="internal">内部测试</option>
          <option value="canary">小范围灰度</option>
          <option value="production">正式发布</option>
        </select>
      </label>
      <label class="space-y-1 md:col-span-3">
        <span class="text-xs font-medium text-gray-500">本次修改说明</span>
        <input v-model="reason" class="input" placeholder="例如：调整团队套餐额度，方便后续追踪" />
      </label>
    </div>

    <div class="grid gap-3 md:grid-cols-4">
      <div class="metric">
        <span>套餐</span>
        <strong>{{ draft.plan_catalog.plans.length }}</strong>
      </div>
      <div class="metric">
        <span>模型</span>
        <strong>{{ draft.model_catalog.models.length }}</strong>
      </div>
      <div class="metric">
        <span>用量规则</span>
        <strong>{{ draft.usage_policy.policies.length }}</strong>
      </div>
      <div class="metric">
        <span>已开启功能</span>
        <strong>{{ enabledFlagCount }}</strong>
      </div>
    </div>

    <div class="text-sm text-gray-600 dark:text-gray-300">
      当前版本 <code>{{ currentVersion }}</code>。上次更新：
      <code>{{ draft.updated_at || '尚未发布' }}</code>，操作人：
      <code>{{ draft.updated_by || '未知' }}</code>。
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { CodexPlusConfig } from '@/api/admin/codexPlus'

const props = defineProps<{
  draft: CodexPlusConfig
  changeReason: string
}>()

const emit = defineEmits<{
  'update:changeReason': [value: string]
}>()

const reason = computed({
  get: () => props.changeReason,
  set: (value: string) => emit('update:changeReason', value)
})

const currentVersion = computed(() => props.draft.config_version || '未生成版本')
const enabledFlagCount = computed(() => Object.values(props.draft.feature_flags.flags).filter(Boolean).length)
</script>

<style scoped>
.input {
  @apply w-full rounded-md border border-gray-300 bg-white px-3 py-2 text-sm dark:border-dark-600 dark:bg-dark-800;
}

.metric {
  @apply rounded-md border border-gray-200 px-3 py-2 dark:border-dark-700;
}

.metric span {
  @apply block text-xs text-gray-500;
}

.metric strong {
  @apply text-lg text-gray-900 dark:text-white;
}
</style>
