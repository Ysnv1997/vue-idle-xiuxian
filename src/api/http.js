import { API_BASE_URL } from '../config/features'
import { clearTokens, getAccessToken, getRefreshToken, setTokens } from './token-storage'

let refreshPromise = null

async function refreshAccessToken() {
  if (refreshPromise) {
    return refreshPromise
  }

  const refreshToken = getRefreshToken()
  if (!refreshToken) {
    return false
  }

  refreshPromise = fetch(`${API_BASE_URL}/auth/refresh`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json'
    },
    body: JSON.stringify({ refreshToken })
  })
    .then(async response => {
      if (!response.ok) {
        clearTokens()
        return false
      }
      const payload = await response.json()
      if (!payload?.token) {
        clearTokens()
        return false
      }
      setTokens(payload.token)
      return true
    })
    .catch(() => {
      clearTokens()
      return false
    })
    .finally(() => {
      refreshPromise = null
    })

  return refreshPromise
}

function parseJWTExp(token) {
  const raw = String(token || '').trim()
  if (!raw) return 0
  const parts = raw.split('.')
  if (parts.length < 2) return 0
  const payloadPart = String(parts[1] || '').trim()
  if (!payloadPart) return 0
  try {
    const normalized = payloadPart.replace(/-/g, '+').replace(/_/g, '/')
    const padded = normalized.padEnd(Math.ceil(normalized.length / 4) * 4, '=')
    const payloadJSON = atob(padded)
    const payload = JSON.parse(payloadJSON)
    const exp = Number(payload?.exp || 0)
    if (!Number.isFinite(exp) || exp <= 0) {
      return 0
    }
    return Math.floor(exp)
  } catch {
    return 0
  }
}

export function accessTokenExpiresInSeconds(token = getAccessToken()) {
  const exp = parseJWTExp(token)
  if (!exp) return 0
  const nowSeconds = Math.floor(Date.now() / 1000)
  return Math.max(0, exp - nowSeconds)
}

export async function ensureAccessTokenValid(minTTLSeconds = 120) {
  const accessToken = getAccessToken()
  if (accessToken) {
    const remainSeconds = accessTokenExpiresInSeconds(accessToken)
    if (remainSeconds > Math.max(0, Math.floor(Number(minTTLSeconds || 0)))) {
      return true
    }
  }

  const refreshed = await refreshAccessToken()
  if (!refreshed) {
    return false
  }
  return Boolean(getAccessToken())
}

export async function httpRequest(path, options = {}) {
  const {
    method = 'GET',
    body,
    headers = {},
    auth = true,
    retryOnAuthFailure = true,
    idempotencyKey = ''
  } = options

  const requestHeaders = {
    ...headers
  }

  if (auth) {
    const accessToken = getAccessToken()
    if (accessToken) {
      requestHeaders.Authorization = `Bearer ${accessToken}`
    }
  }

  if (idempotencyKey) {
    requestHeaders['X-Idempotency-Key'] = idempotencyKey
  }

  let requestBody
  if (body !== undefined && body !== null) {
    requestHeaders['Content-Type'] = 'application/json'
    requestBody = JSON.stringify(body)
  }

  const response = await fetch(`${API_BASE_URL}${path}`, {
    method,
    headers: requestHeaders,
    body: requestBody
  })

  if (response.status === 401 && auth && retryOnAuthFailure) {
    const refreshed = await refreshAccessToken()
    if (refreshed) {
      return httpRequest(path, {
        ...options,
        retryOnAuthFailure: false
      })
    }
  }

  if (!response.ok) {
    const errorPayload = await safeParseJSON(response)
    const message = errorPayload?.error || `HTTP ${response.status}`
    const error = new Error(message)
    error.status = response.status
    error.payload = errorPayload
    throw error
  }

  if (response.status === 204) {
    return null
  }

  return safeParseJSON(response)
}

async function safeParseJSON(response) {
  try {
    return await response.json()
  } catch {
    return null
  }
}
