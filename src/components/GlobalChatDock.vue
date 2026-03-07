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
            <div class="chat-line-main">
              <span class="chat-line-content">
                <button class="chat-line-sender" @click="openProfile(item)">{{ formatSender(item) }}</button>
                <span>说：{{ String(item?.content || '').trim() || '...' }}</span>
              </span>
              <span class="chat-line-time">{{ formatTime(item.createdAt) }}</span>
            </div>
            <div class="chat-line-actions">
              <n-button text size="tiny" class="chat-line-report" @click="reportMessage(item.id)">举报</n-button>
              <n-button v-if="canMuteMessage(item)" text size="tiny" class="chat-line-mute" @click="openMuteDialog(item)">
                禁言
              </n-button>
            </div>
          </div>
        </div>
        <n-empty v-else description="暂无消息" />
      </n-scrollbar>

      <div class="chat-status">
        <p v-if="chatStore.muteStatus?.muted" class="chat-muted">
          当前禁言至 {{ formatTime(chatStore.muteStatus.mutedUntil) }}
          <span v-if="chatStore.muteStatus.reason">，原因：{{ chatStore.muteStatus.reason }}</span>
        </p>
        <p v-if="chatStore.lastError" class="chat-error">{{ chatStore.lastError }}</p>
      </div>

      <div class="chat-compose">
        <n-input
          v-model:value="draft"
          placeholder="输入聊天内容，回车发送"
          :disabled="!chatStore.connected || chatStore.muteStatus?.muted"
          @keyup.enter="send"
        />
        <n-button type="primary" :disabled="!canSend" @click="send">发送</n-button>
      </div>
    </div>
  </section>

  <n-modal v-model:show="showMuteDialog" preset="dialog" title="管理员禁言" style="width: 460px">
    <template v-if="muteTarget">
      <n-space vertical :size="10">
        <n-descriptions bordered :column="1" size="small">
          <n-descriptions-item label="目标昵称">
            {{ muteTarget.senderName || '匿名修士' }}
          </n-descriptions-item>
          <n-descriptions-item label="LinuxDo ID">
            {{ muteTarget.senderLinuxDoUserId }}
          </n-descriptions-item>
          <n-descriptions-item label="消息内容">
            <span class="mute-dialog-content">{{ muteTarget.content || '（空）' }}</span>
          </n-descriptions-item>
        </n-descriptions>
        <div class="mute-dialog-field">
          <span class="mute-dialog-label">禁言时长</span>
          <n-select v-model:value="muteDurationMinutes" :options="muteDurationOptions" />
        </div>
        <div class="mute-dialog-field">
          <span class="mute-dialog-label">禁言原因（可选）</span>
          <n-input
            v-model:value="muteReason"
            maxlength="80"
            placeholder="默认：管理员禁言"
            show-count
          />
        </div>
      </n-space>
    </template>
    <template #action>
      <n-space justify="end">
        <n-button :disabled="muteSubmitting" @click="closeMuteDialog">取消</n-button>
        <n-button type="primary" :loading="muteSubmitting" @click="submitMute">确认禁言</n-button>
      </n-space>
    </template>
  </n-modal>

  <player-profile-dialog v-model:show="showProfileDialog" :loading="profileLoading" :profile="selectedProfile" />
</template>

