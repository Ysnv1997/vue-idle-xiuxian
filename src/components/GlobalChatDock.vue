<template>
  <section class="chat-dock" :class="{ collapsed: isCollapsed }">
    <header class="chat-head">
      <div class="chat-title-wrap">
        <strong>世界聊天</strong>
        <n-tag size="small" :type="chatStore.connected ? 'success' : 'warning'">
          {{ chatStore.connected ? '在线' : '离线' }}
        </n-tag>
      </div>
      <n-space :size="8" align="center">
        <n-button size="tiny" tertiary :loading="chatStore.loadingHistory" @click="refreshHistory">刷新</n-button>
        <n-button size="tiny" quaternary circle @click="toggleCollapse">
          {{ isCollapsed ? '▲' : '▼' }}
        </n-button>
      </n-space>
    </header>

    <div v-show="!isCollapsed" class="chat-body">
      <n-scrollbar ref="scrollRef" class="chat-stream" trigger="none">
        <div class="chat-lines" v-if="messages.length">
          <div v-for="item in messages" :key="item.id" class="chat-line">
            <span class="chat-line-content">{{ formatLine(item) }}</span>
            <span class="chat-line-time">{{ formatTime(item.createdAt) }}</span>
          </div>
        </div>
        <n-empty v-else description="暂无消息" />
      </n-scrollbar>

      <div class="chat-compose">
        <n-input
          v-model:value="draft"
          placeholder="输入聊天内容，回车发送"
          :disabled="!chatStore.connected || chatStore.muteStatus?.muted"
          @keyup.enter="send"
        />
        <n-button type="primary" :disabled="!canSend" @click="send">发送</n-button>
      </div>

      <p v-if="chatStore.muteStatus?.muted" class="chat-muted">
        当前禁言至 {{ formatTime(chatStore.muteStatus.mutedUntil) }}
        <span v-if="chatStore.muteStatus.reason">，原因：{{ chatStore.muteStatus.reason }}</span>
      </p>
      <p v-if="chatStore.lastError" class="chat-error">{{ chatStore.lastError }}</p>
    </div>
  </section>
</template>

<script setup>
  import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
  import { useMessage } from 'naive-ui'
  import { useChatStore } from '../stores/chat'
  import { getAccessToken } from '../api/token-storage'

  const chatStore = useChatStore()
  const message = useMessage()
  const draft = ref('')
  const isCollapsed = ref(false)
  const scrollRef = ref(null)

  const messages = computed(() => (Array.isArray(chatStore.messages) ? chatStore.messages : []))
  const canSend = computed(() => {
    return chatStore.connected && !chatStore.muteStatus?.muted && draft.value.trim().length > 0
  })

  const formatTime = value => {
    if (!value) return '--:--:--'
    const date = new Date(value)
    if (Number.isNaN(date.getTime())) return '--:--:--'
    return date.toLocaleTimeString()
  }

  const formatLine = item => {
    const sender = String(item?.senderName || '匿名修士').trim() || '匿名修士'
    const content = String(item?.content || '').trim()
    return `${sender}说：${content || '...'}`
  }

  const scrollToBottom = () => {
    setTimeout(() => {
      if (!scrollRef.value || isCollapsed.value) return
      scrollRef.value.scrollTo({ top: 99999, behavior: 'smooth' })
    })
  }

  const ensureChatReady = async () => {
    const accessToken = getAccessToken()
    if (!accessToken) return
    if (!chatStore.connected && !chatStore.connecting) {
      chatStore.connect()
    }

    try {
      await Promise.all([chatStore.loadHistory(), chatStore.loadMuteStatus()])
    } catch (error) {
      if (error?.message) {
        message.error(error.message)
      }
    } finally {
      scrollToBottom()
    }
  }

  const refreshHistory = async () => {
    try {
      await Promise.all([chatStore.loadHistory(), chatStore.loadMuteStatus()])
      scrollToBottom()
    } catch (error) {
      message.error(error?.message || '刷新聊天失败')
    }
  }

  const toggleCollapse = () => {
    isCollapsed.value = !isCollapsed.value
    if (!isCollapsed.value) {
      scrollToBottom()
    }
  }

  const send = () => {
    if (!canSend.value) return
    const success = chatStore.sendMessage(draft.value)
    if (success) {
      draft.value = ''
    }
  }

  onMounted(async () => {
    await ensureChatReady()
  })

  onUnmounted(() => {
    chatStore.disconnect()
  })

  watch(
    () => messages.value.length,
    () => {
      scrollToBottom()
    }
  )
</script>

<style scoped>
  .chat-dock {
    position: fixed;
    left: 50%;
    bottom: 10px;
    width: min(980px, calc(100vw - 20px));
    transform: translateX(-50%);
    z-index: 1200;
    border: 1px solid var(--panel-border);
    border-radius: 14px;
    background: color-mix(in srgb, var(--panel-bg) 92%, transparent);
    backdrop-filter: blur(10px);
    box-shadow: 0 14px 30px rgba(20, 22, 26, 0.2);
  }

  .chat-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 8px 10px;
    border-bottom: 1px solid var(--panel-border);
  }

  .chat-title-wrap {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 14px;
    color: var(--ink-main);
  }

  .chat-dock.collapsed .chat-head {
    border-bottom: none;
  }

  .chat-body {
    padding: 8px 10px 10px;
  }

  .chat-stream {
    max-height: 220px;
    border: 1px solid color-mix(in srgb, var(--panel-border) 70%, transparent);
    border-radius: 10px;
    padding: 4px 8px;
    background: color-mix(in srgb, var(--panel-bg) 84%, transparent);
  }

  .chat-lines {
    padding: 2px 0;
  }

  .chat-line {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 12px;
    padding: 6px 2px;
    border-bottom: 1px dashed color-mix(in srgb, var(--panel-border) 85%, transparent);
  }

  .chat-line:last-child {
    border-bottom: none;
  }

  .chat-line-content {
    flex: 1;
    color: var(--ink-main);
    font-size: 13px;
    line-height: 1.45;
    word-break: break-word;
  }

  .chat-line-time {
    flex-shrink: 0;
    color: var(--ink-sub);
    font-size: 12px;
    line-height: 1.4;
  }

  .chat-compose {
    margin-top: 8px;
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    gap: 8px;
    align-items: center;
  }

  .chat-muted,
  .chat-error {
    margin-top: 6px;
    font-size: 12px;
    line-height: 1.4;
  }

  .chat-muted {
    color: #d89614;
  }

  .chat-error {
    color: #d03050;
  }

  @media (max-width: 768px) {
    .chat-dock {
      bottom: 6px;
      width: calc(100vw - 12px);
    }

    .chat-stream {
      max-height: 180px;
    }
  }
</style>
