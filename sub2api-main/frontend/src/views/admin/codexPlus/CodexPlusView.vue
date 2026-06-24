<template>
  <AppLayout>
    <TablePageLayout>
      <template #actions>
        <div class="flex flex-wrap items-start justify-between gap-4">
          <div>
            <div class="mb-1 text-sm font-medium text-gray-500 dark:text-dark-300">管理 / Codex++</div>
            <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">Codex++</h1>
            <p class="mt-1 max-w-3xl text-sm text-gray-600 dark:text-dark-300">
              统一管理客户端看到的套餐、模型、额度和发布版本。先在列表确认，再进入对应分区编辑。
            </p>
          </div>

          <div class="flex flex-wrap items-center justify-end gap-2">
            <button class="btn btn-secondary" type="button" :disabled="loading" @click="loadAll">
              <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
              <span>刷新</span>
            </button>
            <button class="btn btn-secondary" type="button" :disabled="!draft || validating" @click="validateServer">
              <Icon name="checkCircle" size="sm" />
              <span>{{ validating ? '检查中' : '检查配置' }}</span>
            </button>
            <button class="btn btn-primary" type="button" :disabled="!canPublish" @click="publish">
              <Icon name="upload" size="sm" />
              <span>{{ publishing ? '发布中' : '发布配置' }}</span>
            </button>
          </div>
        </div>
      </template>

      <template #filters>
        <div class="space-y-4">
          <div v-if="draft" class="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
            <div class="metric">
              <span>套餐</span>
              <strong>{{ listedPlans.length }} / {{ draft.plan_catalog.plans.length }}</strong>
              <small>已上架 / 全部</small>
            </div>
            <div class="metric">
              <span>模型</span>
              <strong>{{ availableModels.length }} / {{ draft.model_catalog.models.length }}</strong>
              <small>可用 / 全部，默认：{{ defaultModelLabel }}</small>
            </div>
            <div class="metric">
              <span>用量规则</span>
              <strong>{{ draft.usage_policy.policies.length }}</strong>
              <small>服务端统一执行</small>
            </div>
            <div class="metric">
              <span>当前版本</span>
              <strong>{{ draft.config_version || '-' }}</strong>
              <small>{{ scopeLabel(draft.publish_scope) }} · {{ formatDate(draft.updated_at) }}</small>
            </div>
          </div>

          <div v-if="error" class="alert alert-error">{{ error }}</div>
          <div v-if="validationErrors.length" class="alert alert-error">
            <div v-for="item in validationErrors" :key="item">{{ item }}</div>
          </div>
          <div v-if="message" class="alert alert-info">{{ message }}</div>

          <nav v-if="draft" class="tabbar" aria-label="Codex++ 管理分区">
            <button
              v-for="tab in tabs"
              :key="tab.key"
              class="tab-button"
              :class="{ active: activeTab === tab.key }"
              type="button"
              @click="activeTab = tab.key"
            >
              <Icon :name="tab.icon" size="sm" />
              <span>{{ tab.label }}</span>
              <small v-if="tab.count !== undefined">{{ tab.count }}</small>
            </button>
          </nav>
        </div>
      </template>

      <template #table>
        <div class="table-wrapper">
          <div v-if="loading && !draft" class="state-box">
            <Icon name="refresh" size="lg" class="animate-spin text-primary-500" />
            <span>正在读取 Codex++ 管理配置</span>
          </div>

          <div v-else-if="!draft && error" class="state-box">
            <Icon name="exclamationCircle" size="lg" class="text-red-500" />
            <span>{{ error }}</span>
          </div>

          <template v-else-if="draft">
            <div v-if="activeTab === 'overview'" class="overview">
              <section class="list-section">
                <div class="section-head">
                  <div>
                    <h2>套餐列表</h2>
                    <p>决定用户购买后能看到什么、得到多少余额和可用哪些模型。</p>
                  </div>
                  <button class="btn btn-secondary" type="button" @click="activeTab = 'plans'">管理套餐</button>
                </div>
                <table class="overview-table">
                  <thead>
                    <tr>
                      <th>套餐</th>
                      <th>状态</th>
                      <th>价格</th>
                      <th>权益</th>
                      <th>模型分组</th>
                      <th>购买入口</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-for="plan in planRows" :key="plan.plan_id">
                      <td>
                        <div class="font-medium text-gray-900 dark:text-white">{{ plan.name || plan.plan_id }}</div>
                        <div class="muted">{{ plan.plan_id }}</div>
                      </td>
                      <td><span class="status" :class="planStatusClass(plan)">{{ planStatusLabel(plan) }}</span></td>
                      <td>
                        <div>{{ plan.display_price || formatMinorPrice(plan) }}</div>
                        <div class="muted">{{ periodLabel(plan.billing_period) }}</div>
                      </td>
                      <td>
                        <div>{{ formatNumber(plan.entitlement_grant.balance_credit) }} 余额</div>
                        <div class="muted">{{ plan.entitlement_grant.duration_days }} 天</div>
                      </td>
                      <td>
                        <div class="chip-line">
                          <span v-for="group in plan.model_groups" :key="group" class="chip">{{ group }}</span>
                          <span v-if="!plan.model_groups.length" class="muted">未设置</span>
                        </div>
                      </td>
                      <td>{{ plan.purchase_url ? '已配置' : '未配置' }}</td>
                    </tr>
                    <tr v-if="!planRows.length">
                      <td colspan="6" class="empty-cell">还没有配置套餐。</td>
                    </tr>
                  </tbody>
                </table>
              </section>

              <section class="list-section">
                <div class="section-head">
                  <div>
                    <h2>模型列表</h2>
                    <p>客户端能展示和启动的模型，默认模型必须可用。</p>
                  </div>
                  <button class="btn btn-secondary" type="button" @click="activeTab = 'models'">管理模型</button>
                </div>
                <table class="overview-table">
                  <thead>
                    <tr>
                      <th>模型</th>
                      <th>状态</th>
                      <th>路由</th>
                      <th>分组</th>
                      <th>容量</th>
                      <th>倍率</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-for="model in modelRows" :key="model.model_id">
                      <td>
                        <div class="font-medium text-gray-900 dark:text-white">{{ model.display_name || model.model_id }}</div>
                        <div class="muted">{{ model.model_id }}</div>
                      </td>
                      <td>
                        <span class="status" :class="modelStatusClass(model)">{{ modelStatusLabel(model) }}</span>
                        <span v-if="model.is_default" class="status status-default">默认</span>
                      </td>
                      <td>{{ model.route_model || '-' }}</td>
                      <td>{{ model.model_group || '-' }}</td>
                      <td>{{ formatNumber(model.context_window) }}</td>
                      <td>{{ model.billing_multiplier }}</td>
                    </tr>
                    <tr v-if="!modelRows.length">
                      <td colspan="6" class="empty-cell">还没有配置模型。</td>
                    </tr>
                  </tbody>
                </table>
              </section>

              <section class="list-section">
                <div class="section-head">
                  <div>
                    <h2>用量规则</h2>
                    <p>控制余额提醒、每日额度、并发和速率限制。</p>
                  </div>
                  <button class="btn btn-secondary" type="button" @click="activeTab = 'usage'">管理规则</button>
                </div>
                <table class="overview-table">
                  <thead>
                    <tr>
                      <th>规则</th>
                      <th>适用套餐</th>
                      <th>额度</th>
                      <th>并发</th>
                      <th>每分钟请求</th>
                      <th>过期处理</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-for="policy in policyRows" :key="policy.policy_id">
                      <td><code>{{ policy.policy_id }}</code></td>
                      <td>{{ formatScope(policy.applies_to?.plan_ids) }}</td>
                      <td>
                        <div>每日 {{ formatNumber(policy.daily_quota) }}</div>
                        <div class="muted">每月 {{ formatNullableNumber(policy.monthly_quota) }}</div>
                      </td>
                      <td>{{ policy.concurrency_limit }}</td>
                      <td>{{ formatNumber(policy.rpm_limit) }}</td>
                      <td>{{ expiredBehaviorLabel(policy.expired_behavior) }}</td>
                    </tr>
                    <tr v-if="!policyRows.length">
                      <td colspan="6" class="empty-cell">还没有配置用量规则。</td>
                    </tr>
                  </tbody>
                </table>
              </section>

              <section class="list-section">
                <div class="section-head">
                  <div>
                    <h2>发布记录</h2>
                    <p>最近发布和回滚记录，方便确认当前客户端拿到的配置来源。</p>
                  </div>
                  <button class="btn btn-secondary" type="button" @click="activeTab = 'versions'">查看全部</button>
                </div>
                <table class="overview-table">
                  <thead>
                    <tr>
                      <th>版本</th>
                      <th>范围</th>
                      <th>更新时间</th>
                      <th>操作人</th>
                      <th>说明</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-for="entry in versionRows" :key="entry.config_version">
                      <td><code>{{ entry.config_version }}</code></td>
                      <td>{{ scopeLabel(entry.publish_scope) }}</td>
                      <td>{{ formatDate(entry.updated_at) }}</td>
                      <td>{{ entry.updated_by || '-' }}</td>
                      <td>{{ versionReason(entry) }}</td>
                    </tr>
                    <tr v-if="!versionRows.length">
                      <td colspan="5" class="empty-cell">还没有版本记录。</td>
                    </tr>
                  </tbody>
                </table>
              </section>
            </div>

            <div v-else class="panel-shell">
              <CodexPlusConfigPanel
                v-if="activeTab === 'settings'"
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
            </div>
          </template>
        </div>
      </template>
    </TablePageLayout>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import {
  getCodexPlusConfig,
  getCodexPlusOptions,
  listCodexPlusConfigVersions,
  publishCodexPlusConfig,
  rollbackCodexPlusConfig,
  validateCodexPlusConfig,
  type CodexPlusConfig,
  type CodexPlusModel,
  type CodexPlusOptions,
  type CodexPlusPlan,
  type CodexPlusUsageRule,
  type CodexPlusVersionEntry
} from '@/api/admin/codexPlus'
import CodexPlusConfigPanel from './CodexPlusConfigPanel.vue'
import CodexPlusPlanCatalogPanel from './CodexPlusPlanCatalogPanel.vue'
import CodexPlusModelCatalogPanel from './CodexPlusModelCatalogPanel.vue'
import CodexPlusUsagePolicyPanel from './CodexPlusUsagePolicyPanel.vue'
import CodexPlusFeatureFlagsPanel from './CodexPlusFeatureFlagsPanel.vue'
import CodexPlusUserEntitlementPanel from './CodexPlusUserEntitlementPanel.vue'
import CodexPlusConfigVersionsPanel from './CodexPlusConfigVersionsPanel.vue'

