<template>
  <section class="space-y-3">
    <div class="flex flex-wrap items-start justify-between gap-2">
      <div class="space-y-1">
        <p class="text-sm text-gray-600 dark:text-gray-300">
          管理用户可选择的模型。默认模型必须可用且可见，否则客户端无法正常启动。
        </p>
        <div class="flex flex-wrap gap-2 text-xs text-gray-500 dark:text-gray-400">
          <span class="stat">总数 {{ models.length }}</span>
          <span class="stat">可见 {{ visibleCount }}</span>
          <span class="stat">可用 {{ enabledCount }}</span>
          <span class="stat">默认 {{ defaultModelLabel }}</span>
        </div>
      </div>
      <button class="btn-primary" type="button" @click="addModel">新增模型</button>
    </div>

    <div class="guard" :class="defaultWarning ? 'guard-warn' : 'guard-ok'">
      <div>
        <span class="font-medium">默认模型检查：</span>
        {{ defaultWarning || '已设置一个可用且可见的默认模型。' }}
      </div>
      <button
        v-if="defaultWarning && firstEffectiveIndex >= 0"
        class="btn-secondary"
        type="button"
        @click="setDefault(firstEffectiveIndex)"
      >
        使用第一个可用模型
      </button>
    </div>

    <div class="overflow-x-auto">
      <table class="admin-table">
        <thead>
          <tr>
            <th>模型</th>
            <th>实际路由</th>
            <th>分组和标签</th>
            <th>容量和计费</th>
            <th>状态</th>
            <th>停用说明</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="(model, index) in models"
            :key="model.model_id || index"
            :class="{ 'row-muted': !isEffectiveModel(model) }"
          >
            <td>
              <input v-model.trim="model.model_id" class="input mb-1" placeholder="codex-standard" list="codex-model-candidates" />
              <input v-model.trim="model.display_name" class="input" placeholder="标准模型" maxlength="80" />
            </td>
            <td>
              <input v-model.trim="model.route_model" class="input mb-1" placeholder="上游模型编号" list="codex-route-candidates" />
              <input
                :value="model.fallback_model_id || ''"
                class="input"
                placeholder="备用模型"
                list="codex-configured-models"
                @input="model.fallback_model_id = nullableInput(($event.target as HTMLInputElement).value)"
              />
            </td>
            <td>
              <input v-model.trim="model.model_group" class="input mb-1" placeholder="codex" list="codex-model-groups" />
              <input
                :value="(model.operator_tags || []).join(', ')"
                class="input"
                placeholder="标签：默认，高级"
                @input="model.operator_tags = splitList(($event.target as HTMLInputElement).value)"
              />
            </td>
            <td>
              <div class="grid grid-cols-2 gap-1">
                <label class="field-label">
                  <span>上下文</span>
                  <input v-model.number="model.context_window" class="input" type="number" min="1024" step="1024" />
                </label>
                <label class="field-label">
                  <span>倍率</span>
                  <input v-model.number="model.billing_multiplier" class="input" type="number" min="0.01" step="0.01" />
                </label>
              </div>
              <label class="field-label mt-1">
                <span>排序</span>
                <input v-model.number="model.sort_order" class="input" type="number" min="0" step="1" />
              </label>
            </td>
            <td>
              <select :value="modelStatus(model)" class="input mb-1" @change="setModelStatus(model, index, ($event.target as HTMLSelectElement).value as ModelStatus)">
                <option value="active">可用</option>
                <option value="hidden">隐藏</option>
                <option value="disabled">停用</option>
                <option value="delisted">下架</option>
              </select>
              <div class="mb-1 flex flex-wrap gap-1">
                <span class="status-pill" :class="statusClass(model)">{{ statusLabel(model) }}</span>
                <span v-if="model.is_default" class="status-pill default-pill">默认</span>
              </div>
              <label class="check">
                <input :checked="model.is_default" type="radio" name="default-model" @change="setDefault(index)" />
                设为默认
              </label>
              <div class="grid grid-cols-2 gap-1">
                <select v-model="model.rollout_channel" class="input">
                  <option value="internal">内部</option>
                  <option value="canary">灰度</option>
                  <option value="stable">稳定</option>
                  <option value="deprecated">即将停用</option>
                </select>
                <select v-model="model.quality_tier" class="input">
                  <option value="standard">标准</option>
                  <option value="premium">高级</option>
                  <option value="experimental">实验</option>
                  <option value="legacy">旧版</option>
                </select>
              </div>
            </td>
            <td>
              <textarea
                :value="model.disabled_reason || ''"
                class="input min-h-[64px] resize-y"
                maxlength="160"
                placeholder="停用或下架时填写给管理员看的原因"
                @input="model.disabled_reason = nullableInput(($event.target as HTMLTextAreaElement).value)"
              />
              <div class="mt-1 grid grid-cols-2 gap-1">
                <input
                  :value="model.disabled_replacement_model_id || ''"
                  class="input"
                  placeholder="替代模型"
                  list="codex-configured-models"
                  @input="model.disabled_replacement_model_id = nullableInput(($event.target as HTMLInputElement).value)"
                />
                <input
                  :value="model.disabled_message_key || ''"
                  class="input"
                  placeholder="提示标识"
                  @input="model.disabled_message_key = nullableInput(($event.target as HTMLInputElement).value)"
                />
              </div>
              <input
                :value="model.deprecation_at || ''"
                class="input mt-1"
                placeholder="计划停用时间"
                @input="model.deprecation_at = nullableInput(($event.target as HTMLInputElement).value)"
              />
            </td>
            <td>
              <button class="btn-danger" type="button" @click="removeModel(index)">删除</button>
            </td>
          </tr>
        </tbody>
      </table>

      <datalist id="codex-model-candidates">
        <option v-for="(candidate, index) in modelCandidates" :key="candidateKey(candidate, index)" :value="candidate.model_id">
          {{ candidate.group_name || candidate.platform || '' }}
        </option>
      </datalist>
      <datalist id="codex-route-candidates">
        <option v-for="(candidate, index) in modelCandidates" :key="candidateKey(candidate, index)" :value="candidate.model_id">
          {{ candidate.group_name || candidate.platform || '' }}
        </option>
      </datalist>
      <datalist id="codex-configured-models">
        <option v-for="modelId in configuredModelIds" :key="modelId" :value="modelId" />
      </datalist>
      <datalist id="codex-model-groups">
        <option v-for="group in configuredModelGroups" :key="group" :value="group" />
      </datalist>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { CodexPlusModel, CodexPlusOptions } from '@/api/admin/codexPlus'

