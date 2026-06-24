<template>
  <section class="space-y-4">
    <form class="flex flex-wrap items-end gap-2" @submit.prevent="loadUser">
      <label class="space-y-1">
        <span class="text-xs font-medium text-gray-500">用户 ID</span>
        <input v-model.number="userId" class="input w-36" type="number" min="1" />
      </label>
      <button class="btn-primary" type="submit" :disabled="loading || !userId">
        {{ loading ? '查询中...' : '查询用户' }}
      </button>
      <button v-if="entitlement?.user" class="btn-secondary" type="button" :disabled="loading" @click="loadUser">
        刷新
      </button>
      <RouterLink v-if="userId" class="btn-secondary" to="/admin/users">用户列表</RouterLink>
      <RouterLink v-if="userId" class="btn-secondary" :to="`/admin/subscriptions?user_id=${userId}`">订阅记录</RouterLink>
      <RouterLink v-if="userId" class="btn-secondary" :to="`/admin/usage?user_id=${userId}`">使用记录</RouterLink>
    </form>

    <p v-if="error" class="error">{{ error }}</p>

    <template v-if="entitlement?.user">
      <div class="grid gap-3 md:grid-cols-3 xl:grid-cols-6">
        <div class="metric">
          <span>用户</span>
          <strong>#{{ entitlement.user.id }} {{ entitlement.user.email || entitlement.user.username }}</strong>
        </div>
        <div class="metric">
          <span>状态</span>
          <strong><span class="badge" :class="statusClass(entitlement.user.status)">{{ statusText(entitlement.user.status) }}</span></strong>
        </div>
        <div class="metric">
          <span>当前套餐</span>
          <strong>{{ currentPackageLabel }}</strong>
        </div>
        <div class="metric">
          <span>到期时间</span>
          <strong>{{ currentExpiryLabel }}</strong>
        </div>
        <div class="metric">
          <span>余额</span>
          <strong>{{ formatNumber(entitlement.user.balance, 2) }}</strong>
        </div>
        <div class="metric">
          <span>托管密钥</span>
          <strong>{{ managedKeyLabel }}</strong>
        </div>
      </div>

      <section class="panel">
        <div class="section-head">
          <h3 class="section-title">需要关注</h3>
          <span class="text-xs text-gray-500">服务端汇总</span>
        </div>
        <div class="flex flex-wrap gap-2">
          <span
            v-for="item in statusItems"
            :key="`${item.label}-${item.detail}`"
            class="status-pill"
            :class="toneClass(item.tone)"
          >
            <strong>{{ item.label }}</strong>
            <span v-if="item.detail">{{ item.detail }}</span>
          </span>
        </div>
      </section>

      <div class="grid gap-4 xl:grid-cols-[minmax(0,1.2fr)_minmax(0,0.8fr)]">
        <section class="panel">
          <div class="section-head">
            <h3 class="section-title">套餐记录</h3>
            <span class="text-xs text-gray-500">{{ activeSubscriptions.length }} 个生效 / 共 {{ subscriptions.length }} 个</span>
          </div>
          <div class="overflow-x-auto">
            <table class="admin-table min-w-[760px]">
              <thead>
                <tr>
                  <th>用户组</th>
                  <th>状态</th>
                  <th>时间</th>
                  <th>用量</th>
                  <th>备注</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="sub in subscriptionRows" :key="sub.id">
                  <td>
                    <strong>{{ sub.group_name || `用户组 #${sub.group_id}` }}</strong>
                    <span class="block text-xs text-gray-500">{{ sub.group_platform || '-' }} / #{{ sub.group_id }}</span>
                  </td>
                  <td><span class="badge" :class="statusClass(sub.status)">{{ statusText(sub.status) }}</span></td>
                  <td>
                    <span class="block">{{ formatDate(sub.starts_at) }}</span>
                    <span class="block text-xs text-gray-500">{{ formatExpiry(sub.expires_at) }}</span>
                  </td>
                  <td>
                    <span class="block">今日 {{ formatMoney(sub.daily_usage_usd) }}</span>
                    <span class="block text-xs text-gray-500">
                      本周 {{ formatMoney(sub.weekly_usage_usd) }} / 本月 {{ formatMoney(sub.monthly_usage_usd) }}
                    </span>
                  </td>
                  <td>{{ sub.notes || '-' }}</td>
                </tr>
                <tr v-if="!subscriptions.length"><td colspan="5">暂无套餐记录。</td></tr>
              </tbody>
            </table>
          </div>
        </section>

        <section class="panel">
          <div class="section-head">
            <h3 class="section-title">可用用户组和模型范围</h3>
            <span class="text-xs text-gray-500">已识别 {{ allowedGroups.length }} 个</span>
          </div>
          <div class="mb-3 flex flex-wrap gap-1.5">
            <span v-for="scope in modelScopes" :key="scope" class="chip">{{ scope }}</span>
            <span v-if="!modelScopes.length" class="text-xs text-gray-500">暂无模型范围。</span>
          </div>
          <div class="overflow-x-auto">
            <table class="admin-table min-w-[560px]">
              <thead>
                <tr>
                  <th>用户组</th>
                  <th>平台</th>
                  <th>类型</th>
                  <th>状态</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="group in allowedGroups" :key="group.id">
                  <td>
                    <strong>{{ group.name }}</strong>
                    <span class="block text-xs text-gray-500">#{{ group.id }}</span>
                  </td>
                  <td>{{ group.platform || '-' }}</td>
                  <td>{{ group.subscription_type || '-' }}</td>
                  <td><span class="badge" :class="statusClass(group.status)">{{ statusText(group.status) }}</span></td>
                </tr>
                <tr v-if="!allowedGroups.length && userAllowedGroupIds.length">
                  <td colspan="4">未识别的用户组 ID：{{ userAllowedGroupIds.join(', ') }}</td>
                </tr>
                <tr v-else-if="!allowedGroups.length">
                  <td colspan="4">暂无单独授权的用户组。</td>
                </tr>
              </tbody>
            </table>
          </div>
        </section>
      </div>

      <div class="grid gap-4 xl:grid-cols-2">
        <section class="panel">
          <div class="section-head">
            <h3 class="section-title">设备</h3>
            <span class="text-xs text-gray-500">已登记 {{ devices.length }} 台</span>
          </div>
          <div class="overflow-x-auto">
            <table class="admin-table min-w-[560px]">
              <thead>
                <tr>
                  <th>设备</th>
                  <th>状态</th>
                  <th>最后在线</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="device in devices" :key="device.device_id">
                  <td><code>{{ device.device_id }}</code></td>
                  <td><span class="badge" :class="statusClass(device.status)">{{ statusText(device.status) }}</span></td>
                  <td>{{ formatDate(device.last_seen) }}</td>
                </tr>
                <tr v-if="!devices.length"><td colspan="3">暂无设备。</td></tr>
              </tbody>
            </table>
          </div>
        </section>

        <section class="panel">
          <div class="section-head">
            <h3 class="section-title">密钥概况</h3>
            <span class="text-xs text-gray-500">{{ apiKeys.length }} 个用户密钥</span>
          </div>
          <div class="mb-3 grid gap-2 sm:grid-cols-2">
            <div class="summary-box">
              <span>托管供应密钥</span>
              <strong>{{ managedKeyLabel }}</strong>
            </div>
            <div class="summary-box">
              <span>托管密钥 ID</span>
              <strong>{{ managedKeyID }}</strong>
            </div>
          </div>
          <div class="overflow-x-auto">
            <table class="admin-table min-w-[640px]">
              <thead>
                <tr>
                  <th>ID</th>
                  <th>名称</th>
                  <th>用户组</th>
                  <th>状态</th>
                  <th>脱敏密钥</th>
                  <th>最后使用</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="key in apiKeys" :key="key.id">
                  <td>{{ key.id }}</td>
                  <td>{{ key.name || '-' }}</td>
                  <td>{{ key.group_id || '-' }}</td>
                  <td><span class="badge" :class="statusClass(key.status)">{{ statusText(key.status) }}</span></td>
                  <td><code>{{ safeMaskedKey(key.masked_key) }}</code></td>
                  <td>{{ formatDate(key.last_used_at) }}</td>
                </tr>
                <tr v-if="!apiKeys.length"><td colspan="6">暂无 API 密钥。</td></tr>
              </tbody>
            </table>
          </div>
        </section>
      </div>

      <div class="grid gap-4 xl:grid-cols-[minmax(0,0.8fr)_minmax(0,1.2fr)]">
        <section class="panel">
          <div class="section-head">
            <h3 class="section-title">近期用量</h3>
            <span class="text-xs text-gray-500">近 30 天汇总</span>
          </div>
          <div v-if="usageMetrics.length" class="grid gap-2 sm:grid-cols-2">
            <div v-for="metric in usageMetrics" :key="metric.label" class="summary-box">
              <span>{{ metric.label }}</span>
              <strong>{{ metric.value }}</strong>
            </div>
          </div>
          <p v-else class="text-sm text-gray-500">暂无用量汇总。</p>
        </section>

        <section class="panel">
          <div class="section-head">
            <h3 class="section-title">最近事件</h3>
            <span class="text-xs text-gray-500">{{ recentEvents.length }} 条</span>
          </div>
          <div class="overflow-x-auto">
            <table class="admin-table min-w-[640px]">
              <thead>
                <tr>
                  <th>时间</th>
                  <th>类型</th>
                  <th>说明</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="event in recentEvents" :key="`${event.created_at}-${event.event_type}-${event.summary}`">
                  <td>{{ formatDate(event.created_at) }}</td>
                  <td><span class="badge" :class="statusClass(event.event_type)">{{ statusText(event.event_type) }}</span></td>
                  <td>{{ event.summary || '-' }}</td>
                </tr>
                <tr v-if="!recentEvents.length"><td colspan="3">暂无最近事件。</td></tr>
              </tbody>
            </table>
          </div>
        </section>
      </div>

      <section v-if="integrationEntries.length" class="panel">
        <div class="section-head">
          <h3 class="section-title">接入状态</h3>
        </div>
        <div class="flex flex-wrap gap-2">
          <span v-for="[key, value] in integrationEntries" :key="key" class="status-pill" :class="statusClass(value)">
            <strong>{{ key }}</strong>
            <span>{{ statusText(value) }}</span>
          </span>
        </div>
      </section>
    </template>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import {
  getCodexPlusUserEntitlement,
  type CodexPlusSubscriptionSummary,
  type CodexPlusUserEntitlement
} from '@/api/admin/codexPlus'

