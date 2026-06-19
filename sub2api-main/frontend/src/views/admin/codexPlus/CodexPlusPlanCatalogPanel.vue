<template>
  <section class="space-y-3">
    <div class="flex flex-wrap items-center justify-between gap-2">
      <p class="text-sm text-gray-600 dark:text-gray-300">
        Sale price and entitlement fulfillment stay in existing payment plans, groups, and subscriptions.
      </p>
      <div class="flex flex-wrap gap-2">
        <RouterLink class="btn-secondary" to="/admin/orders/plans">Payment plans</RouterLink>
        <RouterLink class="btn-secondary" to="/admin/groups">Groups</RouterLink>
        <button class="btn-primary" type="button" @click="addPlan">Add plan</button>
      </div>
    </div>

    <div class="overflow-x-auto">
      <table class="admin-table">
        <thead>
          <tr>
            <th>Plan</th>
            <th>Price</th>
            <th>Commerce</th>
            <th>Grant</th>
            <th>Sources</th>
            <th>Access</th>
            <th>Status</th>
            <th>User summary</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="(plan, index) in editablePlans" :key="plan.plan_id || index">
            <td class="w-[230px]">
              <div class="two-col">
                <label class="field">
                  <span class="field-label">Plan ID</span>
                  <input v-model.trim="plan.plan_id" class="input" placeholder="pro_monthly" />
                </label>
                <label class="field">
                  <span class="field-label">Sort</span>
                  <input v-model.number="plan.sort_order" class="input" min="0" type="number" />
                </label>
              </div>
              <label class="field">
                <span class="field-label">Name</span>
                <input v-model.trim="plan.name" class="input" placeholder="Codex++ Pro Monthly" />
              </label>
              <label class="field">
                <span class="field-label">Description</span>
                <textarea
                  v-model.trim="plan.description"
                  class="input min-h-[68px] resize-y"
                  placeholder="Shown in the user plan snapshot"
                ></textarea>
              </label>
              <label class="field">
                <span class="field-label">Usage policy</span>
                <input v-model.trim="plan.usage_policy_id" class="input" placeholder="pro_monthly_policy" />
              </label>
            </td>

            <td class="w-[190px]">
              <label class="field">
                <span class="field-label">Period</span>
                <select v-model="plan.billing_period" class="input">
                  <option value="none">none</option>
                  <option value="trial">trial</option>
                  <option value="monthly">monthly</option>
                  <option value="quarterly">quarterly</option>
                  <option value="yearly">yearly</option>
                  <option value="one_time">one_time</option>
                </select>
              </label>
              <div class="two-col">
                <label class="field">
                  <span class="field-label">Currency</span>
                  <input
                    v-model="plan.currency"
                    class="input uppercase"
                    maxlength="3"
                    placeholder="USD"
                    @blur="plan.currency = plan.currency.trim().toUpperCase()"
                  />
                </label>
                <label class="field">
                  <span class="field-label">Minor amount</span>
                  <input v-model.number="plan.price_amount_minor" class="input" min="0" type="number" />
                </label>
              </div>
              <label class="field">
                <span class="field-label">Display price</span>
                <input v-model.trim="plan.display_price" class="input" placeholder="$19.99/month" />
              </label>
            </td>

            <td class="w-[260px]">
              <label class="field">
                <span class="field-label">Purchase URL</span>
                <input
                  v-model="plan.purchase_url"
                  class="input"
                  placeholder="https://..."
                  @blur="normalizePlanUrl(plan, 'purchase_url')"
                />
              </label>
              <label class="field">
                <span class="field-label">Renew URL</span>
                <input
                  v-model="plan.renew_url"
                  class="input"
                  placeholder="https://..."
                  @blur="normalizePlanUrl(plan, 'renew_url')"
                />
              </label>
              <div class="two-col">
                <label class="field">
                  <span class="field-label">Product ref</span>
                  <input
                    v-model="plan.external_billing_refs.product_id"
                    class="input"
                    placeholder="product id"
                    @blur="normalizeBillingRef(plan, 'product_id')"
                  />
                </label>
                <label class="field">
                  <span class="field-label">SKU ref</span>
                  <input
                    v-model="plan.external_billing_refs.sku_id"
                    class="input"
                    placeholder="sku id"
                    @blur="normalizeBillingRef(plan, 'sku_id')"
                  />
                </label>
              </div>
            </td>

            <td class="w-[190px]">
              <label class="field">
                <span class="field-label">Balance credit</span>
                <input v-model.number="plan.entitlement_grant.balance_credit" class="input" min="0" type="number" />
              </label>
              <label class="field">
                <span class="field-label">Duration days</span>
                <input v-model.number="plan.entitlement_grant.duration_days" class="input" min="0" type="number" />
              </label>
              <label class="field">
                <span class="field-label">Daily quota</span>
                <input
                  :value="nullableNumberValue(plan.entitlement_grant.daily_quota)"
                  class="input"
                  min="0"
                  placeholder="null"
                  type="number"
                  @input="plan.entitlement_grant.daily_quota = parseNullableNumber(($event.target as HTMLInputElement).value)"
                />
              </label>
              <label class="field">
                <span class="field-label">Period quota</span>
                <input
                  :value="nullableNumberValue(plan.entitlement_grant.period_quota)"
                  class="input"
                  min="0"
                  placeholder="null"
                  type="number"
                  @input="plan.entitlement_grant.period_quota = parseNullableNumber(($event.target as HTMLInputElement).value)"
                />
              </label>
            </td>

            <td class="w-[250px]">
              <label class="field">
                <span class="field-label">Subscription group IDs</span>
                <input
                  :value="formatNumberList(plan.entitlement_sources.subscription_group_ids)"
                  class="input"
                  list="codex-payment-group-candidates"
                  placeholder="101, 102"
                  @input="plan.entitlement_sources.subscription_group_ids = splitNumberList(($event.target as HTMLInputElement).value)"
                />
              </label>
              <label class="field">
                <span class="field-label">API key group IDs</span>
                <input
                  :value="formatNumberList(plan.entitlement_sources.api_key_group_ids)"
                  class="input"
                  list="codex-payment-group-candidates"
                  placeholder="201, 202"
                  @input="plan.entitlement_sources.api_key_group_ids = splitNumberList(($event.target as HTMLInputElement).value)"
                />
              </label>
              <label class="field">
                <span class="field-label">Group names</span>
                <input
                  :value="plan.entitlement_sources.group_names.join(', ')"
                  class="input"
                  placeholder="codex-plus-pro"
                  @input="plan.entitlement_sources.group_names = splitList(($event.target as HTMLInputElement).value)"
                />
              </label>
              <div class="mt-1 flex flex-wrap gap-1">
                <span class="pill" :class="{ warning: !hasEntitlementSources(plan) }">
                  {{ hasEntitlementSources(plan) ? 'mapped' : 'no mapping' }}
                </span>
              </div>
            </td>

            <td class="w-[250px]">
              <label class="field">
                <span class="field-label">Model groups</span>
                <input
                  :value="plan.model_groups.join(', ')"
                  class="input"
                  placeholder="codex_standard, codex_premium"
                  @input="plan.model_groups = splitList(($event.target as HTMLInputElement).value)"
                />
              </label>

              <details class="details">
                <summary>Copy keys</summary>
                <div class="mt-2 space-y-1.5">
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
            </td>

            <td class="w-[145px]">
              <label class="field">
                <span class="field-label">Status</span>
                <select v-model="plan.status" class="input" @change="syncStatus(plan)">
                  <option value="active">active</option>
                  <option value="hidden">hidden</option>
                  <option value="disabled">disabled</option>
                </select>
              </label>
              <label class="check">
                <input v-model="plan.is_listed" type="checkbox" @change="syncListing(plan)" />
                listed
              </label>
              <div class="mt-2 flex flex-wrap gap-1">
                <span class="pill" :class="{ ok: isPurchasable(plan), warning: !isPurchasable(plan) }">
                  {{ isPurchasable(plan) ? 'purchasable' : 'not purchasable' }}
                </span>
                <span v-if="plan.renew_url" class="pill ok">renew</span>
              </div>
            </td>

            <td class="w-[250px]">
              <div class="summary">
                <div class="font-medium text-gray-900 dark:text-white">{{ plan.name || plan.plan_id || 'Unnamed plan' }}</div>
                <div>{{ plan.display_price || 'No display price' }} / {{ plan.billing_period }}</div>
                <div>{{ plan.description || 'No description' }}</div>
                <div class="mt-1 flex flex-wrap gap-1">
                  <span v-for="group in plan.model_groups" :key="group" class="pill">{{ group }}</span>
                </div>
              </div>
            </td>

            <td class="w-[118px]">
              <div class="space-y-1.5">
                <button class="btn-secondary w-full px-2 py-1 text-xs" type="button" @click="downlistPlan(plan)">Downlist</button>
                <button class="btn-danger w-full" type="button" @click="removePlan(index)">Remove row</button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>

      <datalist id="codex-payment-group-candidates">
        <option v-for="group in paymentGroupOptions" :key="group.id" :value="group.id">
          {{ group.label }}
        </option>
      </datalist>
    </div>

    <div v-if="paymentPlans.length" class="reference-table">
      <div class="reference-title">Available payment plans</div>
      <div class="reference-grid">
        <span v-for="plan in paymentPlans" :key="plan.id">
          #{{ plan.id }} {{ plan.name }} / group {{ plan.group_id }} / {{ plan.price }} / {{ plan.validity_days }}
          {{ plan.validity_unit }}
        </span>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
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

const editablePlans = computed(() => {
  const plans = props.plans as AdminPlan[]
  plans.forEach(ensurePlanDefaults)
  return plans
})

const paymentGroupOptions = computed(() => {
  const seen = new Set<number>()
  return props.paymentPlans
    .map(plan => ({ id: plan.group_id, label: `group ${plan.group_id} via ${plan.name}` }))
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
    name: 'New Codex++ plan',
    description: 'Draft Codex++ plan.',
    billing_period: 'monthly',
    currency: 'USD',
    price_amount_minor: 0,
    display_price: 'TBD',
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
  props.plans.push(plan)
}

function removePlan(index: number) {
  if (props.plans.length <= 1) return
  props.plans.splice(index, 1)
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

function nextSortOrder(): number {
  const orders = editablePlans.value.map(plan => plan.sort_order || 0)
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
.admin-table {
  @apply w-full min-w-[1880px] text-left text-sm;
}

.admin-table th {
  @apply border-b border-gray-200 px-2 py-2 text-xs font-semibold uppercase text-gray-500 dark:border-dark-700;
}

.admin-table td {
  @apply border-b border-gray-100 px-2 py-2 align-top dark:border-dark-800;
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
