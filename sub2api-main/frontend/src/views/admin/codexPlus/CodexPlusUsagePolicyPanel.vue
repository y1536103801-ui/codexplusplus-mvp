<template>
  <section class="space-y-4">
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div>
        <p class="text-sm text-gray-600 dark:text-gray-300">
          Runtime limits are published here, but entitlement and gateway enforcement remain server-side.
        </p>
        <p class="mt-1 text-xs text-gray-500">
          Plan overrides should point plans at a usage_policy_id; scope fields are kept for backend policy metadata.
        </p>
      </div>
      <button class="btn-primary" type="button" @click="addPolicy">Add policy</button>
    </div>

    <div v-if="copyKeyWarnings.length" class="warning-strip">
      <div v-for="warning in copyKeyWarnings" :key="warning">{{ warning }}</div>
    </div>

    <div class="overflow-x-auto">
      <table class="admin-table">
        <thead>
          <tr>
            <th>Policy and scope</th>
            <th>Quota</th>
            <th>Rate limits</th>
            <th>Expiry</th>
            <th>Messages</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="(policy, index) in policies" :key="policy.policy_id || index">
            <td class="min-w-[260px]">
              <label class="field-label">policy_id</label>
              <input v-model.trim="policy.policy_id" class="input mb-2" placeholder="default" />

              <label class="field-label">plan_ids</label>
              <input
                :value="formatList(policy.applies_to?.plan_ids)"
                class="input mb-1"
                placeholder="starter, pro"
                @input="updateScope(policy, 'plan_ids', ($event.target as HTMLInputElement).value)"
              />
              <div class="grid grid-cols-2 gap-1">
                <div>
                  <label class="field-label">model groups</label>
                  <input
                    :value="formatList(policy.applies_to?.model_groups)"
                    class="input"
                    placeholder="default"
                    @input="updateScope(policy, 'model_groups', ($event.target as HTMLInputElement).value)"
                  />
                </div>
                <div>
                  <label class="field-label">segments</label>
                  <input
                    :value="formatList(policy.applies_to?.user_segments)"
                    class="input"
                    placeholder="internal"
                    @input="updateScope(policy, 'user_segments', ($event.target as HTMLInputElement).value)"
                  />
                </div>
              </div>
            </td>

            <td class="min-w-[190px]">
              <div class="grid grid-cols-2 gap-1">
                <div>
                  <label class="field-label">low balance</label>
                  <input v-model.number="policy.low_balance_threshold" class="input" type="number" min="0" />
                </div>
                <div>
                  <label class="field-label">daily</label>
                  <input v-model.number="policy.daily_quota" class="input" type="number" min="0" />
                </div>
              </div>
              <label class="field-label mt-2">monthly</label>
              <input
                :value="policy.monthly_quota ?? ''"
                class="input"
                type="number"
                min="0"
                placeholder="optional"
                @input="updateNullableNumber(policy, 'monthly_quota', ($event.target as HTMLInputElement).value)"
              />
              <p class="hint">Use 0 for no quota in this draft.</p>
            </td>

            <td class="min-w-[220px]">
              <div class="grid grid-cols-3 gap-1">
                <div>
                  <label class="field-label">concur</label>
                  <input v-model.number="policy.concurrency_limit" class="input" type="number" min="1" />
                </div>
                <div>
                  <label class="field-label">RPM</label>
                  <input v-model.number="policy.rpm_limit" class="input" type="number" min="1" />
                </div>
                <div>
                  <label class="field-label">TPM</label>
                  <input v-model.number="policy.tpm_limit" class="input" type="number" min="1" />
                </div>
              </div>
              <div class="mt-2 grid grid-cols-2 gap-1">
                <div>
                  <label class="field-label">burst</label>
                  <input
                    :value="policy.burst_limit ?? policy.rpm_limit"
                    class="input"
                    type="number"
                    min="1"
                    @input="updateOptionalNumber(policy, 'burst_limit', ($event.target as HTMLInputElement).value)"
                  />
                </div>
                <div>
                  <label class="field-label">window sec</label>
                  <input
                    :value="policy.rate_limit_window_seconds ?? 60"
                    class="input"
                    type="number"
                    min="1"
                    @input="updateOptionalNumber(policy, 'rate_limit_window_seconds', ($event.target as HTMLInputElement).value)"
                  />
                </div>
              </div>
            </td>

            <td class="min-w-[200px]">
              <label class="field-label">expired behavior</label>
              <select v-model="policy.expired_behavior" class="input mb-2">
                <option value="block">block</option>
                <option value="degrade">degrade</option>
                <option value="allow_grace_period">allow_grace_period</option>
              </select>
              <div class="grid grid-cols-2 gap-1">
                <div>
                  <label class="field-label">grace hrs</label>
                  <input v-model.number="policy.grace_period_hours" class="input" type="number" min="0" />
                </div>
                <div>
                  <label class="field-label">overage</label>
                  <select v-model="policy.overage_behavior" class="input">
                    <option value="block">block</option>
                    <option value="degrade">degrade</option>
                    <option value="allow_paid_overage">paid</option>
                  </select>
                </div>
              </div>
            </td>

            <td class="min-w-[280px]">
              <label class="field-label">insufficient balance message / key</label>
              <input v-model.trim="policy.insufficient_balance_message" class="input mb-1" />
              <label class="field-label">rate limited message / key</label>
              <input v-model.trim="policy.rate_limited_message" class="input" />

              <details class="mt-2">
                <summary class="copy-summary">Copy keys</summary>
                <div class="mt-2 grid grid-cols-2 gap-1">
                  <input
                    :value="policy.copy_keys?.low_balance_message || ''"
                    class="input"
                    placeholder="usage.low_balance"
                    @input="updateCopyKey(policy, 'low_balance_message', ($event.target as HTMLInputElement).value)"
                  />
                  <input
                    :value="policy.copy_keys?.insufficient_balance_message || ''"
                    class="input"
                    placeholder="usage.insufficient_balance"
                    @input="updateCopyKey(policy, 'insufficient_balance_message', ($event.target as HTMLInputElement).value)"
                  />
                  <input
                    :value="policy.copy_keys?.rate_limited_message || ''"
                    class="input"
                    placeholder="usage.rate_limited"
                    @input="updateCopyKey(policy, 'rate_limited_message', ($event.target as HTMLInputElement).value)"
                  />
                  <input
                    :value="policy.copy_keys?.expired_message || ''"
                    class="input"
                    placeholder="usage.expired"
                    @input="updateCopyKey(policy, 'expired_message', ($event.target as HTMLInputElement).value)"
                  />
                  <input
                    :value="policy.copy_keys?.renew_action || ''"
                    class="input"
                    placeholder="usage.renew_action"
                    @input="updateCopyKey(policy, 'renew_action', ($event.target as HTMLInputElement).value)"
                  />
                  <input
                    :value="policy.copy_keys?.purchase_action || ''"
                    class="input"
                    placeholder="usage.purchase_action"
                    @input="updateCopyKey(policy, 'purchase_action', ($event.target as HTMLInputElement).value)"
                  />
                  <input
                    :value="policy.copy_keys?.device_revoked_message || ''"
                    class="input"
                    placeholder="device.revoked"
                    @input="updateCopyKey(policy, 'device_revoked_message', ($event.target as HTMLInputElement).value)"
                  />
                </div>
                <button class="link-button mt-2" type="button" @click="seedCopyKeys(policy)">
                  Fill registry keys
                </button>
              </details>
            </td>

            <td class="whitespace-nowrap">
              <button class="btn-danger" type="button" @click="removePolicy(index)">Remove</button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <div class="preview-panel">
      <div class="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h3 class="text-sm font-semibold text-gray-900 dark:text-white">Policy preview</h3>
          <p class="text-xs text-gray-500">
            Local read-only estimate for operator review. Server validation and gateway telemetry remain authoritative.
          </p>
        </div>
        <span class="status-pill" :class="previewSeverity">{{ previewStatus }}</span>
      </div>

      <div class="mt-3 grid gap-3 lg:grid-cols-[minmax(0,360px)_1fr]">
        <div class="grid grid-cols-2 gap-2">
          <label class="preview-field">
            <span>linked policy_id</span>
            <input v-model.trim="preview.policyId" class="input" placeholder="default" />
          </label>
          <label class="preview-field">
            <span>plan_id</span>
            <input v-model.trim="preview.planId" class="input" placeholder="starter" />
          </label>
          <label class="preview-field">
            <span>model group</span>
            <input v-model.trim="preview.modelGroup" class="input" placeholder="default" />
          </label>
          <label class="preview-field">
            <span>segment</span>
            <input v-model.trim="preview.userSegment" class="input" placeholder="internal" />
          </label>
          <label class="preview-field">
            <span>balance</span>
            <input v-model.number="preview.balance" class="input" type="number" min="0" />
          </label>
          <label class="preview-field">
            <span>daily used</span>
            <input v-model.number="preview.dailyUsed" class="input" type="number" min="0" />
          </label>
          <label class="preview-field">
            <span>active req</span>
            <input v-model.number="preview.activeRequests" class="input" type="number" min="0" />
          </label>
          <label class="preview-field">
            <span>RPM now</span>
            <input v-model.number="preview.rpm" class="input" type="number" min="0" />
          </label>
          <label class="preview-field">
            <span>TPM now</span>
            <input v-model.number="preview.tpm" class="input" type="number" min="0" />
          </label>
          <label class="preview-field">
            <span>expired hrs</span>
            <input v-model.number="preview.expiredHours" class="input" type="number" min="0" />
          </label>
          <label class="preview-check">
            <input v-model="preview.subscriptionExpired" type="checkbox" />
            subscription expired
          </label>
        </div>

        <div class="hit-box">
          <div class="mb-2 flex flex-wrap items-center gap-2">
            <strong>{{ previewPolicy?.policy_id || 'No policy' }}</strong>
            <span v-if="previewSource" class="source-chip">{{ previewSource }}</span>
          </div>
          <ul class="space-y-1">
            <li v-for="line in previewLines" :key="line.text" :class="['hit-line', line.level]">
              {{ line.text }}
            </li>
          </ul>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import type { CodexPlusUsagePolicyCopyKeys, CodexPlusUsageRule } from '@/api/admin/codexPlus'