type TabKey = 'overview' | 'plans' | 'models' | 'usage' | 'flags' | 'users' | 'versions' | 'settings'
type IconName = InstanceType<typeof Icon>['$props']['name']

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
const canPublish = computed(() => !!draft.value && validationErrors.value.length === 0 && !publishing.value && !validating.value)

const listedPlans = computed(() => {
  return draft.value?.plan_catalog.plans.filter(plan => plan.status === 'active' && plan.is_listed) ?? []
})

const availableModels = computed(() => {
  return draft.value?.model_catalog.models.filter(model => model.is_enabled && !model.is_hidden) ?? []
})

const defaultModelLabel = computed(() => {
  const defaults = draft.value?.model_catalog.models.filter(model => model.is_default) ?? []
  if (defaults.length === 0) return '未设置'
  if (defaults.length > 1) return '重复'
  return defaults[0].display_name || defaults[0].model_id || '未命名'
})

const planRows = computed(() => {
  return [...(draft.value?.plan_catalog.plans ?? [])].sort((left, right) => {
    return (left.sort_order ?? 0) - (right.sort_order ?? 0) || left.name.localeCompare(right.name)
  })
})

const modelRows = computed(() => {
  return [...(draft.value?.model_catalog.models ?? [])].sort((left, right) => {
    return (left.sort_order ?? 0) - (right.sort_order ?? 0) || left.model_id.localeCompare(right.model_id)
  })
})

