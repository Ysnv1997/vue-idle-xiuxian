<template>
  <n-space class="home-container" vertical>
    <n-space justify="center">
      <h2>感谢游玩我的放置仙途</h2>
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
  .home-container {
    padding: 2rem;
  }

  .home-container h2 {
    margin-bottom: 1rem;
    color: #2080f0;
  }

  .home-container p {
    color: #666;
  }
</style>
