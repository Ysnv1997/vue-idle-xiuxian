import { httpRequest } from '../http'
import { clearTokens, setTokens } from '../token-storage'

export async function devLogin(payload = {}) {
  const response = await httpRequest('/auth/dev/login', {
    method: 'POST',
    auth: false,
    body: payload
  })
  if (response?.token) {
    setTokens(response.token)
  }
  return response
}

export async function fetchMe() {
  return httpRequest('/auth/me')
}

export async function refreshSession(refreshToken) {
  const response = await httpRequest('/auth/refresh', {
    method: 'POST',
    auth: false,
    body: { refreshToken }
  })
  if (response?.token) {
    setTokens(response.token)
  }
  return response
}

export async function logout() {
  try {
    await httpRequest('/auth/logout', {
      method: 'POST'
    })
  } finally {
    clearTokens()
  }
}