const policyRows = computed(() => draft.value?.usage_policy.policies ?? [])
const versionRows = computed(() => versions.value.slice(0, 6))

const tabs = computed<Array<{ key: TabKey; label: string; icon: IconName; count?: number }>>(() => [
  { key: 'overview', label: '总览', icon: 'grid' },
  { key: 'plans', label: '套餐', icon: 'creditCard', count: draft.value?.plan_catalog.plans.length },
  { key: 'models', label: '模型', icon: 'brain', count: draft.value?.model_catalog.models.length },
  { key: 'usage', label: '用量规则', icon: 'calculator', count: draft.value?.usage_policy.policies.length },
  { key: 'flags', label: '功能开关', icon: 'shield' },
  { key: 'users', label: '用户查询', icon: 'users' },
  { key: 'versions', label: '版本记录', icon: 'clock', count: versions.value.length },
  { key: 'settings', label: '发布设置', icon: 'cog' }
])

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
    error.value = (err as { message?: string }).message || '读取 Codex++ 管理配置失败'
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
    message.value = '配置检查通过，可以发布。'
  } catch (err) {
    error.value = (err as { message?: string }).message || '配置检查失败'
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
    message.value = `已发布新配置：${published.config_version}。`
  } catch (err) {
    error.value = (err as { message?: string }).message || '发布失败'
  } finally {
    publishing.value = false
  }
}