type AlertTone = 'ok' | 'warn' | 'danger' | 'neutral'
type UnknownRecord = Record<string, unknown>

interface StatusItem {
  label: string
  detail: string
  tone: AlertTone
}

interface DisplayMetric {
  label: string
  value: string
}

const route = useRoute()
const userId = ref<number | null>(null)
const loading = ref(false)
const error = ref('')
const entitlement = ref<CodexPlusUserEntitlement | null>(null)

const subscriptions = computed(() => entitlement.value?.subscriptions ?? [])
const responseActiveSubscriptions = computed(() => entitlement.value?.active_subscriptions ?? [])
const allowedGroups = computed(() => entitlement.value?.allowed_groups ?? [])
const userAllowedGroupIds = computed(() => entitlement.value?.user?.allowed_group_ids ?? [])
const apiKeys = computed(() => entitlement.value?.api_keys ?? [])
const devices = computed(() => entitlement.value?.devices ?? [])
const recentEvents = computed(() => entitlement.value?.recent_events ?? [])

const activeSubscriptions = computed(() => {
  const source = responseActiveSubscriptions.value.length ? responseActiveSubscriptions.value : subscriptions.value.filter(isSubscriptionActive)
  return [...source].sort(compareSubscriptionExpiry)
})

const subscriptionRows = computed(() => {
  return [...subscriptions.value].sort((left, right) => {
    const leftActive = isSubscriptionActive(left) ? 0 : 1
    const rightActive = isSubscriptionActive(right) ? 0 : 1
    if (leftActive !== rightActive) return leftActive - rightActive
    return compareSubscriptionExpiry(left, right)
  })
})