type ScopeKey = 'plan_ids' | 'model_groups' | 'user_segments'
type NumericKey = 'monthly_quota' | 'burst_limit' | 'rate_limit_window_seconds'
type CopyKey = keyof CodexPlusUsagePolicyCopyKeys
type HitLevel = 'ok' | 'warn' | 'block'

interface PreviewState {
  policyId: string
  planId: string
  modelGroup: string
  userSegment: string
  balance: number
  dailyUsed: number
  activeRequests: number
  rpm: number
  tpm: number
  subscriptionExpired: boolean
  expiredHours: number
}

interface PreviewLine {
  text: string
  level: HitLevel
}

const props = defineProps<{ policies: CodexPlusUsageRule[] }>()

const copyKeyPattern = /^[a-z][a-z0-9_.-]{2,127}$/
const preview = ref<PreviewState>({
  policyId: 'default',
  planId: '',
  modelGroup: 'default',
  userSegment: '',
  balance: 0,
  dailyUsed: 0,
  activeRequests: 0,
  rpm: 0,
  tpm: 0,
  subscriptionExpired: false,
  expiredHours: 0
})

const copyKeyWarnings = computed(() => {
  return props.policies.flatMap(policy => {
    const warnings: string[] = []
    const keys = policy.copy_keys
    if (!keys?.insufficient_balance_message && !isCopyKey(policy.insufficient_balance_message)) {
      warnings.push(`Policy ${policy.policy_id || '(new)'} needs a registry-safe insufficient balance copy key before publish.`)
    }
    if (!keys?.rate_limited_message && policy.rate_limited_message && !isCopyKey(policy.rate_limited_message)) {
      warnings.push(`Policy ${policy.policy_id || '(new)'} needs a registry-safe rate limited copy key before publish.`)
    }
    return warnings
  })
})