async function rollback(version: string) {
  if (!window.confirm(`确定恢复到版本 ${version} 吗？系统会生成一个新的发布版本。`)) return
  error.value = ''
  message.value = ''
  try {
    const rolledBack = await rollbackCodexPlusConfig(version, `rollback from ${version}`)
    draft.value = clone(rolledBack)
    await loadVersions()
    message.value = `已恢复并发布为新版本：${rolledBack.config_version}。`
  } catch (err) {
    error.value = (err as { message?: string }).message || '恢复版本失败'
  }
}

function validateDraft(config: CodexPlusConfig): string[] {
  const errors: string[] = []
  const planIds = new Set<string>()
  for (const plan of config.plan_catalog.plans) {
    if (!plan.plan_id) errors.push('每个套餐都需要填写套餐编号。')
    if (plan.plan_id && planIds.has(plan.plan_id)) errors.push(`套餐编号重复：${plan.plan_id}。`)
    planIds.add(plan.plan_id)
    if (!plan.name) errors.push(`套餐 ${plan.plan_id || '新套餐'} 需要填写名称。`)
    if (!plan.model_groups.length) errors.push(`套餐 ${plan.plan_id || '新套餐'} 至少要关联一个模型分组。`)
    if (plan.entitlement_grant.balance_credit < 0 || plan.entitlement_grant.duration_days < 0) {
      errors.push(`套餐 ${plan.plan_id || '新套餐'} 的余额或有效期不能小于 0。`)
    }
  }

  const modelIds = new Set<string>()
  const defaults = config.model_catalog.models.filter(model => model.is_default)
  if (defaults.length !== 1) errors.push('必须且只能设置一个默认模型。')
  for (const model of config.model_catalog.models) {
    if (!model.model_id) errors.push('每个模型都需要填写模型编号。')
    if (model.model_id && modelIds.has(model.model_id)) errors.push(`模型编号重复：${model.model_id}。`)
    modelIds.add(model.model_id)
    if (model.is_default && !model.is_enabled) errors.push('默认模型必须处于可用状态。')
    if (model.context_window < 1024) errors.push(`模型 ${model.model_id} 的上下文容量至少为 1024。`)
    if (model.billing_multiplier <= 0) errors.push(`模型 ${model.model_id} 的计费倍率必须大于 0。`)
  }

  for (const policy of config.usage_policy.policies) {
    if (!policy.policy_id) errors.push('每条用量规则都需要填写规则编号。')
    if (policy.concurrency_limit < 1 || policy.rpm_limit < 1 || policy.tpm_limit < 1) {
      errors.push(`用量规则 ${policy.policy_id || '新规则'} 的限制数值必须大于 0。`)
    }
    if (!policy.insufficient_balance_message) {
      errors.push(`用量规则 ${policy.policy_id || '新规则'} 需要填写余额不足提示。`)
    }
  }
  return Array.from(new Set(errors))
}