const currentSubscription = computed(() => activeSubscriptions.value[0] ?? null)

const currentPackageLabel = computed(() => {
  const sub = currentSubscription.value
  if (!sub) return '无生效套餐'
  return sub.group_name || `用户组 #${sub.group_id}`
})

const currentExpiryLabel = computed(() => {
  const sub = currentSubscription.value
  return sub ? formatExpiry(sub.expires_at) : '-'
})

const managedKeyLabel = computed(() => {
  const key = entitlement.value?.managed_provider_key
  if (!key?.exists) return '未配置'
  return safeMaskedKey(key.masked_key)
})

const managedKeyID = computed(() => entitlement.value?.managed_provider_key?.key_id || '-')

const modelScopes = computed(() => {
  return Array.from(new Set(allowedGroups.value.flatMap(group => group.supported_model_scopes ?? []))).sort()
})

const integrationEntries = computed(() => Object.entries(entitlement.value?.integration_status ?? {}))

const usageMetrics = computed<DisplayMetric[]>(() => {
  const stats = asRecord(entitlement.value?.usage_summary)
  if (!stats) return []

  return [
    { label: '周期', value: formatScalar(stats.period || '30d') },
    { label: '请求数', value: formatNumber(stats.total_requests) },
    { label: '令牌数', value: formatNumber(stats.total_tokens) },
    { label: '实际成本', value: formatMoney(stats.total_actual_cost ?? stats.total_cost) },
    { label: '标准成本', value: formatMoney(stats.total_cost) },
    { label: '平均耗时', value: formatDuration(stats.average_duration_ms ?? stats.avg_duration_ms) }
  ].filter(metric => metric.value !== '-')
})