const previewPolicy = computed(() => {
  if (!props.policies.length) return null

  const requestedPolicyId = preview.value.policyId.trim()
  if (requestedPolicyId) {
    const direct = props.policies.find(policy => same(policy.policy_id, requestedPolicyId))
    if (direct) return direct
  }

  const scopedMatch = props.policies
    .map((policy, index) => ({ policy, index, score: scopeScore(policy) }))
    .filter(item => item.score > 0 && scopeMatches(item.policy))
    .sort((a, b) => b.score - a.score || a.index - b.index)[0]

  if (scopedMatch) return scopedMatch.policy
  return props.policies.find(policy => same(policy.policy_id, 'default')) || props.policies[0]
})

const previewSource = computed(() => {
  const policy = previewPolicy.value
  if (!policy) return ''
  if (preview.value.policyId.trim() && same(policy.policy_id, preview.value.policyId)) {
    return 'plan usage_policy_id match'
  }
  if (scopeScore(policy) > 0 && scopeMatches(policy)) return 'scope metadata match'
  if (same(policy.policy_id, 'default')) return 'default fallback'
  return 'first configured fallback'
})

const previewLines = computed<PreviewLine[]>(() => {
  const policy = previewPolicy.value
  if (!policy) {
    return [{ text: 'No usage policy is configured.', level: 'block' }]
  }

  const lines: PreviewLine[] = [
    {
      text: `Limits: daily ${describeQuota(policy.daily_quota)}, concurrency ${policy.concurrency_limit}, ${policy.rpm_limit} RPM, ${policy.tpm_limit} TPM.`,
      level: 'ok'
    }
  ]

  if (policy.low_balance_threshold > 0 && preview.value.balance <= policy.low_balance_threshold) {
    lines.push({
      text: `Low balance threshold hit at ${policy.low_balance_threshold}.`,
      level: preview.value.balance <= 0 ? 'block' : 'warn'
    })
  }

  if (preview.value.balance <= 0) {
    lines.push({
      text: `Insufficient balance response: ${policy.insufficient_balance_message || policy.copy_keys?.insufficient_balance_message || 'missing copy'}.`,
      level: 'block'
    })
  }

  if (policy.daily_quota > 0 && preview.value.dailyUsed >= policy.daily_quota) {
    lines.push({
      text: `Daily quota exhausted: ${preview.value.dailyUsed} / ${policy.daily_quota}.`,
      level: 'block'
    })
  }

  if (preview.value.activeRequests >= policy.concurrency_limit) {
    lines.push({
      text: `Next request would exceed concurrency ${policy.concurrency_limit}.`,
      level: 'block'
    })
  }

  if (preview.value.rpm >= policy.rpm_limit) {
    lines.push({
      text: `RPM limit would return: ${policy.rate_limited_message || policy.copy_keys?.rate_limited_message || 'missing copy'}.`,
      level: 'block'
    })
  }

  if (preview.value.tpm >= policy.tpm_limit) {
    lines.push({ text: `TPM limit reached: ${preview.value.tpm} / ${policy.tpm_limit}.`, level: 'block' })
  }

  if (preview.value.subscriptionExpired) {
    lines.push(expiredPreviewLine(policy))
  } else {
    lines.push({ text: 'Subscription is active in this preview.', level: 'ok' })
  }

  if (lines.every(line => line.level === 'ok')) {
    lines.push({ text: 'No blocking threshold is crossed by the preview inputs.', level: 'ok' })
  }

  return lines
})

