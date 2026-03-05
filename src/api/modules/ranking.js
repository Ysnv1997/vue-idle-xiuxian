import { httpRequest } from '../http'

export async function fetchRankings(type = 'realm', limit = 50, scope = 'global') {
  const safeType = encodeURIComponent(type || 'realm')
  const safeScope = encodeURIComponent(scope || 'global')
  const safeLimit = Number.isFinite(limit) ? Math.max(1, Math.min(100, Number(limit))) : 50
  return httpRequest(`/rankings?type=${safeType}&scope=${safeScope}&limit=${safeLimit}`)
}

export async function fetchSelfRanking(type = 'realm', scope = 'global') {
  const safeType = encodeURIComponent(type || 'realm')
  const safeScope = encodeURIComponent(scope || 'global')
  return httpRequest(`/rankings/self?type=${safeType}&scope=${safeScope}`)
}

export async function fetchRankingFollows(limit = 100) {
  const safeLimit = Number.isFinite(limit) ? Math.max(1, Math.min(200, Number(limit))) : 100
  return httpRequest(`/rankings/follows?limit=${safeLimit}`)
}

export async function followRankingUser(targetUserId) {
  return httpRequest('/rankings/follows', {
    method: 'POST',
    body: { targetUserId }
  })
}

export async function unfollowRankingUser(targetUserId) {
  const safeTarget = encodeURIComponent(targetUserId || '')
  return httpRequest(`/rankings/follows?targetUserId=${safeTarget}`, {
    method: 'DELETE'
  })
}
