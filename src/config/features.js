const configuredApiBaseURL = import.meta.env.VITE_API_BASE_URL

function normalizeBaseURL(value) {
  const trimmed = String(value || '').trim()
  if (!trimmed) {
    return '/api/v1'
  }
  return trimmed.replace(/\/+$/, '')
}

export const API_BASE_URL = normalizeBaseURL(configuredApiBaseURL)
export const AUTO_DEV_LOGIN = import.meta.env.VITE_AUTO_DEV_LOGIN === 'true'