const previewSeverity = computed(() => {
  if (previewLines.value.some(line => line.level === 'block')) return 'block'
  if (previewLines.value.some(line => line.level === 'warn')) return 'warn'
  return 'ok'
})

const previewStatus = computed(() => {
  if (previewSeverity.value === 'block') return 'would block'
  if (previewSeverity.value === 'warn') return 'attention'
  return 'would allow'
})

function addPolicy() {
  const index = props.policies.length + 1
  props.policies.push({
    policy_id: `policy_${index}`,
    applies_to: { plan_ids: [], model_groups: [], user_segments: [] },
    low_balance_threshold: 0,
    daily_quota: 0,
    monthly_quota: null,
    concurrency_limit: 1,
    rpm_limit: 60,
    tpm_limit: 60000,
    burst_limit: 60,
    rate_limit_window_seconds: 60,
    expired_behavior: 'block',
    grace_period_hours: 0,
    overage_behavior: 'block',
    copy_keys: defaultCopyKeys(),
    device_policy: {
      registration_required: true,
      max_devices_per_user: 2,
      allow_self_service_replacement: false,
      replacement_cooldown_hours: 24,
      strict_enforcement_default: true,
      revoke_reason_taxonomy: ['user_requested', 'admin_revoked', 'device_limit_exceeded', 'risk_control', 'unknown'],
      support_unlock_policy: 'support_only',
      revoked_behavior: 'block_bootstrap',
      message_keys: {
        limit_reached: 'device.limit_reached',
        replacement_cooldown: 'device.replacement_cooldown',
        revoked: 'device.revoked',
        support_unlock_required: 'device.support_unlock_required'
      }
    },
    insufficient_balance_message: 'usage.insufficient_balance',
    rate_limited_message: 'usage.rate_limited'
  })
}

