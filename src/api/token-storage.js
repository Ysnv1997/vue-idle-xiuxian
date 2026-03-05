const ACCESS_TOKEN_KEY = 'xiuxian:access-token'
const REFRESH_TOKEN_KEY = 'xiuxian:refresh-token'

export function getAccessToken() {
  return localStorage.getItem(ACCESS_TOKEN_KEY) || ''
}

export function getRefreshToken() {
  return localStorage.getItem(REFRESH_TOKEN_KEY) || ''
}

export function setTokens(tokenPair) {
  if (!tokenPair) return
  if (tokenPair.accessToken) {
    localStorage.setItem(ACCESS_TOKEN_KEY, tokenPair.accessToken)
  }
  if (tokenPair.refreshToken) {
    localStorage.setItem(REFRESH_TOKEN_KEY, tokenPair.refreshToken)
  }
}

export function clearTokens() {
  localStorage.removeItem(ACCESS_TOKEN_KEY)
  localStorage.removeItem(REFRESH_TOKEN_KEY)
}
