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
      error.value = callbackError?.message || '未知错误'
      setTimeout(() => {
        router.replace('/')
      }, 1200)
    }
  })
</script>
