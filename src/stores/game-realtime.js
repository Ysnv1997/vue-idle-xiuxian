import { defineStore } from 'pinia'
import { buildGameRealtimeWSURL } from '../api/modules/game'
import { ensureAccessTokenValid } from '../api/http'
import { getAccessToken, getRefreshToken } from '../api/token-storage'
import { usePlayerStore } from './player'

export const useGameRealtimeStore = defineStore('game-realtime', {
  state: () => ({
    ws: null,
    connected: false,
    connecting: false,
    lastError: '',
    lastSyncAt: 0,
    worldAnnouncements: [],
    activeWorldAnnouncement: null,
    announcementTimer: null,
    meditationRun: null,
    huntingRun: null,
    explorationRun: null,
    reconnectTimer: null,
    reconnectDelayMs: 3000
  }),
  actions: {
    async connect() {
      if (this.ws || this.connecting) return

      this.connecting = true
      this.lastError = ''
      const valid = await ensureAccessTokenValid(90)
      if (!this.connecting) {
        return
      }
      const accessToken = getAccessToken()
      if (!valid || !accessToken) {
        this.connecting = false
        this.connected = false
        this.lastError = '登录状态已过期，请重新登录'
        return
      }
      const ws = new WebSocket(buildGameRealtimeWSURL(accessToken))

      ws.onopen = () => {
        this.clearReconnectTimer()
        this.ws = ws
        this.connecting = false
        this.connected = true
      }

      ws.onclose = () => {
        if (this.ws === ws) {
          this.ws = null
        }
        this.connected = false
        this.connecting = false
        this.scheduleReconnect()
      }

      ws.onerror = () => {
        this.lastError = '游戏状态连接失败'
      }

      ws.onmessage = event => {
        this.handleIncoming(event.data)
      }
    },
    disconnect() {
      this.clearReconnectTimer()
      if (this.ws) {
        const current = this.ws
        this.ws = null
        current.close()
      }
      this.connected = false
      this.connecting = false
      this.lastSyncAt = 0
      this.clearAnnouncementTimer()
      this.worldAnnouncements = []
      this.activeWorldAnnouncement = null
      this.meditationRun = null
      this.huntingRun = null
      this.explorationRun = null
    },
    scheduleReconnect() {
      if (this.reconnectTimer) return
      if (!getAccessToken() && !getRefreshToken()) return
      this.reconnectTimer = window.setTimeout(() => {
        this.reconnectTimer = null
        this.connect()
      }, this.reconnectDelayMs)
    },
    clearReconnectTimer() {
      if (!this.reconnectTimer) return
      window.clearTimeout(this.reconnectTimer)
      this.reconnectTimer = null
    },
    clearAnnouncementTimer() {
      if (!this.announcementTimer) return
      window.clearTimeout(this.announcementTimer)
      this.announcementTimer = null
    },
    enqueueWorldAnnouncement(announcement) {
      if (!announcement?.message) return
      this.worldAnnouncements.push(announcement)
      this.flushWorldAnnouncement()
    },
    flushWorldAnnouncement() {
      if (this.activeWorldAnnouncement || this.worldAnnouncements.length === 0) return
      this.activeWorldAnnouncement = this.worldAnnouncements.shift()
      this.clearAnnouncementTimer()
      this.announcementTimer = window.setTimeout(() => {
        this.activeWorldAnnouncement = null
        this.announcementTimer = null
        this.flushWorldAnnouncement()
      }, 9000)
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
      const playerStore = usePlayerStore()

      if (event === 'game.error') {
        this.lastError = data?.error || '游戏状态同步失败'
        return
      }

      if (event === 'world.announcement' && data && typeof data === 'object') {
        this.enqueueWorldAnnouncement(data)
        return
      }

       this.lastSyncAt = Date.now()

      if (event === 'player.snapshot' && data && typeof data === 'object') {
        playerStore.applyServerSnapshot(data)
        return
      }

      if (event === 'player.delta' && data && typeof data === 'object') {
        playerStore.applyServerDelta(data)
        return
      }

      if (event === 'game.meditation') {
        this.meditationRun = data || null
        return
      }

      if (event === 'game.hunting') {
        this.huntingRun = data || null
        return
      }

      if (event === 'game.exploration') {
        this.explorationRun = data || null
      }
    }
  }
})
