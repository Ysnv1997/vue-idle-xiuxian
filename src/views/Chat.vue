<template>
  <div class="chat-container">
    <n-card title="聊天">
      <template #header-extra>
        <n-space>
          <n-tag :type="chatStore.connected ? 'success' : 'warning'" size="small">
            {{ chatStore.connected ? '已连接' : '未连接' }}
          </n-tag>
          <n-button size="small" :loading="chatStore.loadingHistory" @click="reloadHistory">刷新历史</n-button>
          <n-button v-if="!chatStore.connected" size="small" :loading="chatStore.connecting" @click="connect">连接</n-button>
          <n-button v-else size="small" @click="disconnect">断开</n-button>
        </n-space>
      </template>
      <n-space vertical>
        <n-alert v-if="chatStore.lastError" type="error" :show-icon="false">
          {{ chatStore.lastError }}
        </n-alert>
        <n-alert v-if="chatStore.muteStatus?.muted" type="warning" :show-icon="false">
          当前禁言至 {{ formatTime(chatStore.muteStatus.mutedUntil) }}
          <span v-if="chatStore.muteStatus.reason">，原因：{{ chatStore.muteStatus.reason }}</span>
        </n-alert>

        <n-card v-if="chatStore.adminEnabled" size="small" title="聊天管理">
          <template #header-extra>
            <n-button
              size="small"
              :loading="chatStore.loadingAdminMutes || chatStore.loadingAdminBlockedWords"
              @click="reloadAdminData"
            >
              刷新管理数据
            </n-button>
          </template>

          <n-space vertical>
            <n-grid :cols="24" :x-gap="10" :y-gap="10">
              <n-grid-item :span="8">
                <n-input v-model:value="adminTargetLinuxDoUserId" placeholder="目标 LinuxDo 用户ID" />
              </n-grid-item>
              <n-grid-item :span="5">
                <n-input-number v-model:value="adminDurationMinutes" :min="1" :max="10080" style="width: 100%" />
              </n-grid-item>
              <n-grid-item :span="7">
                <n-input v-model:value="adminReason" placeholder="禁言原因(可选)" />
              </n-grid-item>
                <n-grid-item :span="4">
                  <n-space justify="end">
                    <n-button type="warning" :loading="adminSubmitting" @click="submitMute">禁言</n-button>
                    <n-button tertiary :loading="adminSubmitting" @click="submitUnmuteByInput">解禁</n-button>
                  </n-space>
                </n-grid-item>
            </n-grid>

            <n-table striped size="small">
              <thead>
                <tr>
                  <th style="width: 120px">目标ID</th>
                  <th style="width: 120px">目标昵称</th>
                  <th style="width: 160px">禁言至</th>
                  <th>原因</th>
                  <th style="width: 120px">操作人</th>
                  <th style="width: 90px">操作</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="mute in chatStore.adminMutes" :key="mute.id">
                  <td>{{ mute.targetLinuxDoUserId || '-' }}</td>
                  <td>{{ mute.targetName || '-' }}</td>
                  <td>{{ formatTime(mute.mutedUntil) }}</td>
                  <td>{{ mute.reason || '-' }}</td>
                  <td>{{ mute.createdByLinuxDoUserId || '-' }}</td>
                  <td>
                    <n-button size="tiny" tertiary :loading="adminSubmitting" @click="submitUnmute(mute.targetLinuxDoUserId)">
                      解禁
                    </n-button>
                  </td>
                </tr>
                <tr v-if="chatStore.adminMutes.length === 0">
                  <td colspan="6">
                    <n-empty description="暂无生效中的禁言记录" />
                  </td>
                </tr>
              </tbody>
            </n-table>

            <n-grid :cols="24" :x-gap="10" :y-gap="10">
              <n-grid-item :span="10">
                <n-input v-model:value="adminBlockedWord" placeholder="新增或更新违禁词" />
              </n-grid-item>
              <n-grid-item :span="5">
                <n-select v-model:value="adminBlockedWordEnabled" :options="blockedWordStatusOptions" />
              </n-grid-item>
              <n-grid-item :span="9">
                <n-space justify="end">
                  <n-button type="primary" :loading="adminWordSubmitting" @click="submitBlockedWord">
                    保存违禁词
                  </n-button>
                  <n-button tertiary :loading="chatStore.loadingAdminBlockedWords" @click="reloadBlockedWords">
                    刷新违禁词
                  </n-button>
                </n-space>
              </n-grid-item>
            </n-grid>

            <n-table striped size="small">
              <thead>
                <tr>
                  <th style="width: 200px">违禁词</th>
                  <th style="width: 90px">状态</th>
                  <th style="width: 170px">更新时间</th>
                  <th style="width: 180px">操作</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="item in chatStore.adminBlockedWords" :key="item.word">
                  <td>{{ item.word || '-' }}</td>
                  <td>{{ item.enabled ? '启用' : '停用' }}</td>
                  <td>{{ formatTime(item.updatedAt) }}</td>
                  <td>
                    <n-space>
                      <n-button
                        size="tiny"
                        tertiary
                        :loading="adminWordSubmitting"
                        @click="toggleBlockedWord(item)"
                      >
                        {{ item.enabled ? '停用' : '启用' }}
                      </n-button>
                      <n-button
                        size="tiny"
                        tertiary
                        type="error"
                        :loading="adminWordSubmitting"
                        @click="removeBlockedWord(item.word)"
                      >
                        删除
                      </n-button>
                    </n-space>
                  </td>
                </tr>
                <tr v-if="chatStore.adminBlockedWords.length === 0">
                  <td colspan="4">
                    <n-empty description="暂无违禁词配置" />
                  </td>
                </tr>
              </tbody>
            </n-table>
          </n-space>
        </n-card>

        <div class="chat-log">
          <n-empty v-if="chatStore.messages.length === 0" description="暂无消息" />
          <div v-for="item in chatStore.messages" :key="item.id" class="chat-message">
            <div class="chat-meta">
              <span class="sender">{{ item.senderName || '未知修士' }}</span>
              <span class="time">{{ formatTime(item.createdAt) }}</span>
            </div>
            <div class="chat-content">{{ item.content }}</div>
            <div class="chat-actions">
              <n-button text size="tiny" @click="reportMessage(item.id)">举报</n-button>
            </div>
          </div>
        </div>

        <n-space align="center">
          <n-input
            v-model:value="draft"
            placeholder="输入消息，按 Enter 发送"
            :disabled="!chatStore.connected"
            @keyup.enter="send"
          />
          <n-button type="primary" :disabled="!chatStore.connected" @click="send">发送</n-button>
        </n-space>
      </n-space>
    </n-card>
  </div>