function removePolicy(index: number) {
  if (props.policies.length <= 1) return
  props.policies.splice(index, 1)
}

function updateScope(policy: CodexPlusUsageRule, key: ScopeKey, value: string) {
  if (!policy.applies_to) {
    policy.applies_to = { plan_ids: [], model_groups: [], user_segments: [] }
  }
  policy.applies_to[key] = splitList(value)
}

function updateNullableNumber(policy: CodexPlusUsageRule, key: 'monthly_quota', value: string) {
  policy[key] = value.trim() === '' ? null : Math.max(0, Number(value) || 0)
}

function updateOptionalNumber(policy: CodexPlusUsageRule, key: Exclude<NumericKey, 'monthly_quota'>, value: string) {
  policy[key] = Math.max(1, Number(value) || 1)
}

function updateCopyKey(policy: CodexPlusUsageRule, key: CopyKey, value: string) {
  if (!policy.copy_keys) policy.copy_keys = {}
  policy.copy_keys[key] = value.trim()
}

function seedCopyKeys(policy: CodexPlusUsageRule) {
  const defaults = defaultCopyKeys()
  policy.copy_keys = {
    low_balance_message: policy.copy_keys?.low_balance_message || defaults.low_balance_message,
    insufficient_balance_message: policy.copy_keys?.insufficient_balance_message || defaults.insufficient_balance_message,
    rate_limited_message: policy.copy_keys?.rate_limited_message || defaults.rate_limited_message,
    expired_message: policy.copy_keys?.expired_message || defaults.expired_message,
    renew_action: policy.copy_keys?.renew_action || defaults.renew_action,
    purchase_action: policy.copy_keys?.purchase_action || defaults.purchase_action,
    device_revoked_message: policy.copy_keys?.device_revoked_message || defaults.device_revoked_message
  }
  policy.insufficient_balance_message = policy.copy_keys.insufficient_balance_message || 'usage.insufficient_balance'
  policy.rate_limited_message = policy.copy_keys.rate_limited_message || 'usage.rate_limited'
}

function defaultCopyKeys(): CodexPlusUsagePolicyCopyKeys {
  return {
    low_balance_message: 'usage.low_balance',
    insufficient_balance_message: 'usage.insufficient_balance',
    rate_limited_message: 'usage.rate_limited',
    expired_message: 'usage.expired',
    renew_action: 'usage.renew_action',
    purchase_action: 'usage.purchase_action',
    device_revoked_message: 'device.revoked'
  }
}

function formatList(values?: string[]) {
  return (values || []).join(', ')
}

function splitList(value: string): string[] {
  return value
    .split(',')
    .map(item => item.trim())
    .filter(Boolean)
}

function scopeScore(policy: CodexPlusUsageRule) {
  const scope = policy.applies_to
  if (!scope) return 0
  return nonEmpty(scope.plan_ids) + nonEmpty(scope.model_groups) + nonEmpty(scope.user_segments)
}

function scopeMatches(policy: CodexPlusUsageRule) {
  const scope = policy.applies_to
  if (!scope) return false
  return (
    scopeListMatches(scope.plan_ids, preview.value.planId) &&
    scopeListMatches(scope.model_groups, preview.value.modelGroup) &&
    scopeListMatches(scope.user_segments, preview.value.userSegment)
  )
}

function scopeListMatches(values: string[] | undefined, candidate: string) {
  if (!values?.length) return true
  const trimmed = candidate.trim()
  return !!trimmed && values.some(value => same(value, trimmed))
}

