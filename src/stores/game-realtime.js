import { defineStore } from 'pinia'
import { buildGameRealtimeWSURL } from '../api/modules/game'
import { getAccessToken } from '../api/token-storage'
import { usePlayerStore } from './player'

export const useGameRealtimeStore = defineStore('game-realtime', {
  state: () => ({
    ws: null,
    connected: false,
    connecting: false,
    lastError: '',
    lastSyncAt: 0,
    meditationRun: null,
    huntingRun: null,
    reconnectTimer: null,
    reconnectDelayMs: 3000
  }),
  actions: {
    connect() {
      if (this.ws || this.connecting) return

      const accessToken = getAccessToken()
      if (!accessToken) {
        this.lastError = '请先登录后再连接游戏状态'
        return
      }

      this.connecting = true
      this.lastError = ''
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
      this.meditationRun = null
      this.huntingRun = null
    },
    scheduleReconnect() {
      if (this.reconnectTimer) return
      if (!getAccessToken()) return
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
      }
    }
  }
})
