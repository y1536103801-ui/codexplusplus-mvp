<template>
  <section class="space-y-4">
    <div class="plan-toolbar">
      <div>
        <h3 class="toolbar-title">套餐列表</h3>
        <p class="toolbar-desc">每张卡片就是一个套餐。卡片里的字段只属于该套餐，删除按钮在卡片右上角。</p>
      </div>
      <div class="flex flex-wrap gap-2">
        <RouterLink class="btn-secondary" to="/admin/orders/plans">支付套餐</RouterLink>
        <RouterLink class="btn-secondary" to="/admin/groups">用户组</RouterLink>
        <button class="btn-primary" type="button" @click="addPlan">新增套餐</button>
      </div>
    </div>

    <div v-if="!editablePlans.length" class="empty-card">
      <div class="font-medium text-gray-900 dark:text-white">还没有套餐</div>
      <p>点击“新增套餐”创建第一项，填写后记得保存配置。</p>
    </div>

    <div v-else class="plan-list">
      <article v-for="(plan, index) in editablePlans" :key="plan.plan_id || index" class="plan-card">
        <header class="plan-card-header">
          <div class="min-w-0">
            <div class="flex flex-wrap items-center gap-2">
              <span class="plan-index">套餐 {{ index + 1 }}</span>
              <span class="pill" :class="{ ok: plan.status === 'active', warning: plan.status !== 'active' }">
                {{ statusLabel(plan.status) }}
              </span>
              <span class="pill" :class="{ ok: plan.is_listed, warning: !plan.is_listed }">
                {{ plan.is_listed ? '用户可见' : '用户不可见' }}
              </span>
              <span class="pill" :class="{ ok: isPurchasable(plan), warning: !isPurchasable(plan) }">
                {{ isPurchasable(plan) ? '可购买' : '未开放购买' }}
              </span>
            </div>
            <h4 class="plan-title">{{ planDisplayName(plan, index) }}</h4>
            <p class="plan-subtitle">
              {{ plan.display_price || '未设置价格' }} · {{ periodLabel(plan.billing_period) }} ·
              {{ formatQuotaLabel(plan.entitlement_grant.balance_credit, '次') }}
            </p>
          </div>
          <div class="plan-actions">
            <button class="btn-secondary px-3 py-1.5 text-xs" type="button" @click="downlistPlan(plan)">下架</button>
            <button class="btn-danger px-3 py-1.5 text-xs" type="button" @click="removePlan(index)">删除套餐</button>
          </div>
        </header>

        <div class="plan-grid">
          <section class="plan-section">
            <div class="section-heading">基本信息</div>
            <div class="two-col">
              <label class="field">
                <span class="field-label">套餐编号</span>
                <input v-model.trim="plan.plan_id" class="input" placeholder="team_monthly" />
              </label>
              <label class="field">
                <span class="field-label">排序</span>
                <input v-model.number="plan.sort_order" class="input" min="0" type="number" />
              </label>
            </div>
            <label class="field">
              <span class="field-label">套餐名称</span>
              <input v-model.trim="plan.name" class="input" placeholder="团队套餐" />
            </label>
            <label class="field">
              <span class="field-label">用户看到的说明</span>
              <textarea
                v-model.trim="plan.description"
                class="input min-h-[82px] resize-y"
                placeholder="显示给用户看的套餐说明"
              ></textarea>
            </label>
            <div class="two-col">
              <label class="field">
                <span class="field-label">状态</span>
                <select v-model="plan.status" class="input" @change="syncStatus(plan)">
                  <option value="active">上架</option>
                  <option value="hidden">隐藏</option>
                  <option value="disabled">停用</option>
                </select>
              </label>
              <label class="check mt-6">
                <input v-model="plan.is_listed" type="checkbox" @change="syncListing(plan)" />
                用户可见
              </label>
            </div>
          </section>

          <section class="plan-section">
            <div class="section-heading">价格与购买</div>
            <label class="field">
              <span class="field-label">周期</span>
              <select v-model="plan.billing_period" class="input">
                <option value="none">无周期</option>
                <option value="trial">试用</option>
                <option value="monthly">月付</option>
                <option value="quarterly">季付</option>
                <option value="yearly">年付</option>
                <option value="one_time">一次性</option>
              </select>
            </label>
            <div class="two-col">
              <label class="field">
                <span class="field-label">显示价格</span>
                <input v-model.trim="plan.display_price" class="input" placeholder="¥99 / 月" />
              </label>
              <label class="field">
                <span class="field-label">金额（分）</span>
                <input v-model.number="plan.price_amount_minor" class="input" min="0" type="number" />
              </label>
            </div>
            <label class="field">
              <span class="field-label">币种</span>
              <input
                v-model="plan.currency"
                class="input uppercase"
                maxlength="3"
                placeholder="USD"
                @blur="plan.currency = plan.currency.trim().toUpperCase()"
              />
            </label>
            <label class="field">
              <span class="field-label">购买链接</span>
              <input
                v-model="plan.purchase_url"
                class="input"
                placeholder="https://..."
                @blur="normalizePlanUrl(plan, 'purchase_url')"
              />
            </label>
            <label class="field">
              <span class="field-label">续费链接</span>
              <input
                v-model="plan.renew_url"
                class="input"
                placeholder="https://..."
                @blur="normalizePlanUrl(plan, 'renew_url')"
              />
            </label>
          </section>

          <section class="plan-section">
            <div class="section-heading">用户获得</div>
            <div class="two-col">
              <label class="field">
                <span class="field-label">发放余额</span>
                <input v-model.number="plan.entitlement_grant.balance_credit" class="input" min="0" type="number" />
              </label>
              <label class="field">
                <span class="field-label">有效天数</span>
                <input v-model.number="plan.entitlement_grant.duration_days" class="input" min="0" type="number" />
              </label>
            </div>
            <div class="two-col">
              <label class="field">
                <span class="field-label">每日额度</span>
                <input
                  :value="nullableNumberValue(plan.entitlement_grant.daily_quota)"
                  class="input"
                  min="0"
                  placeholder="不限制"
                  type="number"
                  @input="plan.entitlement_grant.daily_quota = parseNullableNumber(($event.target as HTMLInputElement).value)"
                />
              </label>
              <label class="field">
                <span class="field-label">周期额度</span>
                <input
                  :value="nullableNumberValue(plan.entitlement_grant.period_quota)"
                  class="input"
                  min="0"
                  placeholder="不限制"
                  type="number"
                  @input="plan.entitlement_grant.period_quota = parseNullableNumber(($event.target as HTMLInputElement).value)"
                />
              </label>
            </div>
            <label class="field">
              <span class="field-label">模型分组</span>
              <input
                :value="plan.model_groups.join(', ')"
                class="input"
                placeholder="codex_standard, codex_premium"
                @input="plan.model_groups = splitList(($event.target as HTMLInputElement).value)"
              />
            </label>
            <label class="field">
              <span class="field-label">用量规则</span>
              <input v-model.trim="plan.usage_policy_id" class="input" placeholder="default_policy" />
            </label>
          </section>

          <section class="plan-section">
            <div class="section-heading">绑定来源</div>
            <label class="field">
              <span class="field-label">订阅用户组</span>
              <input
                :value="formatNumberList(plan.entitlement_sources.subscription_group_ids)"
                class="input"
                list="codex-payment-group-candidates"
                placeholder="101, 102"
                @input="plan.entitlement_sources.subscription_group_ids = splitNumberList(($event.target as HTMLInputElement).value)"
              />
            </label>
            <label class="field">
              <span class="field-label">密钥用户组</span>
              <input
                :value="formatNumberList(plan.entitlement_sources.api_key_group_ids)"
                class="input"
                list="codex-payment-group-candidates"
                placeholder="201, 202"
                @input="plan.entitlement_sources.api_key_group_ids = splitNumberList(($event.target as HTMLInputElement).value)"
              />
            </label>
            <label class="field">
              <span class="field-label">用户组名称</span>
              <input
                :value="plan.entitlement_sources.group_names.join(', ')"
                class="input"
                placeholder="codex-plus-pro"
                @input="plan.entitlement_sources.group_names = splitList(($event.target as HTMLInputElement).value)"
              />
            </label>
            <div class="two-col">
              <label class="field">
                <span class="field-label">商品编号</span>
                <input
                  v-model="plan.external_billing_refs.product_id"
                  class="input"
                  placeholder="商品 ID"
                  @blur="normalizeBillingRef(plan, 'product_id')"
                />
              </label>
              <label class="field">
                <span class="field-label">规格编号</span>
                <input
                  v-model="plan.external_billing_refs.sku_id"
                  class="input"
                  placeholder="SKU ID"
                  @blur="normalizeBillingRef(plan, 'sku_id')"
                />
              </label>
            </div>
            <div class="mt-1">
              <span class="pill" :class="{ ok: hasEntitlementSources(plan), warning: !hasEntitlementSources(plan) }">
                {{ hasEntitlementSources(plan) ? '已绑定权益来源' : '未绑定权益来源' }}
              </span>
            </div>
          </section>
        </div>

        <div class="plan-preview">
          <div>
            <span class="preview-label">用户侧预览</span>
            <strong>{{ planDisplayName(plan, index) }}</strong>
            <span>{{ plan.display_price || '未设置价格' }}，{{ plan.description || '暂无说明' }}</span>
          </div>
          <div class="flex flex-wrap gap-1">
            <span v-for="group in plan.model_groups" :key="group" class="pill">{{ group }}</span>
          </div>
        </div>

        <details class="details">
          <summary>高级文案标识（通常不用改）</summary>
          <div class="mt-2 grid gap-2 md:grid-cols-2 xl:grid-cols-3">
            <input v-model.trim="plan.copy_keys.purchase_action" class="input" placeholder="billing.action.purchase" />
            <input v-model.trim="plan.copy_keys.renew_action" class="input" placeholder="billing.action.renew" />
            <input v-model.trim="plan.copy_keys.upgrade_action" class="input" placeholder="billing.action.upgrade" />
            <input
              v-model.trim="plan.copy_keys.not_purchased_message"
              class="input"
              placeholder="billing.message.not_purchased"
            />
            <input v-model.trim="plan.copy_keys.expired_message" class="input" placeholder="billing.message.expired" />
            <input
              v-model.trim="plan.copy_keys.low_balance_message"
              class="input"
              placeholder="billing.message.low_balance"
            />
          </div>
        </details>
      </article>
    </div>

    <datalist id="codex-payment-group-candidates">
      <option v-for="group in paymentGroupOptions" :key="group.id" :value="group.id">
        {{ group.label }}
      </option>
    </datalist>

    <div v-if="paymentPlans.length" class="reference-table">
      <div class="reference-title">已有支付套餐</div>
      <div class="reference-grid">
        <span v-for="plan in paymentPlans" :key="plan.id">
          #{{ plan.id }} {{ plan.name }} / 用户组 {{ plan.group_id }} / {{ plan.price }} / {{ plan.validity_days }}
          {{ plan.validity_unit }}
        </span>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, watch } from 'vue'
