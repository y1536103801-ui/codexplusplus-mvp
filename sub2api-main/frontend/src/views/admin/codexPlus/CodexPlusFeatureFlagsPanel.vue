<template>
  <section class="space-y-3">
    <div class="server-note">
      <strong>strict_device_enforcement is server-only.</strong>
      <span>Published backend config decides gateway enforcement; desktop clients can only report device context.</span>
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
            <strong>{{ flag }}</strong>
            <em v-if="isServerOnly(flag)">server/gateway</em>
          </span>
          <small>{{ descriptions[flag] || 'Desktop/bootstrap feature flag' }}</small>
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
  advanced_provider_config: 'Show advanced provider settings in desktop.',
  install_assistant: 'Enable guided installation flows.',
  new_user_tutorial: 'Show first-run tutorial surfaces.',
  model_selector: 'Allow users to choose available models.',
  diagnostic_export: 'Enable diagnostic export tools.',
  announcements: 'Show Codex++ announcements.',
  force_update_prompt: 'Prompt users when an update is required.',
  strict_device_enforcement: 'Server rollout switch: when enabled, managed gateway requests without valid device context are rejected.'
}

function isServerOnly(flag: string) {
  return serverOnlyFlags.has(flag)
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
