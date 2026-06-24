<template>
  <section class="space-y-4">
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div>
        <p class="text-sm text-gray-600 dark:text-gray-300">
          设置用户使用 Codex++ 时的额度、并发和限速规则。发布后由服务端统一执行。
        </p>
        <p class="mt-1 text-xs text-gray-500">
          套餐会通过“用量规则编号”关联到这里。普通场景只需要维护默认规则。
        </p>
      </div>
      <button class="btn-primary" type="button" @click="addPolicy">新增规则</button>
    </div>

    <div v-if="copyKeyWarnings.length" class="warning-strip">
      <div v-for="warning in copyKeyWarnings" :key="warning">{{ warning }}</div>
    </div>

    <div class="overflow-x-auto">
      <table class="admin-table">
        <thead>
          <tr>
            <th>规则和适用范围</th>
            <th>额度</th>
            <th>速度限制</th>
            <th>过期处理</th>
            <th>提示文案</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="(policy, index) in policies" :key="policy.policy_id || index">
            <td class="min-w-[260px]">
              <label class="field-label">规则编号</label>
              <input v-model.trim="policy.policy_id" class="input mb-2" placeholder="default" />

              <label class="field-label">适用套餐</label>
              <input
                :value="formatList(policy.applies_to?.plan_ids)"
                class="input mb-1"
                placeholder="starter, pro"
                @input="updateScope(policy, 'plan_ids', ($event.target as HTMLInputElement).value)"
              />
              <div class="grid grid-cols-2 gap-1">
                <div>
                  <label class="field-label">模型分组</label>
                  <input
                    :value="formatList(policy.applies_to?.model_groups)"
                    class="input"
                    placeholder="default"
                    @input="updateScope(policy, 'model_groups', ($event.target as HTMLInputElement).value)"
                  />
                </div>
                <div>
                  <label class="field-label">用户分层</label>
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
                  <label class="field-label">低余额提醒</label>
                  <input v-model.number="policy.low_balance_threshold" class="input" type="number" min="0" />
                </div>
                <div>
                  <label class="field-label">每日额度</label>
                  <input v-model.number="policy.daily_quota" class="input" type="number" min="0" />
                </div>
              </div>
              <label class="field-label mt-2">每月额度</label>
              <input
                :value="policy.monthly_quota ?? ''"
                class="input"
                type="number"
                min="0"
                placeholder="不限制"
                @input="updateNullableNumber(policy, 'monthly_quota', ($event.target as HTMLInputElement).value)"
              />
              <p class="hint">填 0 表示当前规则不限制该项。</p>
            </td>

            <td class="min-w-[220px]">
              <div class="grid grid-cols-3 gap-1">
                <div>
                  <label class="field-label">并发</label>
                  <input v-model.number="policy.concurrency_limit" class="input" type="number" min="1" />
                </div>
                <div>
                  <label class="field-label">每分钟请求</label>
                  <input v-model.number="policy.rpm_limit" class="input" type="number" min="1" />
                </div>
                <div>
                  <label class="field-label">每分钟令牌</label>
                  <input v-model.number="policy.tpm_limit" class="input" type="number" min="1" />
                </div>
              </div>
              <div class="mt-2 grid grid-cols-2 gap-1">
                <div>
                  <label class="field-label">突发上限</label>
                  <input
                    :value="policy.burst_limit ?? policy.rpm_limit"
                    class="input"
                    type="number"
                    min="1"
                    @input="updateOptionalNumber(policy, 'burst_limit', ($event.target as HTMLInputElement).value)"
                  />
                </div>
                <div>
                  <label class="field-label">统计秒数</label>
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
              <label class="field-label">套餐过期后</label>
              <select v-model="policy.expired_behavior" class="input mb-2">
                <option value="block">停止使用</option>
                <option value="degrade">降级使用</option>
                <option value="allow_grace_period">允许宽限期</option>
              </select>
              <div class="grid grid-cols-2 gap-1">
                <div>
                  <label class="field-label">宽限小时</label>
                  <input v-model.number="policy.grace_period_hours" class="input" type="number" min="0" />
                </div>
                <div>
                  <label class="field-label">超额后</label>
                  <select v-model="policy.overage_behavior" class="input">
                    <option value="block">停止使用</option>
                    <option value="degrade">降级使用</option>
                    <option value="allow_paid_overage">允许付费超额</option>
                  </select>
                </div>
              </div>
            </td>

            <td class="min-w-[280px]">
              <label class="field-label">余额不足提示</label>
              <input v-model.trim="policy.insufficient_balance_message" class="input mb-1" />
              <label class="field-label">触发限速提示</label>
              <input v-model.trim="policy.rate_limited_message" class="input" />

              <details class="mt-2">
                <summary class="copy-summary">高级文案标识</summary>
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
                  自动填入系统标识
                </button>
              </details>
            </td>

            <td class="whitespace-nowrap">
              <button class="btn-danger" type="button" @click="removePolicy(index)">删除</button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <div class="preview-panel">
      <div class="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h3 class="text-sm font-semibold text-gray-900 dark:text-white">规则预览</h3>
          <p class="text-xs text-gray-500">
            这里用示例数值预估命中结果，最终以服务端执行为准。
          </p>
        </div>
        <span class="status-pill" :class="previewSeverity">{{ previewStatus }}</span>
      </div>

      <div class="mt-3 grid gap-3 lg:grid-cols-[minmax(0,360px)_1fr]">
        <div class="grid grid-cols-2 gap-2">
          <label class="preview-field">
            <span>关联规则</span>
            <input v-model.trim="preview.policyId" class="input" placeholder="default" />
          </label>
          <label class="preview-field">
            <span>套餐编号</span>
            <input v-model.trim="preview.planId" class="input" placeholder="starter" />
          </label>
          <label class="preview-field">
            <span>模型分组</span>
            <input v-model.trim="preview.modelGroup" class="input" placeholder="default" />
          </label>
          <label class="preview-field">
            <span>用户分层</span>
            <input v-model.trim="preview.userSegment" class="input" placeholder="internal" />
          </label>
          <label class="preview-field">
            <span>余额</span>
            <input v-model.number="preview.balance" class="input" type="number" min="0" />
          </label>
          <label class="preview-field">
            <span>今日已用</span>
            <input v-model.number="preview.dailyUsed" class="input" type="number" min="0" />
          </label>
          <label class="preview-field">
            <span>正在请求</span>
            <input v-model.number="preview.activeRequests" class="input" type="number" min="0" />
          </label>
          <label class="preview-field">
            <span>当前每分钟请求</span>
            <input v-model.number="preview.rpm" class="input" type="number" min="0" />
          </label>
          <label class="preview-field">
            <span>当前每分钟令牌</span>
            <input v-model.number="preview.tpm" class="input" type="number" min="0" />
          </label>
          <label class="preview-field">
            <span>已过期小时</span>
            <input v-model.number="preview.expiredHours" class="input" type="number" min="0" />
          </label>
          <label class="preview-check">
            <input v-model="preview.subscriptionExpired" type="checkbox" />
            套餐已过期
          </label>
        </div>

        <div class="hit-box">
          <div class="mb-2 flex flex-wrap items-center gap-2">
            <strong>{{ previewPolicy?.policy_id || '没有匹配规则' }}</strong>
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
      warnings.push(`规则 ${policy.policy_id || '新规则'} 需要一个合法的“余额不足”文案标识。`)
    }
    if (!keys?.rate_limited_message && policy.rate_limited_message && !isCopyKey(policy.rate_limited_message)) {
      warnings.push(`规则 ${policy.policy_id || '新规则'} 需要一个合法的“触发限速”文案标识。`)
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
    return '套餐直接关联'
  }
  if (scopeScore(policy) > 0 && scopeMatches(policy)) return '适用范围匹配'
  if (same(policy.policy_id, 'default')) return '默认规则'
  return '使用第一条规则'
})