import type { CodexPlusOptions, CodexPlusPlan } from '@/api/admin/codexPlus'

interface PlanEntitlementSources {
  subscription_group_ids: number[]
  api_key_group_ids: number[]
  group_names: string[]
}

interface PlanCopyKeys {
  purchase_action: string
  renew_action: string
  upgrade_action: string
  not_purchased_message: string
  expired_message: string
  low_balance_message: string
}

interface PlanExternalBillingRefs {
  product_id: string | null
  sku_id: string | null
}

type AdminPlan = CodexPlusPlan & {
  price_amount_minor: number
  entitlement_grant: CodexPlusPlan['entitlement_grant'] & {
    period_quota?: number | null
  }
  entitlement_sources: PlanEntitlementSources
  usage_policy_id: string
  purchase_url: string | null
  copy_keys: PlanCopyKeys
  sort_order: number
  external_billing_refs: PlanExternalBillingRefs
}

const props = defineProps<{
  plans: CodexPlusPlan[]
  paymentPlans: CodexPlusOptions['payment_plans']
}>()

const editablePlans = computed(() => props.plans as AdminPlan[])

watch(
  () => props.plans,
  () => normalizePlans(),
  { immediate: true }
)

const paymentGroupOptions = computed(() => {
  const seen = new Set<number>()
  return props.paymentPlans
    .map(plan => ({ id: plan.group_id, label: `用户组 ${plan.group_id}，来自 ${plan.name}` }))
    .filter(group => {
      if (!group.id || seen.has(group.id)) return false
      seen.add(group.id)
      return true
    })
})

