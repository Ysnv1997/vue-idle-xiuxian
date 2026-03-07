import { httpRequest } from '../http'

export async function fetchPlayerSnapshot() {
  return httpRequest('/player/snapshot')
}

export async function fetchActivePlayerCount() {
  return httpRequest('/player/active-count')
}

export async function fetchPublicPlayerProfile(userId) {
  const params = new URLSearchParams()
  params.set('userId', String(userId || ''))
  return httpRequest(`/player/public-profile?${params.toString()}`)
}
