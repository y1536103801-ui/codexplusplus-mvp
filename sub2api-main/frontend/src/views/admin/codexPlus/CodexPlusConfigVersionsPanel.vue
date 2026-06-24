<template>
  <section class="space-y-3">
    <div class="flex items-center justify-between gap-2">
      <p class="text-sm text-gray-600 dark:text-gray-300">恢复历史版本会生成一份新的已发布配置，不会直接覆盖审计记录。</p>
      <button class="btn-secondary" type="button" @click="$emit('refresh')">刷新</button>
    </div>

    <div class="overflow-x-auto">
      <table class="table">
        <thead>
          <tr>
            <th>版本</th>
            <th>发布范围</th>
            <th>更新时间</th>
            <th>操作人</th>
            <th>说明</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="entry in versions" :key="entry.config_version">
            <td><code>{{ entry.config_version }}</code></td>
            <td>{{ scopeLabel(entry.publish_scope) }}</td>
            <td>{{ formatDate(entry.updated_at) }}</td>
            <td>{{ entry.updated_by }}</td>
            <td>{{ versionReason(entry) }}</td>
            <td>
              <button class="btn-danger" type="button" @click="$emit('rollback', entry.config_version)">
                恢复
              </button>
            </td>
          </tr>
          <tr v-if="!versions.length"><td colspan="6">还没有版本记录。</td></tr>
        </tbody>
      </table>
    </div>
  </section>
</template>

<script setup lang="ts">
import type { CodexPlusVersionEntry } from '@/api/admin/codexPlus'

defineProps<{ versions: CodexPlusVersionEntry[] }>()
defineEmits<{
  refresh: []
  rollback: [version: string]
}>()

function formatDate(value: string) {
  if (!value) return '-'
  return new Date(value).toLocaleString()
}

function scopeLabel(value: string) {
  const labels: Record<string, string> = {
    draft: '草稿',
    internal: '内部测试',
    canary: '小范围灰度',
    production: '正式发布'
  }
  return labels[value] || value || '-'
}

function versionReason(entry: CodexPlusVersionEntry) {
  if (entry.rollback_from) return `从 ${entry.rollback_from} 恢复`
  const reason = (entry.change_reason || '').trim()
  if (!reason) return '未填写'
  if (/initial hidden Codex\+\+ MVP config/i.test(reason)) return '系统初始化配置'
  if (/rollback from/i.test(reason)) return reason.replace(/rollback from/i, '从').replace(/$/, ' 恢复')
  return reason
}
</script>

<style scoped>
.table {
  @apply w-full min-w-[900px] text-left text-sm;
}

.table th {
  @apply border-b border-gray-200 px-2 py-2 text-xs uppercase text-gray-500 dark:border-dark-700;
}

.table td {
  @apply border-b border-gray-100 px-2 py-2 align-top dark:border-dark-800;
}

.btn-secondary {
  @apply rounded-md border border-gray-300 px-3 py-2 text-sm hover:bg-gray-50 dark:border-dark-600 dark:hover:bg-dark-800;
}

.btn-danger {
  @apply rounded-md border border-red-300 px-2 py-1 text-xs text-red-600 hover:bg-red-50 dark:border-red-800 dark:hover:bg-red-950;
}
</style>