const statusItems = computed<StatusItem[]>(() => {
  const current = entitlement.value
  if (!current?.user) return []

  const items: StatusItem[] = []
  const userStatus = normalize(current.user.status)
  if (userStatus && userStatus !== 'active') {
    items.push({ label: '用户状态', detail: statusText(current.user.status), tone: 'danger' })
  }
  if (Number(current.user.balance) <= 0) {
    items.push({ label: '余额', detail: formatNumber(current.user.balance, 2), tone: 'warn' })
  }
  if (!activeSubscriptions.value.length) {
    items.push({ label: '套餐', detail: '没有生效套餐', tone: 'warn' })
  }
  if (!current.managed_provider_key?.exists) {
    items.push({ label: '托管密钥', detail: '未配置', tone: 'warn' })
  }

  const revokedDevices = devices.value.filter(device => isRiskStatus(device.status))
  if (revokedDevices.length) {
    items.push({ label: '设备', detail: `${revokedDevices.length} 台异常`, tone: 'danger' })
  }

  const unavailableIntegrations = integrationEntries.value.filter(([, value]) => !isHealthyStatus(value))
  if (unavailableIntegrations.length) {
    items.push({ label: '接入状态', detail: unavailableIntegrations.map(([key]) => key).join(', '), tone: 'warn' })
  }

  const errorEvents = recentEvents.value.filter(event => isRiskStatus(`${event.event_type} ${event.summary}`))
  if (errorEvents.length) {
    items.push({ label: '最近事件', detail: `${errorEvents.length} 条疑似异常`, tone: 'warn' })
  }

  if (!items.length) {
    items.push({ label: '正常', detail: '没有需要处理的状态', tone: 'ok' })
  }
  return items
})

onMounted(() => {
  const initialUserID = firstNumber(route.query.user_id)
  if (initialUserID) {
    userId.value = initialUserID
    void loadUser()
  }
})

async function loadUser() {
  if (!userId.value) return
  loading.value = true
  error.value = ''
  try {
    entitlement.value = await getCodexPlusUserEntitlement(userId.value)
  } catch (err) {
    error.value = (err as { message?: string }).message || '查询用户授权失败'
    entitlement.value = null
  } finally {
    loading.value = false
  }
}

function firstNumber(value: unknown): number | null {
  const raw = Array.isArray(value) ? value[0] : value
  const parsed = Number(raw)
  return Number.isFinite(parsed) && parsed > 0 ? parsed : null
}

function compareSubscriptionExpiry(left: CodexPlusSubscriptionSummary, right: CodexPlusSubscriptionSummary) {
  return timestamp(left.expires_at) - timestamp(right.expires_at)
}