function planStatusLabel(plan: CodexPlusPlan): string {
  if (plan.status === 'active' && plan.is_listed) return '已上架'
  if (plan.status === 'active') return '内部可用'
  if (plan.status === 'hidden') return '隐藏'
  if (plan.status === 'disabled') return '停用'
  return plan.status || '-'
}

function planStatusClass(plan: CodexPlusPlan): string {
  if (plan.status === 'active' && plan.is_listed) return 'status-ok'
  if (plan.status === 'active') return 'status-info'
  if (plan.status === 'hidden') return 'status-muted'
  return 'status-danger'
}

function modelStatusLabel(model: CodexPlusModel): string {
  if (model.is_enabled && !model.is_hidden) return '可用'
  if (model.is_enabled && model.is_hidden) return '隐藏'
  if (!model.is_enabled && model.is_hidden) return '下架'
  return '停用'
}

function modelStatusClass(model: CodexPlusModel): string {
  if (model.is_enabled && !model.is_hidden) return 'status-ok'
  if (model.is_enabled && model.is_hidden) return 'status-info'
  return 'status-danger'
}

function formatMinorPrice(plan: CodexPlusPlan): string {
  if (!plan.currency) return String(plan.price_amount_minor ?? 0)
  return `${plan.currency} ${((plan.price_amount_minor || 0) / 100).toFixed(2)}`
}

function formatNumber(value: number | null | undefined): string {
  if (typeof value !== 'number' || Number.isNaN(value)) return '-'
  return value.toLocaleString()
}

function formatNullableNumber(value: number | null | undefined): string {
  if (typeof value !== 'number') return '不限制'
  if (value === 0) return '不限制'
  return formatNumber(value)
}