</template>

<script setup>
  import { onMounted, onUnmounted, ref } from 'vue'
  import { useDialog, useMessage } from 'naive-ui'
  import { useChatStore } from '../stores/chat'

  const message = useMessage()
  const dialog = useDialog()
  const chatStore = useChatStore()

  const draft = ref('')
  const adminSubmitting = ref(false)
  const adminWordSubmitting = ref(false)
  const adminTargetLinuxDoUserId = ref('')
  const adminDurationMinutes = ref(30)
  const adminReason = ref('')
  const adminBlockedWord = ref('')
  const adminBlockedWordEnabled = ref(true)

  const blockedWordStatusOptions = [
    { label: '启用', value: true },
    { label: '停用', value: false }
  ]

  const connect = async () => {
    chatStore.connect()
  }

  const disconnect = () => {
    chatStore.disconnect()
  }

  const reloadHistory = async () => {
    try {
      await Promise.all([chatStore.loadHistory(), chatStore.loadMuteStatus()])
    } catch (error) {
      message.error(error?.message || '加载历史消息失败')
    }
  }

  const reloadAdminMutes = async () => {
    try {
      await chatStore.loadAdminMutes({
        targetLinuxDoUserId: adminTargetLinuxDoUserId.value.trim(),
        limit: 100,
        silentForbidden: false
      })
    } catch (error) {
      message.error(error?.message || '加载禁言列表失败')
    }
  }

  const reloadBlockedWords = async () => {
    try {
      await chatStore.loadAdminBlockedWords({
        includeDisabled: true,
        limit: 200,
        silentForbidden: false
      })
    } catch (error) {
      message.error(error?.message || '加载违禁词失败')
    }
  }

  const reloadAdminData = async () => {
    await Promise.all([reloadAdminMutes(), reloadBlockedWords()])
  }

  const send = () => {
    if (!draft.value.trim()) {
      message.warning('消息不能为空')
      return
    }
    const success = chatStore.sendMessage(draft.value)
    if (success) {
      draft.value = ''
    }
  }

  const reportMessage = messageId => {
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

  const submitMute = async () => {
    const targetLinuxDoUserId = adminTargetLinuxDoUserId.value.trim()
    if (!targetLinuxDoUserId) {
      message.warning('请输入目标 LinuxDo 用户ID')
      return
    }

    const durationMinutes = Number(adminDurationMinutes.value)
    if (!Number.isFinite(durationMinutes) || durationMinutes <= 0) {
      message.warning('请输入有效的禁言时长(分钟)')
      return
    }

    adminSubmitting.value = true
    try {
      const result = await chatStore.adminMute(targetLinuxDoUserId, Math.floor(durationMinutes), adminReason.value.trim())
      message.success(result?.message || '禁言成功')
      await reloadAdminMutes()
    } catch (error) {
      message.error(error?.message || '禁言失败')
    } finally {
      adminSubmitting.value = false
    }
  }

  const submitUnmute = async targetLinuxDoUserId => {
    if (!targetLinuxDoUserId) {
      message.warning('目标用户ID为空')
      return
    }

    adminSubmitting.value = true
    try {
      const result = await chatStore.adminUnmute(String(targetLinuxDoUserId).trim())
      message.success(result?.message || '解除禁言成功')
      await reloadAdminMutes()
    } catch (error) {
      message.error(error?.message || '解除禁言失败')
    } finally {
      adminSubmitting.value = false
    }
  }

  const submitUnmuteByInput = async () => {
    await submitUnmute(adminTargetLinuxDoUserId.value.trim())
  }

  const submitBlockedWord = async () => {
    const word = adminBlockedWord.value.trim()
    if (!word) {
      message.warning('请输入违禁词')
      return
    }

    adminWordSubmitting.value = true
    try {
      const result = await chatStore.adminUpsertBlockedWord(word, Boolean(adminBlockedWordEnabled.value))
      message.success(result?.message || '违禁词更新成功')
      adminBlockedWord.value = ''
      await reloadBlockedWords()
    } catch (error) {
      message.error(error?.message || '违禁词更新失败')
    } finally {
      adminWordSubmitting.value = false
    }
  }

  const toggleBlockedWord = async item => {
    const word = String(item?.word || '').trim()
    if (!word) return

    adminWordSubmitting.value = true
    try {
      await chatStore.adminUpsertBlockedWord(word, !Boolean(item?.enabled))
      message.success('违禁词状态已更新')
      await reloadBlockedWords()
    } catch (error) {
      message.error(error?.message || '违禁词状态更新失败')
    } finally {
      adminWordSubmitting.value = false
    }
  }

  const removeBlockedWord = async wordRaw => {
    const word = String(wordRaw || '').trim()
    if (!word) return

    adminWordSubmitting.value = true
    try {
      await chatStore.adminDeleteBlockedWord(word)
      message.success('违禁词已删除')
      await reloadBlockedWords()
    } catch (error) {
      message.error(error?.message || '违禁词删除失败')
    } finally {
      adminWordSubmitting.value = false
    }
  }

  const formatTime = value => {
    if (!value) return '-'
    const date = new Date(value)
    if (Number.isNaN(date.getTime())) return '-'
    return date.toLocaleString()
  }

  onMounted(async () => {
    chatStore.connect()
    await reloadHistory()
    try {
      await chatStore.loadAdminMutes({ silentForbidden: true, limit: 100 })
      await chatStore.loadAdminBlockedWords({ silentForbidden: true, limit: 200, includeDisabled: true })
    } catch {
      // handled in store
    }
  })

  onUnmounted(() => {
    chatStore.disconnect()
  })
</script>

<style scoped>
  .chat-container {
    margin: 0 auto;
  }

  .chat-log {
    border: 1px solid rgba(127, 127, 127, 0.2);
    border-radius: 6px;
    min-height: 360px;
    max-height: 480px;
    overflow-y: auto;
    padding: 10px;
  }

  .chat-message {
    padding: 8px 10px;
    border-bottom: 1px solid rgba(127, 127, 127, 0.15);
  }

  .chat-message:last-child {
    border-bottom: none;
  }

  .chat-meta {
    display: flex;
    justify-content: space-between;
    font-size: 12px;
    opacity: 0.75;
    margin-bottom: 4px;
  }

  .sender {
    font-weight: 600;
  }

  .chat-content {
    line-height: 1.5;
    white-space: pre-wrap;
    word-break: break-word;
  }

  .chat-actions {
    margin-top: 4px;
  }
</style>