const previewLines = computed<PreviewLine[]>(() => {
  const policy = previewPolicy.value
  if (!policy) {
    return [{ text: '还没有配置用量规则。', level: 'block' }]
  }

  const lines: PreviewLine[] = [
    {
      text: `限制：每日 ${describeQuota(policy.daily_quota)}，并发 ${policy.concurrency_limit}，每分钟请求 ${policy.rpm_limit}，每分钟令牌 ${policy.tpm_limit}。`,
      level: 'ok'
    }
  ]

  if (policy.low_balance_threshold > 0 && preview.value.balance <= policy.low_balance_threshold) {
    lines.push({
      text: `余额已低于提醒线：${policy.low_balance_threshold}。`,
      level: preview.value.balance <= 0 ? 'block' : 'warn'
    })
  }

  if (preview.value.balance <= 0) {
    lines.push({
      text: `会返回余额不足提示：${policy.insufficient_balance_message || policy.copy_keys?.insufficient_balance_message || '未设置'}。`,
      level: 'block'
    })
  }

  if (policy.daily_quota > 0 && preview.value.dailyUsed >= policy.daily_quota) {
    lines.push({
      text: `今日额度已用完：${preview.value.dailyUsed} / ${policy.daily_quota}。`,
      level: 'block'
    })
  }

  if (preview.value.activeRequests >= policy.concurrency_limit) {
    lines.push({
      text: `下一次请求会超过并发上限 ${policy.concurrency_limit}。`,
      level: 'block'
    })
  }

  if (preview.value.rpm >= policy.rpm_limit) {
    lines.push({
      text: `会触发每分钟请求限制，返回提示：${policy.rate_limited_message || policy.copy_keys?.rate_limited_message || '未设置'}。`,
      level: 'block'
    })
  }

  if (preview.value.tpm >= policy.tpm_limit) {
    lines.push({ text: `会触发每分钟令牌限制：${preview.value.tpm} / ${policy.tpm_limit}。`, level: 'block' })
  }

  if (preview.value.subscriptionExpired) {
    lines.push(expiredPreviewLine(policy))
  } else {
    lines.push({ text: '示例中的套餐仍在有效期内。', level: 'ok' })
  }

  if (lines.every(line => line.level === 'ok')) {
    lines.push({ text: '示例数值没有触发阻止规则。', level: 'ok' })
  }

  return lines
})

const previewSeverity = computed(() => {
  if (previewLines.value.some(line => line.level === 'block')) return 'block'
  if (previewLines.value.some(line => line.level === 'warn')) return 'warn'
  return 'ok'
})

const previewStatus = computed(() => {
  if (previewSeverity.value === 'block') return '会被阻止'
  if (previewSeverity.value === 'warn') return '需要关注'
  return '可以通过'
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
    return { text: '套餐过期后会降级使用，不会完全阻止。', level: 'warn' }
  }
  if (policy.expired_behavior === 'allow_grace_period') {
    const withinGrace = preview.value.expiredHours <= policy.grace_period_hours
    return withinGrace
      ? { text: `已过期，但仍在 ${policy.grace_period_hours} 小时宽限期内。`, level: 'warn' }
      : { text: `已超过宽限期，过期 ${preview.value.expiredHours} 小时。`, level: 'block' }
  }
  return { text: '套餐过期后会立即停止使用。', level: 'block' }
}

function describeQuota(value: number) {
  return value > 0 ? String(value) : '不限制'
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
