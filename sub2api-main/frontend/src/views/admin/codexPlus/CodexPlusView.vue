<template>
  <div class="space-y-5 p-4 md:p-6">
    <header class="flex flex-wrap items-start justify-between gap-3">
      <div>
        <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">Codex++ Operations</h1>
        <p class="mt-1 text-sm text-gray-600 dark:text-gray-300">
          Publish validated bootstrap config while reusing existing payment plans, groups, and subscriptions.
        </p>
      </div>
      <div class="flex flex-wrap gap-2">
        <button class="btn-secondary" type="button" :disabled="loading" @click="loadAll">Reload</button>
        <button class="btn-secondary" type="button" :disabled="!draft || validating" @click="validateServer">
          {{ validating ? 'Validating...' : 'Validate' }}
        </button>
        <button class="btn-primary" type="button" :disabled="!canPublish" @click="publish">
          {{ publishing ? 'Publishing...' : 'Publish' }}
        </button>
      </div>
    </header>

    <div v-if="loading" class="notice">Loading Codex++ admin state...</div>
    <div v-if="error" class="error">{{ error }}</div>
    <div v-if="validationErrors.length" class="error">
      <div v-for="item in validationErrors" :key="item">{{ item }}</div>
    </div>
    <div v-if="message" class="notice">{{ message }}</div>

    <template v-if="draft">
      <nav class="tabs" aria-label="Codex++ admin sections">
        <button
          v-for="tab in tabs"
          :key="tab.key"
          class="tab"
          :class="{ active: activeTab === tab.key }"
          type="button"
          @click="activeTab = tab.key"
        >
          {{ tab.label }}
        </button>
      </nav>

      <CodexPlusConfigPanel
        v-if="activeTab === 'overview'"
        :draft="draft"
        :change-reason="changeReason"
        @update:change-reason="changeReason = $event"
      />
      <CodexPlusPlanCatalogPanel
        v-else-if="activeTab === 'plans'"
        :plans="draft.plan_catalog.plans"
        :payment-plans="options.payment_plans"
      />
      <CodexPlusModelCatalogPanel
        v-else-if="activeTab === 'models'"
        :models="draft.model_catalog.models"
        :model-candidates="options.models"
      />
      <CodexPlusUsagePolicyPanel
        v-else-if="activeTab === 'usage'"
        :policies="draft.usage_policy.policies"
      />
      <CodexPlusFeatureFlagsPanel
        v-else-if="activeTab === 'flags'"
        :model="draft.feature_flags.flags"
        :flags="options.feature_flags"
      />
      <CodexPlusUserEntitlementPanel v-else-if="activeTab === 'users'" />
      <CodexPlusConfigVersionsPanel
        v-else
        :versions="versions"
        @refresh="loadVersions"
        @rollback="rollback"
      />
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import {
  getCodexPlusConfig,
  getCodexPlusOptions,
  listCodexPlusConfigVersions,
  publishCodexPlusConfig,
  rollbackCodexPlusConfig,
  validateCodexPlusConfig,
  type CodexPlusConfig,
  type CodexPlusOptions,
  type CodexPlusVersionEntry
} from '@/api/admin/codexPlus'
import CodexPlusConfigPanel from './CodexPlusConfigPanel.vue'
import CodexPlusPlanCatalogPanel from './CodexPlusPlanCatalogPanel.vue'
import CodexPlusModelCatalogPanel from './CodexPlusModelCatalogPanel.vue'
import CodexPlusUsagePolicyPanel from './CodexPlusUsagePolicyPanel.vue'
import CodexPlusFeatureFlagsPanel from './CodexPlusFeatureFlagsPanel.vue'
import CodexPlusUserEntitlementPanel from './CodexPlusUserEntitlementPanel.vue'
import CodexPlusConfigVersionsPanel from './CodexPlusConfigVersionsPanel.vue'

type TabKey = 'overview' | 'plans' | 'models' | 'usage' | 'flags' | 'users' | 'versions'

const tabs: Array<{ key: TabKey; label: string }> = [
  { key: 'overview', label: 'Overview' },
  { key: 'plans', label: 'Plans' },
  { key: 'models', label: 'Models' },
  { key: 'usage', label: 'Usage Policy' },
  { key: 'flags', label: 'Feature Flags' },
  { key: 'users', label: 'Users and Devices' },
  { key: 'versions', label: 'Versions and Audit' }
]

const emptyOptions: CodexPlusOptions = {
  groups: [],
  payment_plans: [],
  models: [],
  feature_flags: [
    'advanced_provider_config',
    'install_assistant',
    'new_user_tutorial',
    'model_selector',
    'diagnostic_export',
    'announcements',
    'force_update_prompt',
    'strict_device_enforcement'
  ],
  policy_presets: []
}

const activeTab = ref<TabKey>('overview')
const loading = ref(false)
const validating = ref(false)
const publishing = ref(false)
const error = ref('')
const message = ref('')
const changeReason = ref('')
const draft = ref<CodexPlusConfig | null>(null)
const options = ref<CodexPlusOptions>(emptyOptions)
const versions = ref<CodexPlusVersionEntry[]>([])

const validationErrors = computed(() => draft.value ? validateDraft(draft.value) : [])
const canPublish = computed(() => !!draft.value && validationErrors.value.length === 0 && !publishing.value)

onMounted(loadAll)