function isSubscriptionActive(sub: CodexPlusSubscriptionSummary) {
  return normalize(sub.status) === 'active' && !isExpired(sub.expires_at)
}

function isExpired(value?: string | null) {
  const time = timestamp(value)
  return time > 0 && time < Date.now()
}

function timestamp(value?: string | null) {
  if (!value) return 0
  const parsed = Date.parse(value)
  return Number.isFinite(parsed) ? parsed : 0
}

function formatDate(value?: string | null) {
  if (!value) return '-'
  const parsed = Date.parse(value)
  if (!Number.isFinite(parsed)) return value
  return new Date(parsed).toLocaleString()
}

function formatExpiry(value?: string | null) {
  if (!value) return '-'
  const parsed = timestamp(value)
  if (!parsed) return value
  return `${formatDate(value)} (${relativeExpiry(parsed)})`
}

function relativeExpiry(time: number) {
  const diff = time - Date.now()
  const abs = Math.abs(diff)
  const day = 24 * 60 * 60 * 1000
  const hour = 60 * 60 * 1000
  const value = abs >= day ? `${Math.ceil(abs / day)}d` : `${Math.ceil(abs / hour)}h`
  return diff >= 0 ? `剩余 ${value}` : `已过期 ${value}`
}

function formatScalar(value: unknown) {
  if (value === null || value === undefined || value === '') return '-'
  if (typeof value === 'number') return formatNumber(value)
  return String(value)
}

function formatNumber(value: unknown, fractionDigits = 0) {
  const number = Number(value)
  if (!Number.isFinite(number)) return '-'
  return new Intl.NumberFormat(undefined, {
    minimumFractionDigits: fractionDigits,
    maximumFractionDigits: fractionDigits
  }).format(number)
}

function formatMoney(value: unknown) {
  const number = Number(value)
  if (!Number.isFinite(number)) return '-'
  return new Intl.NumberFormat(undefined, {
    style: 'currency',
    currency: 'USD',
    maximumFractionDigits: 4
  }).format(number)
}

function formatDuration(value: unknown) {
  const number = Number(value)
  if (!Number.isFinite(number)) return '-'
  return `${formatNumber(number, number >= 10 ? 0 : 1)} ms`
}

function asRecord(value: unknown): UnknownRecord | null {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return null
  return value as UnknownRecord
}

function safeMaskedKey(value?: string) {
  const trimmed = (value || '').trim()
  if (!trimmed) return '已隐藏'
  if (trimmed.includes('*') || trimmed.includes('...') || trimmed.includes('…') || trimmed.length <= 16) {
    return trimmed
  }
  return `${trimmed.slice(0, 6)}...${trimmed.slice(-4)}`
}

function normalize(value?: string) {
  return (value || '').trim().toLowerCase()
}

function isHealthyStatus(value?: string) {
  const status = normalize(value)
  return ['active', 'ok', 'loaded', 'enabled', 'available', 'paid', 'success'].some(item => status.includes(item))
}

function isRiskStatus(value?: string) {
  const status = normalize(value)
  return ['revoked', 'expired', 'disabled', 'error', 'failed', 'unavailable', 'blocked', 'deleted', 'inactive', 'denied'].some(item =>
    status.includes(item)
  )
}

function statusClass(status?: string) {
  if (isRiskStatus(status)) return 'badge-red'
  if (isHealthyStatus(status)) return 'badge-green'
  const normalized = normalize(status)
  if (['pending', 'paused', 'warning', 'warn', 'limited'].some(item => normalized.includes(item))) return 'badge-yellow'
  return 'badge-gray'
}

function statusText(value: unknown) {
  const raw = String(value || '').trim()
  const labels: Record<string, string> = {
    active: '正常',
    ok: '正常',
    loaded: '已加载',
    enabled: '已启用',
    available: '可用',
    paid: '已支付',
    success: '成功',
    pending: '等待中',
    paused: '已暂停',
    warning: '需关注',
    warn: '需关注',
    limited: '受限',
    revoked: '已撤销',
    expired: '已过期',
    disabled: '已停用',
    error: '错误',
    failed: '失败',
    unavailable: '不可用',
    blocked: '已阻止',
    deleted: '已删除',
    inactive: '未启用',
    denied: '已拒绝'
  }
  return labels[raw.toLowerCase()] || raw || '-'
}

