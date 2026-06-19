<template>
  <AuthLayout>
    <div class="space-y-6">
      <div class="text-center">
        <h2 class="text-2xl font-bold text-gray-900 dark:text-white">
          授权 Codex++ 桌面端
        </h2>
        <p class="mt-2 text-sm text-gray-500 dark:text-dark-400">
          确认后，当前账号会登录到正在等待的 Codex++ Manager。
        </p>
      </div>

      <div
        v-if="verificationCode"
        class="rounded-lg border border-gray-200 bg-gray-50 p-4 text-center dark:border-dark-700 dark:bg-dark-800"
      >
        <p class="text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-dark-400">
          桌面端确认码
        </p>
        <p class="mt-2 font-mono text-3xl font-semibold text-gray-900 dark:text-white">
          {{ verificationCode }}
        </p>
      </div>

      <div class="rounded-lg border border-primary-100 bg-primary-50 p-4 dark:border-primary-900/40 dark:bg-primary-950/20">
        <p class="text-sm text-gray-700 dark:text-dark-200">
          当前账号：<span class="font-medium text-gray-900 dark:text-white">{{ userLabel }}</span>
        </p>
      </div>

      <div v-if="message" :class="messageClass" class="rounded-lg border p-4 text-sm">
        {{ message }}
      </div>

      <div class="space-y-3">
        <button
          type="button"
          class="btn btn-primary w-full"
          :disabled="!canApprove || isSubmitting"
          @click="approve"
        >
          {{ isSubmitting ? '正在授权...' : '授权并返回桌面端' }}
        </button>
        <button
          type="button"
          class="btn btn-secondary w-full"
          @click="router.push('/dashboard')"
        >
          取消
        </button>
      </div>
    </div>
  </AuthLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { AuthLayout } from '@/components/layout'
import { completeDesktopLogin } from '@/api/auth'
import { useAuthStore } from '@/stores/auth'

const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()

const isSubmitting = ref(false)
const message = ref('')
const messageType = ref<'info' | 'success' | 'error'>('info')

const sessionToken = computed(() => String(route.query.session_token ?? '').trim())
const verificationCode = computed(() => {
  const value = String(route.query.verification_code ?? '').trim()
  return /^\d{6}$/.test(value) ? value : ''
})
const userLabel = computed(() => authStore.user?.email || authStore.user?.username || '当前登录用户')
const canApprove = computed(() => authStore.isAuthenticated && sessionToken.value.length > 0 && messageType.value !== 'success')
const messageClass = computed(() => {
  if (messageType.value === 'success') {
    return 'border-green-200 bg-green-50 text-green-700 dark:border-green-900/50 dark:bg-green-950/20 dark:text-green-300'
  }
  if (messageType.value === 'error') {
    return 'border-red-200 bg-red-50 text-red-700 dark:border-red-900/50 dark:bg-red-950/20 dark:text-red-300'
  }
  return 'border-gray-200 bg-gray-50 text-gray-700 dark:border-dark-700 dark:bg-dark-800 dark:text-dark-200'
})

onMounted(() => {
  if (!sessionToken.value) {
    messageType.value = 'error'
    message.value = '授权链接缺少会话信息，请回到 Codex++ Manager 重新发起登录。'
    return
  }
  if (!authStore.isAuthenticated) {
    void router.replace({
      path: '/login',
      query: { redirect: route.fullPath }
    })
  }
})

async function approve(): Promise<void> {
  if (!canApprove.value) {
    return
  }
  isSubmitting.value = true
  message.value = ''
  try {
    await completeDesktopLogin(sessionToken.value)
    messageType.value = 'success'
    message.value = '授权完成。请回到 Codex++ Manager，它会自动完成登录。'
  } catch (error) {
    const err = error as { message?: string }
    messageType.value = 'error'
    message.value = err.message || '授权失败，请回到 Codex++ Manager 重新发起登录。'
  } finally {
    isSubmitting.value = false
  }
}
</script>
