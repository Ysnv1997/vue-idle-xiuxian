import { defineStore } from 'pinia'
import { API_BASE_URL, AUTO_DEV_LOGIN } from '../config/features'
import { devLogin, fetchMe, logout } from '../api/modules/auth'
import { clearTokens, getAccessToken, getRefreshToken, setTokens } from '../api/token-storage'

export const useSessionStore = defineStore('session', {
  state: () => ({
    accessToken: '',
    refreshToken: '',
    user: null,
    loading: false
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
    clearSession() {
      clearTokens()
      this.accessToken = ''
      this.refreshToken = ''
      this.user = null
    },
    async initializeSession() {
      this.hydrateTokens()
      if (!this.accessToken && AUTO_DEV_LOGIN) {
        await this.loginAsDev()
      }

      if (!this.accessToken) {
        return
      }

      this.loading = true
      try {
        this.user = await fetchMe()
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