function splitList(value: string): string[] {
  return value.split(',').map(item => item.trim()).filter(Boolean)
}

function splitNumberList(value: string): number[] {
  const seen = new Set<number>()
  return value
    .split(',')
    .map(item => Number(item.trim()))
    .filter(item => Number.isInteger(item) && item > 0)
    .filter(item => {
      if (seen.has(item)) return false
      seen.add(item)
      return true
    })
}

function formatNumberList(values: number[]): string {
  return values.join(', ')
}

function nullableNumberValue(value: number | null | undefined): string {
  return typeof value === 'number' ? String(value) : ''
}

function parseNullableNumber(value: string): number | null {
  if (value.trim() === '') return null
  const parsed = Number(value)
  return Number.isFinite(parsed) && parsed >= 0 ? parsed : null
}

function addPlan() {
  const nextIndex = props.plans.length + 1
  const plan = {
    plan_id: `plan_${nextIndex}`,
    name: '新套餐',
    description: '待完善的 Codex++ 套餐。',
    billing_period: 'monthly',
    currency: 'USD',
    price_amount_minor: 0,
    display_price: '待定',
    entitlement_grant: { balance_credit: 0, duration_days: 30, daily_quota: null, period_quota: null },
    entitlement_sources: emptyEntitlementSources(),
    model_groups: ['default'],
    usage_policy_id: 'default_policy',
    purchase_url: null,
    renew_url: null,
    copy_keys: defaultCopyKeys(),
    is_listed: false,
    status: 'hidden',
    sort_order: nextSortOrder(),
    external_billing_refs: { product_id: null, sku_id: null }
  } satisfies AdminPlan
  ensurePlanDefaults(plan)
  props.plans.push(plan)
}

