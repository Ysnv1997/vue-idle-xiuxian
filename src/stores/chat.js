import { defineStore } from 'pinia'
import {
  buildChatWSURL,
  deleteChatBlockedWord,
  fetchChatAdminMutes,
  fetchChatBlockedWords,
  fetchChatHistory,
  fetchChatMuteStatus,
  muteChatUser,
  reportChatMessage,
  upsertChatBlockedWord,
  unmuteChatUser
} from '../api/modules/chat'
import { getAccessToken } from '../api/token-storage'

export const useChatStore = defineStore('chat', {
  state: () => ({
    channel: 'world',
    messages: [],
    connected: false,
    connecting: false,
    loadingHistory: false,
    muteStatus: {
      muted: false,
      mutedUntil: '',
      reason: ''
    },
    adminChecked: false,
    adminEnabled: false,
    loadingAdminMutes: false,
    adminMutes: [],
    loadingAdminBlockedWords: false,
    adminBlockedWords: [],
    lastError: '',
    ws: null
  }),
  actions: {
    async loadHistory(limit = 50) {
      this.loadingHistory = true
      this.lastError = ''
      try {
        const result = await fetchChatHistory(this.channel, limit)
        this.messages = Array.isArray(result?.messages) ? result.messages : []
        return this.messages
      } catch (error) {
        this.lastError = error?.message || '加载聊天记录失败'
        throw error
      } finally {
        this.loadingHistory = false
      }
    },
    async loadMuteStatus() {
      try {
        const status = await fetchChatMuteStatus()
        this.muteStatus = {
          muted: Boolean(status?.muted),
          mutedUntil: status?.mutedUntil || '',
          reason: status?.reason || ''
        }
      } catch (error) {
        this.lastError = error?.message || '获取禁言状态失败'
        throw error
      }

      return this.muteStatus
    },
    connect() {
      if (this.ws || this.connecting) return

      const accessToken = getAccessToken()
      if (!accessToken) {
        this.lastError = '请先登录后再连接聊天'
        return
      }

      this.connecting = true
      this.lastError = ''
      const ws = new WebSocket(buildChatWSURL(accessToken))

      ws.onopen = () => {
        this.ws = ws
        this.connecting = false
        this.connected = true
      }

      ws.onclose = () => {
        if (this.ws === ws) {
          this.ws = null
        }
        this.connecting = false
        this.connected = false
      }

      ws.onerror = () => {
        this.lastError = '聊天连接失败'
      }

      ws.onmessage = event => {
        this.handleIncoming(event.data)
      }
    },
    disconnect() {
      if (this.ws) {
        this.ws.close()
        this.ws = null
      }
      this.connected = false
      this.connecting = false
    },
    handleIncoming(rawData) {
      let payload = null
      try {
        payload = JSON.parse(rawData)
      } catch {
        return
      }

      const event = payload?.event
      const data = payload?.data

      if (event === 'chat.error') {
        this.lastError = data?.error || '聊天发送失败'
        return
      }

      if (event === 'chat.receive' || event === 'chat.sent') {
        this.pushMessage(data)
      }
    },
    pushMessage(message) {
      if (!message || typeof message !== 'object') return
      if (!message.id) return

      const exists = this.messages.some(item => String(item.id) === String(message.id))
      if (!exists) {
        this.messages.push(message)
      }

      if (this.messages.length > 200) {
        this.messages = this.messages.slice(-200)
      }
    },
    sendMessage(content) {
      if (!this.ws || !this.connected) {
        this.lastError = '聊天未连接'
        return false
      }
      if (this.muteStatus?.muted) {
        this.lastError = '当前处于禁言状态，无法发送消息'
        return false
      }

      const text = String(content || '').trim()
      if (!text) {
        this.lastError = '消息不能为空'
        return false
      }

      this.ws.send(
        JSON.stringify({
          event: 'chat.send',
          data: {
            channel: this.channel,
            content: text
          }
        })
      )
      return true
    },
    async report(messageId, reason = '') {
      try {
        await reportChatMessage(messageId, reason)
      } catch (error) {
        this.lastError = error?.message || '举报失败'
        throw error
      }
    },
    async loadAdminMutes({ targetLinuxDoUserId = '', limit = 50, silentForbidden = true } = {}) {
      this.loadingAdminMutes = true
      try {
        const result = await fetchChatAdminMutes(targetLinuxDoUserId, limit)
        this.adminChecked = true
        this.adminEnabled = true
        this.adminMutes = Array.isArray(result?.mutes) ? result.mutes : []
        return this.adminMutes
      } catch (error) {
        if (error?.status === 403) {
          this.adminChecked = true
          this.adminEnabled = false
          this.adminMutes = []
          if (!silentForbidden) {
            this.lastError = error?.message || '无聊天管理权限'
            throw error
          }
          return []
        }
        this.lastError = error?.message || '加载禁言列表失败'
        throw error
      } finally {
        this.loadingAdminMutes = false
      }
    },
    async adminMute(targetLinuxDoUserId, durationMinutes, reason = '') {
      try {
        const result = await muteChatUser(targetLinuxDoUserId, durationMinutes, reason)
        this.adminChecked = true
        this.adminEnabled = true
        return result
      } catch (error) {
        if (error?.status === 403) {
          this.adminChecked = true
          this.adminEnabled = false
        }
        this.lastError = error?.message || '禁言操作失败'
        throw error
      }
    },
    async adminUnmute(targetLinuxDoUserId) {
      try {
        const result = await unmuteChatUser(targetLinuxDoUserId)
        this.adminChecked = true
        this.adminEnabled = true
        return result
      } catch (error) {
        if (error?.status === 403) {
          this.adminChecked = true
          this.adminEnabled = false
        }
        this.lastError = error?.message || '解除禁言失败'
        throw error
      }
    },
    async loadAdminBlockedWords({ includeDisabled = true, limit = 200, silentForbidden = true } = {}) {
      this.loadingAdminBlockedWords = true
      try {
        const result = await fetchChatBlockedWords(includeDisabled, limit)
        this.adminChecked = true
        this.adminEnabled = true
        this.adminBlockedWords = Array.isArray(result?.words) ? result.words : []
        return this.adminBlockedWords
      } catch (error) {
        if (error?.status === 403) {
          this.adminChecked = true
          this.adminEnabled = false
          this.adminBlockedWords = []
          if (!silentForbidden) {
            this.lastError = error?.message || '无聊天管理权限'
            throw error
          }
          return []
        }
        this.lastError = error?.message || '加载违禁词失败'
        throw error
      } finally {
        this.loadingAdminBlockedWords = false
      }
    },
    async adminUpsertBlockedWord(word, enabled = true) {
      try {
        const result = await upsertChatBlockedWord(word, enabled)
        this.adminChecked = true
        this.adminEnabled = true
        return result
      } catch (error) {
        if (error?.status === 403) {
          this.adminChecked = true
          this.adminEnabled = false
        }
        this.lastError = error?.message || '违禁词更新失败'
        throw error
      }
    },
    async adminDeleteBlockedWord(word) {
      try {
        const result = await deleteChatBlockedWord(word)
        this.adminChecked = true
        this.adminEnabled = true
        return result
      } catch (error) {
        if (error?.status === 403) {
          this.adminChecked = true
          this.adminEnabled = false
        }
        this.lastError = error?.message || '违禁词删除失败'
        throw error
      }
    }
  }
})