type ModelStatus = 'active' | 'hidden' | 'disabled' | 'delisted'

const props = defineProps<{
  models: CodexPlusModel[]
  modelCandidates: CodexPlusOptions['models']
}>()

const enabledCount = computed(() => props.models.filter(model => model.is_enabled).length)
const visibleCount = computed(() => props.models.filter(model => !model.is_hidden).length)
const configuredModelIds = computed(() => Array.from(new Set(props.models.map(model => model.model_id.trim()).filter(Boolean))).sort())
const configuredModelGroups = computed(() => Array.from(new Set(props.models.map(model => model.model_group.trim()).filter(Boolean))).sort())
const defaultModels = computed(() => props.models.filter(model => model.is_default))
const firstEffectiveIndex = computed(() => props.models.findIndex(isEffectiveModel))

const defaultModelLabel = computed(() => {
  if (defaultModels.value.length !== 1) return defaultModels.value.length === 0 ? '未设置' : '重复'
  return defaultModels.value[0].display_name || defaultModels.value[0].model_id || '新模型'
})

const defaultWarning = computed(() => {
  if (defaultModels.value.length === 0) return '还没有默认模型。发布前请选择一个可用模型。'
  if (defaultModels.value.length > 1) return '存在多个默认模型。重新选择一个默认模型会自动清除其它默认标记。'
  if (!isEffectiveModel(defaultModels.value[0])) return '默认模型必须可用且可见，隐藏或停用的模型不能作为默认模型。'
  return ''
})

function addModel() {
  const nextIndex = props.models.length + 1
  props.models.push({
    model_id: `codex-model-${nextIndex}`,
    display_name: '新模型',
    route_model: 'codex-default',
    model_group: 'codex',
    context_window: 8192,
    billing_multiplier: 1,
    is_default: props.models.length === 0,
    is_enabled: true,
    is_hidden: false,
    disabled_reason: null,
    rollout_channel: 'stable',
    quality_tier: 'standard',
    fallback_model_id: null,
    deprecation_at: null,
    disabled_replacement_model_id: null,
    disabled_message_key: null,
    sort_order: nextSortOrder(),
    operator_tags: []
  })
}

function removeModel(index: number) {
  if (props.models.length <= 1) return
  const wasDefault = props.models[index].is_default
  props.models.splice(index, 1)
  if (wasDefault || props.models.filter(model => model.is_default).length !== 1) {
    promoteDefault()
  }
}

function setDefault(index: number) {
  props.models.forEach((model, i) => {
    model.is_default = i === index
    if (i === index) {
      model.is_enabled = true
      model.is_hidden = false
    }
  })
}

function setModelStatus(model: CodexPlusModel, index: number, status: ModelStatus) {
  model.is_enabled = status === 'active' || status === 'hidden'
  model.is_hidden = status === 'hidden' || status === 'delisted'
  normalizeDisabledFields(model)

  if (model.is_default && !isEffectiveModel(model)) {
    model.is_default = false
    promoteDefault(index)
  }
}

