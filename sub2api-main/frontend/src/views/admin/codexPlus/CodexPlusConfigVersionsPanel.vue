<template>
  <section class="space-y-3">
    <div class="flex items-center justify-between gap-2">
      <p class="text-sm text-gray-600 dark:text-gray-300">Rollback writes the selected snapshot as a new validated version.</p>
      <button class="btn-secondary" type="button" @click="$emit('refresh')">Refresh</button>
    </div>

    <div class="overflow-x-auto">
      <table class="table">
        <thead>
          <tr>
            <th>Version</th>
            <th>Scope</th>
            <th>Updated</th>
            <th>Actor</th>
            <th>Reason</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="entry in versions" :key="entry.config_version">
            <td><code>{{ entry.config_version }}</code></td>
            <td>{{ entry.publish_scope }}</td>
            <td>{{ formatDate(entry.updated_at) }}</td>
            <td>{{ entry.updated_by }}</td>
            <td>{{ entry.rollback_from ? `rollback from ${entry.rollback_from}` : entry.change_reason }}</td>
            <td>
              <button class="btn-danger" type="button" @click="$emit('rollback', entry.config_version)">
                Rollback
              </button>
            </td>
          </tr>
          <tr v-if="!versions.length"><td colspan="6">No version history yet.</td></tr>
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
