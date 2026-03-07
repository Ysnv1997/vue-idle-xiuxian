<template>
  <section class="page-view auth-view">
    <header class="page-head">
      <p class="page-eyebrow">身份校验</p>
      <h2>登录处理中</h2>
      <p class="page-desc">正在同步登录状态，请稍候。</p>
    </header>

    <n-card :bordered="false" class="page-card">
      <n-space vertical>
        <n-text>正在同步登录状态，请稍候...</n-text>
        <n-text v-if="error" type="error">登录失败：{{ error }}</n-text>
      </n-space>
    </n-card>
  </section>
</template>

<script setup>
  import { onMounted, ref } from 'vue'
  import { useRoute, useRouter } from 'vue-router'
  import { useSessionStore } from '../stores/session'

  const route = useRoute()
  const router = useRouter()
  const sessionStore = useSessionStore()
  const error = ref('')

  onMounted(async () => {
    try {
      const consumed = await sessionStore.consumeOAuthCallback(route.query)
      if (!consumed && !sessionStore.isAuthenticated) {
        router.replace('/')
        return
      }
      router.replace('/cultivation')
    } catch (callbackError) {
      error.value = resolveAuthErrorMessage(callbackError?.message)
      setTimeout(() => {
        router.replace('/')
      }, 1200)
    }
  })

  const resolveAuthErrorMessage = raw => {
    const reason = String(raw || '').trim()
    if (!reason) {
      return '未知错误'
    }
    if (reason === 'registration_limit_reached') {
      return '开放注册人数已满，请持续关注'
    }
    if (reason === 'token_exchange_failed') {
      return '登录失败：OAuth 换取令牌失败'
    }
    if (reason === 'fetch_profile_failed') {
      return '登录失败：获取 LinuxDo 用户信息失败'
    }
    if (reason === 'invalid_state' || reason === 'missing_code') {
      return '登录失败：授权状态已失效，请重试'
    }
    if (reason === 'local_login_failed') {
      return '登录失败：本地账号处理失败'
    }
    if (reason.startsWith('oauth_error:')) {
      return '登录失败：LinuxDo 授权被取消或拒绝'
    }
    return reason
  }
</script>
