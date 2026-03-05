<template>
  <section class="page-view settings-view">
    <header class="page-head">
      <p class="page-eyebrow">洞府设置</p>
      <h2>游戏设置</h2>
      <p class="page-desc">查看版本信息与社区入口。</p>
    </header>

    <n-card :bordered="false" class="page-card">
      <template #header-extra>游戏版本 {{ version }}</template>
      <n-space vertical>
        <n-alert type="info" :show-icon="false">
          当前版本：<strong>{{ version }}</strong>
        </n-alert>
        <n-space>
          <n-button tertiary @click="qq = true">玩家交流群</n-button>
        </n-space>
      </n-space>
    </n-card>

    <n-card :bordered="false" class="page-card" title="账号与会话">
      <n-space vertical>
        <n-alert type="success" :show-icon="false">
          当前登录：<strong>{{ sessionStore.user?.username || '道友' }}</strong>
        </n-alert>
        <n-space>
          <n-button type="error" secondary :loading="loggingOut" @click="confirmLogout">退出登录</n-button>
        </n-space>
      </n-space>
    </n-card>

    <n-modal preset="dialog" title="玩家交流群" v-model:show="qq">
      <n-card :bordered="false" size="huge" role="dialog" aria-modal="true">
        <n-space vertical>
          <n-text depth="3">QQ群号</n-text>
          <n-input value="" readonly type="text" />
        </n-space>
      </n-card>
    </n-modal>
  </section>
</template>

<script setup>
  import { ref } from 'vue'
  import { useDialog, useMessage } from 'naive-ui'
  import { useRouter } from 'vue-router'
  import { useSessionStore } from '../stores/session'

  const version = __APP_VERSION__
  const qq = ref(false)
  const loggingOut = ref(false)

  const sessionStore = useSessionStore()
  const message = useMessage()
  const dialog = useDialog()
  const router = useRouter()

  const executeLogout = async () => {
    if (loggingOut.value) {
      return
    }

    loggingOut.value = true
    try {
      const remoteLoggedOut = await sessionStore.logout()
      if (remoteLoggedOut) {
        message.success('已退出登录')
      } else {
        message.warning('本地已退出登录，服务端退出请求失败')
      }
      await router.replace('/')
    } catch (error) {
      message.error(error?.message || '退出登录失败')
    } finally {
      loggingOut.value = false
    }
  }

  const confirmLogout = () => {
    dialog.warning({
      title: '退出登录',
      content: '确认退出当前账号吗？',
      positiveText: '退出登录',
      negativeText: '取消',
      onPositiveClick: executeLogout
    })
  }
</script>