function promoteDefault(excludeIndex = -1) {
  const nextIndex = props.models.findIndex((model, index) => index !== excludeIndex && isEffectiveModel(model))
  props.models.forEach((model, index) => {
    model.is_default = index === nextIndex
  })
}

function modelStatus(model: CodexPlusModel): ModelStatus {
  if (!model.is_enabled && model.is_hidden) return 'delisted'
  if (!model.is_enabled) return 'disabled'
  if (model.is_hidden) return 'hidden'
  return 'active'
}

function statusLabel(model: CodexPlusModel): string {
  const labels: Record<ModelStatus, string> = {
    active: '可用',
    hidden: '隐藏',
    disabled: '停用',
    delisted: '下架'
  }
  return labels[modelStatus(model)]
}

function statusClass(model: CodexPlusModel): string {
  switch (modelStatus(model)) {
    case 'active':
      return 'status-active'
    case 'hidden':
      return 'status-hidden'
    case 'disabled':
      return 'status-disabled'
    case 'delisted':
      return 'status-delisted'
  }
}

function isEffectiveModel(model: CodexPlusModel): boolean {
  return model.is_enabled && !model.is_hidden
}

function normalizeDisabledFields(model: CodexPlusModel) {
  if (model.is_enabled) return
  if (!model.disabled_reason?.trim()) {
    model.disabled_reason = '管理员停用'
  }
  if (!model.disabled_message_key?.trim()) {
    model.disabled_message_key = 'model.disabled'
  }
}

function splitList(value: string): string[] {
  return value.split(',').map(item => item.trim()).filter(Boolean)
}

function nullableInput(value: string): string | null {
  const trimmed = value.trim()
  return trimmed ? trimmed : null
}

function nextSortOrder(): number {
  const sortOrders = props.models.map(model => model.sort_order || 0)
  return sortOrders.length ? Math.max(...sortOrders) + 10 : 10
}

function candidateKey(candidate: CodexPlusOptions['models'][number], index: number): string {
  return `${candidate.group_id || candidate.group_name || candidate.platform || 'candidate'}:${candidate.model_id}:${index}`
}
</script>

<style scoped>
.admin-table {
  @apply w-full min-w-[1360px] text-left text-sm;
}

.admin-table th {
  @apply border-b border-gray-200 px-2 py-2 text-xs font-semibold uppercase text-gray-500 dark:border-dark-700;
}

.admin-table td {
  @apply border-b border-gray-100 px-2 py-2 align-top dark:border-dark-800;
}

.row-muted {
  @apply bg-gray-50/60 dark:bg-dark-900/40;
}

.input {
  @apply w-full rounded-md border border-gray-300 bg-white px-2 py-1.5 text-sm dark:border-dark-600 dark:bg-dark-800;
}

.field-label {
  @apply block space-y-1 text-[11px] font-medium uppercase text-gray-500;
}

.check {
  @apply mb-1 flex items-center gap-2 text-xs;
}

.stat {
  @apply rounded border border-gray-200 px-2 py-1 dark:border-dark-700;
}

.guard {
  @apply flex flex-wrap items-center justify-between gap-2 rounded-md border px-3 py-2 text-sm;
}

.guard-ok {
  @apply border-emerald-200 bg-emerald-50 text-emerald-800 dark:border-emerald-900 dark:bg-emerald-950 dark:text-emerald-200;
}

.guard-warn {
  @apply border-amber-200 bg-amber-50 text-amber-800 dark:border-amber-900 dark:bg-amber-950 dark:text-amber-200;
}

.status-pill {
  @apply rounded border px-1.5 py-0.5 text-[11px] font-medium;
}

.status-active {
  @apply border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-900 dark:bg-emerald-950 dark:text-emerald-200;
}

.status-hidden {
  @apply border-sky-200 bg-sky-50 text-sky-700 dark:border-sky-900 dark:bg-sky-950 dark:text-sky-200;
}

.status-disabled {
  @apply border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-900 dark:bg-amber-950 dark:text-amber-200;
}

.status-delisted {
  @apply border-red-200 bg-red-50 text-red-700 dark:border-red-900 dark:bg-red-950 dark:text-red-200;
}

.default-pill {
  @apply border-primary-200 bg-primary-50 text-primary-700 dark:border-primary-900 dark:bg-primary-950 dark:text-primary-200;
}

.btn-primary {
  @apply rounded-md bg-primary-600 px-3 py-2 text-sm font-medium text-white hover:bg-primary-700;
}

.btn-secondary {
  @apply rounded-md border border-current px-2 py-1 text-xs font-medium hover:bg-white/40 dark:hover:bg-dark-800;
}

.btn-danger {
  @apply rounded-md border border-red-300 px-2 py-1 text-xs text-red-600 hover:bg-red-50 dark:border-red-800 dark:hover:bg-red-950;
}
</style>
