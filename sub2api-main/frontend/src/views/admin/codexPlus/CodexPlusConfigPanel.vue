<template>
  <section class="space-y-4">
    <div class="grid gap-4 md:grid-cols-4">
      <label class="space-y-1">
        <span class="text-xs font-medium text-gray-500">Scope</span>
        <select v-model="draft.publish_scope" class="input">
          <option value="draft">draft</option>
          <option value="internal">internal</option>
          <option value="canary">canary</option>
          <option value="production">production</option>
        </select>
      </label>
      <label class="space-y-1 md:col-span-3">
        <span class="text-xs font-medium text-gray-500">Change reason</span>
        <input v-model="reason" class="input" placeholder="Short operational note for audit history" />
      </label>
    </div>

    <div class="grid gap-3 md:grid-cols-4">
      <div class="metric">
        <span>Plans</span>
        <strong>{{ draft.plan_catalog.plans.length }}</strong>
      </div>
      <div class="metric">
        <span>Models</span>
        <strong>{{ draft.model_catalog.models.length }}</strong>
      </div>
      <div class="metric">
        <span>Policies</span>
        <strong>{{ draft.usage_policy.policies.length }}</strong>
      </div>
      <div class="metric">
        <span>Enabled flags</span>
        <strong>{{ enabledFlagCount }}</strong>
      </div>
    </div>

    <div class="text-sm text-gray-600 dark:text-gray-300">
      Current version <code>{{ currentVersion }}</code>. Last update:
      <code>{{ draft.updated_at || 'not published' }}</code> by
      <code>{{ draft.updated_by || 'unknown' }}</code>.
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

const currentVersion = computed(() => props.draft.config_version || 'unversioned')
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