async function loadAll() {
  loading.value = true
  error.value = ''
  message.value = ''
  try {
    const [config, opts, versionList] = await Promise.all([
      getCodexPlusConfig(),
      getCodexPlusOptions(),
      listCodexPlusConfigVersions()
    ])
    draft.value = clone(config)
    options.value = opts
    versions.value = versionList
    changeReason.value = config.change_reason || ''
  } catch (err) {
    error.value = (err as { message?: string }).message || 'Failed to load Codex++ admin state'
  } finally {
    loading.value = false
  }
}

async function loadVersions() {
  versions.value = await listCodexPlusConfigVersions()
}

async function validateServer() {
  if (!draft.value) return
  validating.value = true
  error.value = ''
  message.value = ''
  try {
    await validateCodexPlusConfig(draft.value)
    message.value = 'Server validation passed.'
  } catch (err) {
    error.value = (err as { message?: string }).message || 'Server validation failed'
  } finally {
    validating.value = false
  }
}

async function publish() {
  if (!draft.value || !canPublish.value) return
  publishing.value = true
  error.value = ''
  message.value = ''
  try {
    await validateCodexPlusConfig(draft.value)
    const published = await publishCodexPlusConfig(draft.value, changeReason.value)
    draft.value = clone(published)
    await loadVersions()
    message.value = `Published ${published.config_version}.`
  } catch (err) {
    error.value = (err as { message?: string }).message || 'Publish failed'
  } finally {
    publishing.value = false
  }
}

async function rollback(version: string) {
  if (!window.confirm(`Rollback Codex++ config from ${version}? This creates a new published version.`)) return
  error.value = ''
  message.value = ''
  try {
    const rolledBack = await rollbackCodexPlusConfig(version, `rollback from ${version}`)
    draft.value = clone(rolledBack)
    await loadVersions()
    message.value = `Rolled back as ${rolledBack.config_version}.`
  } catch (err) {
    error.value = (err as { message?: string }).message || 'Rollback failed'
  }
}

function validateDraft(config: CodexPlusConfig): string[] {
  const errors: string[] = []
  const planIds = new Set<string>()
  for (const plan of config.plan_catalog.plans) {
    if (!plan.plan_id) errors.push('Each plan needs a plan_id.')
    if (plan.plan_id && planIds.has(plan.plan_id)) errors.push(`Duplicate plan_id ${plan.plan_id}.`)
    planIds.add(plan.plan_id)
    if (!plan.name) errors.push(`Plan ${plan.plan_id || '(new)'} needs a name.`)
    if (!plan.model_groups.length) errors.push(`Plan ${plan.plan_id || '(new)'} needs at least one model group.`)
    if (plan.entitlement_grant.balance_credit < 0 || plan.entitlement_grant.duration_days < 0) {
      errors.push(`Plan ${plan.plan_id || '(new)'} has negative entitlement values.`)
    }
  }

  const modelIds = new Set<string>()
  const defaults = config.model_catalog.models.filter(model => model.is_default)
  if (defaults.length !== 1) errors.push('Exactly one default model is required.')
  for (const model of config.model_catalog.models) {
    if (!model.model_id) errors.push('Each model needs a model_id.')
    if (model.model_id && modelIds.has(model.model_id)) errors.push(`Duplicate model_id ${model.model_id}.`)
    modelIds.add(model.model_id)
    if (model.is_default && !model.is_enabled) errors.push('Default model must be enabled.')
    if (model.context_window < 1024) errors.push(`Model ${model.model_id} context window must be at least 1024.`)
    if (model.billing_multiplier <= 0) errors.push(`Model ${model.model_id} billing multiplier must be positive.`)
  }

  for (const policy of config.usage_policy.policies) {
    if (!policy.policy_id) errors.push('Each usage policy needs a policy_id.')
    if (policy.concurrency_limit < 1 || policy.rpm_limit < 1 || policy.tpm_limit < 1) {
      errors.push(`Policy ${policy.policy_id || '(new)'} limits must be positive.`)
    }
    if (!policy.insufficient_balance_message) {
      errors.push(`Policy ${policy.policy_id || '(new)'} needs an insufficient balance message.`)
    }
  }
  return Array.from(new Set(errors))
}

function clone<T>(value: T): T {
  return JSON.parse(JSON.stringify(value)) as T
}
</script>

<style scoped>
.btn-primary {
  @apply rounded-md bg-primary-600 px-3 py-2 text-sm font-medium text-white hover:bg-primary-700 disabled:cursor-not-allowed disabled:opacity-50;
}

.btn-secondary {
  @apply rounded-md border border-gray-300 px-3 py-2 text-sm hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-50 dark:border-dark-600 dark:hover:bg-dark-800;
}

.tabs {
  @apply flex flex-wrap gap-2 border-b border-gray-200 dark:border-dark-700;
}

.tab {
  @apply -mb-px rounded-t-md px-3 py-2 text-sm text-gray-600 hover:bg-gray-50 dark:text-gray-300 dark:hover:bg-dark-800;
}

.tab.active {
  @apply border border-gray-200 border-b-white bg-white font-medium text-primary-700 dark:border-dark-700 dark:border-b-dark-900 dark:bg-dark-900 dark:text-primary-300;
}

.notice {
  @apply rounded-md border border-blue-200 bg-blue-50 px-3 py-2 text-sm text-blue-800 dark:border-blue-900 dark:bg-blue-950 dark:text-blue-200;
}

.error {
  @apply rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700 dark:border-red-900 dark:bg-red-950 dark:text-red-200;
}
</style>
