<template>
  <section class="page-view home-view">
    <header class="page-head">
      <p class="page-eyebrow">初入仙途</p>
      <h2>欢迎</h2>
      <p class="page-desc">感谢游玩《修仙大世界》，开始你的修仙之旅吧。</p>
    </header>

    <n-card :bordered="false" class="page-card">
      <n-space class="home-content" vertical>
        <n-space justify="center">
          <h3>感谢游玩修仙大世界</h3>
        </n-space>
        <n-space justify="center">
          <p>开始你的修仙之旅吧！</p>
        </n-space>
        <n-space justify="center" v-if="showLoginButton">
          <n-button type="primary" @click="sessionStore.redirectToLinuxDoLogin">使用 Linux.do 登录</n-button>
        </n-space>
        <n-space justify="center" v-if="showLoginStatus">
          <n-tag type="success">已登录：{{ sessionStore.user?.username || '道友' }}</n-tag>
          <n-button tertiary @click="logout">退出登录</n-button>
        </n-space>
      </n-space>
    </n-card>
  </section>
</template>

<script setup>
  import { computed } from 'vue'
  import { useSessionStore } from '../stores/session'
  import { useMessage } from 'naive-ui'
  import { useRouter } from 'vue-router'
  const router = useRouter()
  const sessionStore = useSessionStore()
  const message = useMessage()
  const showLoginButton = computed(() => !sessionStore.isAuthenticated)
  const showLoginStatus = computed(() => sessionStore.isAuthenticated)

  const logout = async () => {
    await sessionStore.logout()
    router.push('/')
    message.success('已退出登录')
  }
</script>

<style scoped>
  .home-content {
    min-height: 220px;
    justify-content: center;
  }

  .home-content h3 {
    margin-bottom: 8px;
    color: var(--ink-main);
    font-family: var(--font-display);
    font-size: 28px;
    text-align: center;
  }

  .home-content p {
    color: var(--ink-sub);
    font-size: 15px;
  }

  @media (max-width: 768px) {
    .home-content h3 {
      font-size: 24px;
    }
  }
</style>