function removePlan(index: number) {
  const plan = props.plans[index] as AdminPlan | undefined
  const label = plan ? planDisplayName(plan, index) : `套餐 ${index + 1}`
  if (typeof window !== 'undefined' && !window.confirm(`删除套餐「${label}」？保存配置后才会正式生效。`)) return
  props.plans.splice(index, 1)
}

function planDisplayName(plan: AdminPlan, index: number): string {
  return plan.name || plan.plan_id || `套餐 ${index + 1}`
}

function statusLabel(value: string): string {
  const labels: Record<string, string> = {
    active: '上架',
    hidden: '隐藏',
    disabled: '停用'
  }
  return labels[value] || value || '未设置'
}

function formatQuotaLabel(value: number | null | undefined, unit: string): string {
  return typeof value === 'number' ? `${value} ${unit}` : '不限制'
}

function downlistPlan(plan: AdminPlan) {
  plan.status = 'hidden'
  plan.is_listed = false
  plan.purchase_url = null
}

function syncStatus(plan: AdminPlan) {
  if (plan.status !== 'active') {
    plan.is_listed = false
    plan.purchase_url = null
  }
  if (plan.status === 'disabled') {
    plan.renew_url = null
  }
}

function syncListing(plan: AdminPlan) {
  if (plan.is_listed) {
    plan.status = 'active'
  } else {
    plan.purchase_url = null
  }
  syncStatus(plan)
}

function normalizePlanUrl(plan: AdminPlan, field: 'purchase_url' | 'renew_url') {
  const raw = plan[field]
  plan[field] = typeof raw === 'string' && raw.trim() ? raw.trim() : null
  syncStatus(plan)
}