function expiredPreviewLine(policy: CodexPlusUsageRule): PreviewLine {
  if (policy.expired_behavior === 'degrade') {
    return { text: 'Expired entitlement would degrade access instead of full block.', level: 'warn' }
  }
  if (policy.expired_behavior === 'allow_grace_period') {
    const withinGrace = preview.value.expiredHours <= policy.grace_period_hours
    return withinGrace
      ? { text: `Expired but still inside ${policy.grace_period_hours}h grace period.`, level: 'warn' }
      : { text: `Grace period exceeded after ${preview.value.expiredHours}h.`, level: 'block' }
  }
  return { text: 'Expired entitlement would be blocked immediately.', level: 'block' }
}

function describeQuota(value: number) {
  return value > 0 ? String(value) : 'not capped'
}

function nonEmpty(values?: string[]) {
  return values?.length ? 1 : 0
}

function same(left: string | undefined, right: string | undefined) {
  return (left || '').trim().toLowerCase() === (right || '').trim().toLowerCase()
}

function isCopyKey(value?: string) {
  return copyKeyPattern.test((value || '').trim())
}
</script>

<style scoped>
.admin-table {
  @apply w-full min-w-[1280px] text-left text-sm;
}

.admin-table th {
  @apply border-b border-gray-200 px-2 py-2 text-xs font-semibold uppercase text-gray-500 dark:border-dark-700;
}

.admin-table td {
  @apply border-b border-gray-100 px-2 py-2 align-top dark:border-dark-800;
}

.input {
  @apply w-full rounded-md border border-gray-300 bg-white px-2 py-1.5 text-sm dark:border-dark-600 dark:bg-dark-800;
}

.field-label {
  @apply mb-1 block text-[11px] font-medium uppercase text-gray-500;
}

.hint {
  @apply mt-1 text-[11px] text-gray-500;
}

.btn-primary {
  @apply rounded-md bg-primary-600 px-3 py-2 text-sm font-medium text-white hover:bg-primary-700;
}

.btn-danger {
  @apply rounded-md border border-red-300 px-2 py-1 text-xs text-red-600 hover:bg-red-50 dark:border-red-800 dark:hover:bg-red-950;
}

.link-button {
  @apply text-xs font-medium text-primary-600 hover:text-primary-700;
}

.warning-strip {
  @apply rounded-md border border-amber-300 bg-amber-50 px-3 py-2 text-xs text-amber-800 dark:border-amber-700 dark:bg-amber-950/40 dark:text-amber-200;
}

.copy-summary {
  @apply cursor-pointer text-xs font-medium text-gray-600 dark:text-gray-300;
}

.preview-panel {
  @apply rounded-md border border-gray-200 p-3 dark:border-dark-700;
}

.preview-field {
  @apply block;
}

.preview-field span {
  @apply mb-1 block text-[11px] font-medium uppercase text-gray-500;
}

.preview-check {
  @apply col-span-2 flex items-center gap-2 text-xs text-gray-700 dark:text-gray-200;
}

.hit-box {
  @apply min-h-[180px] rounded-md border border-gray-200 bg-gray-50 p-3 text-sm dark:border-dark-700 dark:bg-dark-900;
}

.source-chip {
  @apply rounded-full bg-gray-200 px-2 py-0.5 text-[11px] text-gray-700 dark:bg-dark-700 dark:text-gray-200;
}

.status-pill {
  @apply rounded-full px-2.5 py-1 text-xs font-semibold;
}

.status-pill.ok {
  @apply bg-emerald-100 text-emerald-700 dark:bg-emerald-950 dark:text-emerald-300;
}

.status-pill.warn {
  @apply bg-amber-100 text-amber-700 dark:bg-amber-950 dark:text-amber-300;
}

.status-pill.block {
  @apply bg-red-100 text-red-700 dark:bg-red-950 dark:text-red-300;
}

.hit-line {
  @apply rounded px-2 py-1 text-xs;
}

.hit-line.ok {
  @apply bg-white text-gray-700 dark:bg-dark-800 dark:text-gray-200;
}

.hit-line.warn {
  @apply bg-amber-100 text-amber-800 dark:bg-amber-950 dark:text-amber-200;
}

.hit-line.block {
  @apply bg-red-100 text-red-800 dark:bg-red-950 dark:text-red-200;
}
</style>
