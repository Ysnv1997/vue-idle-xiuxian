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