function normalizeBillingRef(plan: AdminPlan, field: keyof PlanExternalBillingRefs) {
  const raw = plan.external_billing_refs[field]
  plan.external_billing_refs[field] = typeof raw === 'string' && raw.trim() ? raw.trim() : null
}

function hasEntitlementSources(plan: AdminPlan): boolean {
  return (
    plan.entitlement_sources.subscription_group_ids.length > 0 ||
    plan.entitlement_sources.api_key_group_ids.length > 0 ||
    plan.entitlement_sources.group_names.length > 0
  )
}

function isPurchasable(plan: AdminPlan): boolean {
  return plan.status === 'active' && plan.is_listed && !!plan.purchase_url
}

function periodLabel(value: string): string {
  const labels: Record<string, string> = {
    none: '无周期',
    trial: '试用',
    monthly: '月付',
    quarterly: '季付',
    yearly: '年付',
    one_time: '一次性'
  }
  return labels[value] || value || '-'
}

function ensurePlanDefaults(plan: AdminPlan) {
  plan.description = plan.description || ''
  plan.status = plan.status || 'hidden'
  plan.currency = plan.currency || 'USD'
  plan.price_amount_minor = typeof plan.price_amount_minor === 'number' ? plan.price_amount_minor : 0
  plan.usage_policy_id = plan.usage_policy_id || ''
  plan.purchase_url = plan.purchase_url ?? null
  plan.renew_url = plan.renew_url ?? null
  plan.sort_order = typeof plan.sort_order === 'number' ? plan.sort_order : 0

  plan.entitlement_grant = plan.entitlement_grant || { balance_credit: 0, duration_days: 0, daily_quota: null }
  plan.entitlement_grant.daily_quota = plan.entitlement_grant.daily_quota ?? null
  plan.entitlement_grant.period_quota = plan.entitlement_grant.period_quota ?? null

  plan.entitlement_sources = plan.entitlement_sources || emptyEntitlementSources()
  plan.entitlement_sources.subscription_group_ids = plan.entitlement_sources.subscription_group_ids || []
  plan.entitlement_sources.api_key_group_ids = plan.entitlement_sources.api_key_group_ids || []
  plan.entitlement_sources.group_names = plan.entitlement_sources.group_names || []

  plan.copy_keys = { ...defaultCopyKeys(), ...(plan.copy_keys || {}) }
  plan.external_billing_refs = plan.external_billing_refs || { product_id: null, sku_id: null }
  plan.external_billing_refs.product_id = plan.external_billing_refs.product_id ?? null
  plan.external_billing_refs.sku_id = plan.external_billing_refs.sku_id ?? null
  syncStatus(plan)
}

function normalizePlans() {
  for (const plan of props.plans as AdminPlan[]) {
    ensurePlanDefaults(plan)
  }
}

function nextSortOrder(): number {
  const orders = (props.plans as AdminPlan[]).map(plan => plan.sort_order || 0)
  return orders.length ? Math.max(...orders) + 10 : 10
}

function emptyEntitlementSources(): PlanEntitlementSources {
  return { subscription_group_ids: [], api_key_group_ids: [], group_names: [] }
}

function defaultCopyKeys(): PlanCopyKeys {
  return {
    purchase_action: 'billing.action.purchase',
    renew_action: 'billing.action.renew',
    upgrade_action: 'billing.action.upgrade',
    not_purchased_message: 'billing.message.not_purchased',
    expired_message: 'billing.message.expired',
    low_balance_message: 'billing.message.low_balance'
  }
}
</script>

<style scoped>
.plan-toolbar {
  @apply flex flex-wrap items-start justify-between gap-3 rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-900;
}

.toolbar-title {
  @apply text-base font-semibold text-gray-900 dark:text-white;
}

.toolbar-desc {
  @apply mt-1 text-sm text-gray-500 dark:text-gray-400;
}

.empty-card {
  @apply rounded-lg border border-dashed border-gray-300 bg-white p-6 text-sm text-gray-500 dark:border-dark-600 dark:bg-dark-900 dark:text-gray-400;
}