function formatScope(values: string[] | undefined): string {
  if (!values?.length) return '全部'
  return values.join(', ')
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

function expiredBehaviorLabel(value: CodexPlusUsageRule['expired_behavior']): string {
  const labels: Record<string, string> = {
    block: '停止使用',
    degrade: '降级使用',
    allow_grace_period: '宽限期'
  }
  return labels[value] || value || '-'
}

function scopeLabel(value: string): string {
  const labels: Record<string, string> = {
    draft: '草稿',
    internal: '内部测试',
    canary: '小范围灰度',
    production: '正式发布'
  }
  return labels[value] || value || '-'
}

function formatDate(value?: string): string {
  if (!value) return '-'
  return new Date(value).toLocaleString()
}

function versionReason(entry: CodexPlusVersionEntry): string {
  if (entry.rollback_from) return `从 ${entry.rollback_from} 恢复`
  const reason = (entry.change_reason || '').trim()
  if (!reason) return '未填写'
  if (/initial hidden Codex\+\+ MVP config/i.test(reason)) return '系统初始化配置'
  if (/rollback from/i.test(reason)) return reason.replace(/rollback from/i, '从').replace(/$/, ' 恢复')
  return reason
}

function clone<T>(value: T): T {
  return JSON.parse(JSON.stringify(value)) as T
}
</script>

<style scoped>
.metric {
  @apply rounded-lg border border-gray-200 bg-white px-4 py-3 shadow-sm dark:border-dark-700 dark:bg-dark-800;
}

.metric span {
  @apply block text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-dark-300;
}

.metric strong {
  @apply mt-1 block text-xl font-semibold text-gray-900 dark:text-white;
}

.metric small {
  @apply mt-1 block truncate text-xs text-gray-500 dark:text-dark-300;
}

.alert {
  @apply rounded-lg border px-4 py-3 text-sm;
}

.alert-error {
  @apply border-red-200 bg-red-50 text-red-700 dark:border-red-900 dark:bg-red-950/40 dark:text-red-200;
}

.alert-info {
  @apply border-blue-200 bg-blue-50 text-blue-700 dark:border-blue-900 dark:bg-blue-950/40 dark:text-blue-200;
}

.tabbar {
  @apply flex flex-wrap items-center gap-2;
}

.tab-button {
  @apply inline-flex items-center gap-2 rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm font-medium text-gray-600 shadow-sm transition-colors hover:bg-gray-50 dark:border-dark-700 dark:bg-dark-800 dark:text-dark-300 dark:hover:bg-dark-700;
}

.tab-button.active {
  @apply border-primary-200 bg-primary-50 text-primary-700 dark:border-primary-900 dark:bg-primary-950/40 dark:text-primary-200;
}

.tab-button small {
  @apply rounded bg-gray-100 px-1.5 py-0.5 text-[11px] text-gray-500 dark:bg-dark-700 dark:text-dark-300;
}

.overview {
  @apply space-y-6 p-5;
}

.panel-shell {
  @apply p-5;
}

.state-box {
  @apply flex h-full min-h-[320px] flex-col items-center justify-center gap-3 text-sm text-gray-500 dark:text-dark-300;
}

.list-section {
  @apply space-y-3;
}

.section-head {
  @apply flex flex-wrap items-start justify-between gap-3;
}

.section-head h2 {
  @apply text-base font-semibold text-gray-900 dark:text-white;
}

.section-head p {
  @apply mt-1 text-sm text-gray-500 dark:text-dark-300;
}

.overview-table {
  @apply w-full min-w-[920px] text-left text-sm;
}

.overview-table th {
  @apply border-b border-gray-200 bg-gray-50/80 px-4 py-3 text-xs font-medium uppercase text-gray-500 dark:border-dark-700 dark:bg-dark-800/80 dark:text-dark-300;
}

.overview-table td {
  @apply border-b border-gray-100 px-4 py-3 align-top text-gray-700 dark:border-dark-800 dark:text-dark-200;
}

.muted {
  @apply mt-0.5 text-xs text-gray-500 dark:text-dark-300;
}

.empty-cell {
  @apply py-8 text-center text-gray-500 dark:text-dark-300;
}

.chip-line {
  @apply flex max-w-md flex-wrap gap-1;
}

.chip {
  @apply rounded border border-gray-200 bg-gray-50 px-1.5 py-0.5 text-[11px] text-gray-600 dark:border-dark-700 dark:bg-dark-800 dark:text-dark-300;
}

.status {
  @apply mr-1 inline-flex items-center rounded border px-1.5 py-0.5 text-[11px] font-medium;
}

.status-ok {
  @apply border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-900 dark:bg-emerald-950/40 dark:text-emerald-200;
}

.status-info {
  @apply border-sky-200 bg-sky-50 text-sky-700 dark:border-sky-900 dark:bg-sky-950/40 dark:text-sky-200;
}

.status-muted {
  @apply border-gray-200 bg-gray-50 text-gray-600 dark:border-dark-700 dark:bg-dark-800 dark:text-dark-300;
}

.status-danger {
  @apply border-red-200 bg-red-50 text-red-700 dark:border-red-900 dark:bg-red-950/40 dark:text-red-200;
}

.status-default {
  @apply border-primary-200 bg-primary-50 text-primary-700 dark:border-primary-900 dark:bg-primary-950/40 dark:text-primary-200;
}
</style>