function toneClass(tone: AlertTone) {
  if (tone === 'ok') return 'tone-ok'
  if (tone === 'warn') return 'tone-warn'
  if (tone === 'danger') return 'tone-danger'
  return 'tone-neutral'
}
</script>

<style scoped>
.input {
  @apply rounded-md border border-gray-300 bg-white px-3 py-2 text-sm dark:border-dark-600 dark:bg-dark-800;
}

.btn-primary {
  @apply rounded-md bg-primary-600 px-3 py-2 text-sm font-medium text-white hover:bg-primary-700 disabled:cursor-not-allowed disabled:opacity-50;
}

.btn-secondary {
  @apply rounded-md border border-gray-300 px-3 py-2 text-sm hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-50 dark:border-dark-600 dark:hover:bg-dark-800;
}

.panel {
  @apply rounded-md border border-gray-200 bg-white p-3 dark:border-dark-700 dark:bg-dark-900;
}

.section-head {
  @apply mb-2 flex flex-wrap items-center justify-between gap-2;
}

.section-title {
  @apply text-sm font-semibold text-gray-900 dark:text-white;
}

.metric {
  @apply rounded-md border border-gray-200 bg-white px-3 py-2 dark:border-dark-700 dark:bg-dark-900;
}

.metric span {
  @apply block text-xs text-gray-500;
}

.metric strong {
  @apply break-words text-sm text-gray-900 dark:text-white;
}

.summary-box {
  @apply rounded-md border border-gray-100 bg-gray-50 px-3 py-2 dark:border-dark-800 dark:bg-dark-950;
}

.summary-box span {
  @apply block text-xs text-gray-500;
}

.summary-box strong {
  @apply break-words text-sm text-gray-900 dark:text-white;
}

.admin-table {
  @apply w-full text-left text-sm;
}

.admin-table th {
  @apply border-b border-gray-200 px-2 py-2 text-xs font-semibold uppercase text-gray-500 dark:border-dark-700;
}

.admin-table td {
  @apply border-b border-gray-100 px-2 py-2 align-top dark:border-dark-800;
}

.badge {
  @apply inline-flex rounded px-1.5 py-0.5 text-xs font-medium;
}

.badge-green {
  @apply bg-green-50 text-green-700 dark:bg-green-950 dark:text-green-200;
}

.badge-yellow {
  @apply bg-yellow-50 text-yellow-700 dark:bg-yellow-950 dark:text-yellow-200;
}

.badge-red {
  @apply bg-red-50 text-red-700 dark:bg-red-950 dark:text-red-200;
}

.badge-gray {
  @apply bg-gray-100 text-gray-700 dark:bg-dark-800 dark:text-gray-200;
}

.chip {
  @apply rounded border border-primary-200 bg-primary-50 px-2 py-1 text-xs font-medium text-primary-700 dark:border-primary-900 dark:bg-primary-950 dark:text-primary-200;
}

.status-pill {
  @apply inline-flex items-center gap-2 rounded-md border px-2.5 py-1.5 text-xs;
}

.status-pill strong {
  @apply font-semibold;
}

.tone-ok {
  @apply border-green-200 bg-green-50 text-green-700 dark:border-green-900 dark:bg-green-950 dark:text-green-200;
}

.tone-warn {
  @apply border-yellow-200 bg-yellow-50 text-yellow-700 dark:border-yellow-900 dark:bg-yellow-950 dark:text-yellow-200;
}

.tone-danger {
  @apply border-red-200 bg-red-50 text-red-700 dark:border-red-900 dark:bg-red-950 dark:text-red-200;
}

.tone-neutral {
  @apply border-gray-200 bg-gray-50 text-gray-700 dark:border-dark-700 dark:bg-dark-800 dark:text-gray-200;
}

.error {
  @apply rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700 dark:border-red-900 dark:bg-red-950 dark:text-red-200;
}
</style>