.plan-list {
  @apply space-y-4;
}

.plan-card {
  @apply rounded-lg border border-gray-200 bg-white p-4 shadow-sm dark:border-dark-700 dark:bg-dark-900;
}

.plan-card-header {
  @apply mb-4 flex flex-wrap items-start justify-between gap-3 border-b border-gray-100 pb-4 dark:border-dark-800;
}

.plan-index {
  @apply rounded bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-600 dark:bg-dark-800 dark:text-gray-300;
}

.plan-title {
  @apply mt-2 truncate text-xl font-semibold text-gray-900 dark:text-white;
}

.plan-subtitle {
  @apply mt-1 text-sm text-gray-500 dark:text-gray-400;
}

.plan-actions {
  @apply flex flex-wrap gap-2;
}

.plan-grid {
  @apply grid gap-3 lg:grid-cols-2 2xl:grid-cols-4;
}

.plan-section {
  @apply rounded-lg border border-gray-200 bg-gray-50 p-3 dark:border-dark-700 dark:bg-dark-800/60;
}

.section-heading {
  @apply mb-3 text-sm font-semibold text-gray-900 dark:text-white;
}

.plan-preview {
  @apply mt-4 flex flex-wrap items-center justify-between gap-3 rounded-lg border border-dashed border-gray-200 bg-white px-3 py-2 text-sm text-gray-600 dark:border-dark-700 dark:bg-dark-900 dark:text-gray-300;
}

.plan-preview > div:first-child {
  @apply flex min-w-0 flex-wrap items-center gap-2;
}

.preview-label {
  @apply text-xs font-medium text-gray-400 dark:text-gray-500;
}

.two-col {
  @apply grid grid-cols-2 gap-2;
}

.field {
  @apply mb-1.5 block;
}

.field-label {
  @apply mb-1 block text-[11px] font-medium uppercase text-gray-500 dark:text-gray-400;
}

.input {
  @apply w-full rounded-md border border-gray-300 bg-white px-2 py-1.5 text-sm text-gray-900 placeholder:text-gray-400 dark:border-dark-600 dark:bg-dark-800 dark:text-gray-100;
}

.check {
  @apply flex items-center gap-2 text-xs text-gray-700 dark:text-gray-300;
}

.details {
  @apply rounded-md border border-gray-200 px-2 py-1.5 text-xs text-gray-600 dark:border-dark-700 dark:text-gray-300;
}

.details summary {
  @apply cursor-pointer font-medium;
}

.summary {
  @apply space-y-1 text-xs text-gray-600 dark:text-gray-300;
}

.pill {
  @apply rounded border border-gray-200 bg-gray-50 px-1.5 py-0.5 text-[11px] text-gray-600 dark:border-dark-700 dark:bg-dark-800 dark:text-gray-300;
}

.pill.ok {
  @apply border-green-200 bg-green-50 text-green-700 dark:border-green-900 dark:bg-green-950 dark:text-green-200;
}

.pill.warning {
  @apply border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-900 dark:bg-amber-950 dark:text-amber-200;
}

.reference-table {
  @apply rounded-md border border-gray-200 bg-white px-3 py-2 text-xs text-gray-500 dark:border-dark-700 dark:bg-dark-900 dark:text-gray-400;
}

.reference-title {
  @apply mb-1 font-medium text-gray-700 dark:text-gray-200;
}

.reference-grid {
  @apply grid gap-1 md:grid-cols-2 xl:grid-cols-3;
}

.btn-primary {
  @apply rounded-md bg-primary-600 px-3 py-2 text-sm font-medium text-white hover:bg-primary-700;
}

.btn-secondary {
  @apply rounded-md border border-gray-300 px-3 py-2 text-sm hover:bg-gray-50 dark:border-dark-600 dark:hover:bg-dark-800;
}

.btn-danger {
  @apply rounded-md border border-red-300 px-2 py-1 text-xs text-red-600 hover:bg-red-50 dark:border-red-800 dark:hover:bg-red-950;
}
</style>