<script setup>
  import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
  import { useDialog, useMessage } from 'naive-ui'
  import { useChatStore } from '../stores/chat'
  import { getAccessToken } from '../api/token-storage'
  import { useSessionStore } from '../stores/session'
  import { fetchPublicPlayerProfile } from '../api/modules/player'
  import PlayerProfileDialog from './PlayerProfileDialog.vue'

  const chatStore = useChatStore()
  const sessionStore = useSessionStore()
  const message = useMessage()
  const dialog = useDialog()
  const draft = ref('')
  const isCollapsed = ref(false)
  const scrollRef = ref(null)
  const showMuteDialog = ref(false)
  const muteSubmitting = ref(false)
  const muteReason = ref('')
  const muteDurationMinutes = ref(5)
  const muteTarget = ref(null)
  const showProfileDialog = ref(false)
  const profileLoading = ref(false)
  const selectedProfile = ref(null)

  const messages = computed(() => (Array.isArray(chatStore.messages) ? chatStore.messages : []))
  const canModerateChat = computed(() => Boolean(sessionStore.user?.canModerateChat))
  const myLinuxDoUserId = computed(() => String(sessionStore.user?.linuxDoUserId || '').trim())
  const canSend = computed(() => {
    return chatStore.connected && !chatStore.muteStatus?.muted && draft.value.trim().length > 0
  })
  const muteDurationOptions = [
    { label: '5 分钟', value: 5 },
    { label: '30 分钟', value: 30 },
    { label: '60 分钟', value: 60 },
    { label: '3 小时', value: 180 },
    { label: '12 小时', value: 720 },
    { label: '24 小时', value: 1440 }
  ]

  const formatTime = value => {
    if (!value) return '--:--:--'
    const date = new Date(value)
    if (Number.isNaN(date.getTime())) return '--:--:--'
    return date.toLocaleTimeString()
  }

  const formatSender = item => {
    const sender = String(item?.senderName || '匿名修士').trim() || '匿名修士'
    const realm = String(item?.senderRealm || '').trim()
    return realm ? `${sender} · ${realm}` : sender
  }

  const resolveSenderLinuxDoUserId = item => String(item?.senderLinuxDoUserId || '').trim()

  const canMuteMessage = item => {
    if (!canModerateChat.value) return false
    const senderLinuxDoUserId = resolveSenderLinuxDoUserId(item)
    if (!senderLinuxDoUserId) return false
    if (senderLinuxDoUserId === myLinuxDoUserId.value) return false
    return true
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

  const reportMessage = messageId => {
    if (!messageId) return
    dialog.warning({
      title: '举报消息',
      content: '确认举报该条消息？',
      positiveText: '确认',
      negativeText: '取消',
      onPositiveClick: async () => {
        try {
          await chatStore.report(messageId, 'player_report')
          message.success('举报已提交')
        } catch (error) {
          message.error(error?.message || '举报失败')
        }
      }
    })
  }

  const openProfile = async item => {
    const userId = String(item?.senderUserID || item?.senderUserId || '').trim()
    if (!userId) {
      message.warning('该玩家资料暂不可查看')
      return
    }
    showProfileDialog.value = true
    profileLoading.value = true
    selectedProfile.value = null
    try {
      selectedProfile.value = await fetchPublicPlayerProfile(userId)
    } catch (error) {
      message.error(error?.message || '加载玩家资料失败')
      showProfileDialog.value = false
    } finally {
      profileLoading.value = false
    }
  }

  const openMuteDialog = item => {
    if (!canMuteMessage(item)) {
      message.warning('该消息发送者暂不可禁言')
      return
    }
    muteTarget.value = {
      id: item?.id || 0,
      senderName: String(item?.senderName || '').trim(),
      senderLinuxDoUserId: resolveSenderLinuxDoUserId(item),
      content: String(item?.content || '').trim()
    }
    muteDurationMinutes.value = 5
    muteReason.value = ''
    showMuteDialog.value = true
  }

  const closeMuteDialog = () => {
    showMuteDialog.value = false
    muteSubmitting.value = false
    muteReason.value = ''
    muteDurationMinutes.value = 5
    muteTarget.value = null
  }

  const submitMute = async () => {
    if (!canModerateChat.value) {
      message.error('当前角色无聊天管理权限')
      return
    }
    const targetLinuxDoUserId = String(muteTarget.value?.senderLinuxDoUserId || '').trim()
    if (!targetLinuxDoUserId) {
      message.warning('目标 LinuxDo ID 缺失，无法禁言')
      return
    }
    const durationMinutes = Math.max(1, Math.floor(Number(muteDurationMinutes.value || 0)))
    if (!durationMinutes) {
      message.warning('请选择有效禁言时长')
      return
    }

    muteSubmitting.value = true
    try {
      const result = await chatStore.adminMute(targetLinuxDoUserId, durationMinutes, muteReason.value.trim())
      message.success(result?.message || '禁言成功')
      closeMuteDialog()
      await chatStore.loadAdminMutes({ targetLinuxDoUserId, limit: 20, silentForbidden: true }).catch(() => {})
    } catch (error) {
      message.error(error?.message || '禁言失败')
    } finally {
      muteSubmitting.value = false
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
    overflow: hidden;
    display: flex;
    flex-direction: column;
  }

  .chat-dock:not(.collapsed) {
    height: 220px;
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
    display: flex;
    flex-direction: column;
    gap: 6px;
    flex: 1;
    min-height: 0;
    overflow: hidden;
  }

  .chat-stream {
    flex: 1;
    min-height: 0;
    height: auto;
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
    align-items: center;
    justify-content: space-between;
    gap: 8px;
    padding: 6px 2px;
    border-bottom: 1px dashed color-mix(in srgb, var(--panel-border) 85%, transparent);
  }

  .chat-line:last-child {
    border-bottom: none;
  }

  .chat-line-main {
    min-width: 0;
    flex: 1;
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 12px;
  }

  .chat-line-content {
    flex: 1;
    color: var(--ink-main);
    font-size: 13px;
    line-height: 1.45;
    word-break: break-word;
  }

  .chat-line-sender {
    border: none;
    background: transparent;
    padding: 0;
    margin: 0 4px 0 0;
    color: var(--accent-primary);
    cursor: pointer;
    font-weight: 700;
  }

  .chat-line-time {
    flex-shrink: 0;
    color: var(--ink-sub);
    font-size: 12px;
    line-height: 1.4;
  }

  .chat-line-report {
    flex-shrink: 0;
  }

  .chat-line-actions {
    display: flex;
    align-items: center;
    gap: 8px;
    flex-shrink: 0;
  }

  .chat-line-mute {
    color: #d03050;
  }

  .chat-compose {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    gap: 8px;
    align-items: center;
    flex-shrink: 0;
  }

  .chat-status {
    min-height: 0;
  }

  .chat-muted,
  .chat-error {
    margin: 0;
    font-size: 12px;
    line-height: 1.4;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .chat-muted {
    color: #d89614;
  }

  .chat-error {
    color: #d03050;
  }

  .mute-dialog-content {
    color: var(--ink-main);
    white-space: pre-wrap;
    word-break: break-word;
  }

  .mute-dialog-field {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .mute-dialog-label {
    font-size: 12px;
    color: var(--ink-sub);
  }

  @media (max-width: 768px) {
    .chat-dock {
      bottom: 6px;
      width: calc(100vw - 12px);
      border-radius: 12px;
    }

    .chat-dock:not(.collapsed) {
      height: 46vh;
      max-height: 340px;
    }

    .chat-head {
      padding: 8px;
    }

    .chat-title-wrap {
      font-size: 13px;
    }

    .chat-line {
      flex-direction: column;
      align-items: stretch;
      gap: 4px;
    }

    .chat-line-main {
      flex-direction: column;
      gap: 4px;
    }

    .chat-line-time {
      align-self: flex-end;
    }

    .chat-line-actions {
      justify-content: flex-end;
    }

    .chat-compose {
      grid-template-columns: 1fr;
    }

    .chat-compose :deep(.n-button) {
      width: 100%;
    }

    .chat-muted,
    .chat-error {
      white-space: normal;
    }
  }

  @media (max-width: 480px) {
    .chat-body {
      padding: 6px 8px 8px;
    }

    .chat-stream {
      padding: 4px 6px;
    }
  }
</style>
