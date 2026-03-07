import { defineStore } from 'pinia'
import { API_BASE_URL, AUTO_DEV_LOGIN } from '../config/features'
import { devLogin, fetchMe, logout } from '../api/modules/auth'
import { ensureAccessTokenValid } from '../api/http'
import { clearTokens, getAccessToken, getRefreshToken, setTokens } from '../api/token-storage'

export const useSessionStore = defineStore('session', {
  state: () => ({
    accessToken: '',
    refreshToken: '',
    user: null,
    loading: false,
    tokenHeartbeatTimer: null,
    tokenHeartbeatIntervalMs: 5 * 60 * 1000
  }),
  getters: {
    isAuthenticated(state) {
      return Boolean(state.accessToken)
    }
  },
  actions: {
    hydrateTokens() {
      this.accessToken = getAccessToken()
      this.refreshToken = getRefreshToken()
    },
    startTokenHeartbeat() {
      if (this.tokenHeartbeatTimer || typeof window === 'undefined') {
        return
      }
      this.tokenHeartbeatTimer = window.setInterval(() => {
        this.keepSessionAlive({ silent: true })
      }, this.tokenHeartbeatIntervalMs)
    },
    stopTokenHeartbeat() {
      if (!this.tokenHeartbeatTimer || typeof window === 'undefined') {
        return
      }
      window.clearInterval(this.tokenHeartbeatTimer)
      this.tokenHeartbeatTimer = null
    },
    async keepSessionAlive({ silent = true } = {}) {
      if (!this.accessToken && !this.refreshToken) {
        return false
      }
      try {
        const ok = await ensureAccessTokenValid(20 * 60)
        this.hydrateTokens()
        if (!ok) {
          this.clearSession()
          return false
        }
        return true
      } catch (error) {
        if (!silent) {
          throw error
        }
        return false
      }
    },
    clearSession() {
      this.stopTokenHeartbeat()
      clearTokens()
      this.accessToken = ''
      this.refreshToken = ''
      this.user = null
    },
    async initializeSession() {
      this.hydrateTokens()
      if (!this.accessToken && this.refreshToken) {
        await ensureAccessTokenValid(60)
        this.hydrateTokens()
      }
      if (!this.accessToken && AUTO_DEV_LOGIN) {
        await this.loginAsDev()
      }

      if (!this.accessToken) {
        return
      }

      this.loading = true
      try {
        this.user = await fetchMe()
        this.startTokenHeartbeat()
      } catch (error) {
        this.clearSession()
        throw error
      } finally {
        this.loading = false
      }
    },
    async loginAsDev(payload = {}) {
      this.loading = true
      try {
        await devLogin(payload)
        this.hydrateTokens()
        this.user = await fetchMe()
        this.startTokenHeartbeat()
      } finally {
        this.loading = false
      }
    },
    redirectToLinuxDoLogin() {
      const callbackURL = `${window.location.origin}${window.location.pathname}#/auth/callback`
      const authorizeURL = new URL(`${API_BASE_URL}/auth/linux-do/authorize`, window.location.origin)
      authorizeURL.searchParams.set('redirect', callbackURL)
      window.location.href = authorizeURL.toString()
    },
    async consumeOAuthCallback(query = {}) {
      const accessToken = normalizeQueryValue(query.accessToken)
      const refreshToken = normalizeQueryValue(query.refreshToken)
      const error = normalizeQueryValue(query.error)

      if (error) {
        throw new Error(error)
      }
      if (!accessToken || !refreshToken) {
        return false
      }

      setTokens({ accessToken, refreshToken })
      this.hydrateTokens()

      this.loading = true
      try {
        this.user = await fetchMe()
        this.startTokenHeartbeat()
      } catch (consumeError) {
        this.clearSession()
        throw consumeError
      } finally {
        this.loading = false
      }

      return true
    },
    async logout() {
      let remoteLoggedOut = true
      try {
        await logout()
      } catch (error) {
        remoteLoggedOut = false
      } finally {
        this.clearSession()
      }
      return remoteLoggedOut
    }
  }
})

function normalizeQueryValue(value) {
  if (Array.isArray(value)) {
    return value[0] || ''
  }
  if (typeof value === 'string') {
    return value
  }
  return ''
}
