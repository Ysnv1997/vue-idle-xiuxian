import { API_BASE_URL } from '../../config/features'
import { httpRequest } from '../http'

export async function fetchChatHistory(channel = 'world', limit = 50) {
  const params = new URLSearchParams()
  params.set('channel', channel)
  params.set('limit', String(limit))
  return httpRequest(`/chat/history?${params.toString()}`)
}

export async function reportChatMessage(messageId, reason = '') {
  return httpRequest('/chat/report', {
    method: 'POST',
    body: { messageId, reason }
  })
}

export async function fetchChatMuteStatus() {
  return httpRequest('/chat/mute-status')
}

export async function fetchChatAdminMutes(targetLinuxDoUserId = '', limit = 50) {
  const params = new URLSearchParams()
  if (targetLinuxDoUserId) {
    params.set('targetLinuxDoUserId', String(targetLinuxDoUserId))
  }
  params.set('limit', String(limit))
  return httpRequest(`/chat/admin/mutes?${params.toString()}`)
}

export async function muteChatUser(targetLinuxDoUserId, durationMinutes, reason = '') {
  return httpRequest('/chat/admin/mute', {
    method: 'POST',
    body: { targetLinuxDoUserId, durationMinutes, reason }
  })
}

export async function unmuteChatUser(targetLinuxDoUserId) {
  return httpRequest('/chat/admin/unmute', {
    method: 'POST',
    body: { targetLinuxDoUserId }
  })
}

export async function fetchChatBlockedWords(includeDisabled = true, limit = 200) {
  const params = new URLSearchParams()
  params.set('includeDisabled', includeDisabled ? 'true' : 'false')
  params.set('limit', String(limit))
  return httpRequest(`/chat/admin/block-words?${params.toString()}`)
}

export async function upsertChatBlockedWord(word, enabled = true) {
  return httpRequest('/chat/admin/block-words', {
    method: 'POST',
    body: { word, enabled }
  })
}

export async function deleteChatBlockedWord(word) {
  const params = new URLSearchParams()
  params.set('word', String(word))
  return httpRequest(`/chat/admin/block-words?${params.toString()}`, {
    method: 'DELETE'
  })
}

export function buildChatWSURL(accessToken) {
  const origin = API_BASE_URL.replace(/^http:/, 'ws:').replace(/^https:/, 'wss:')
  const params = new URLSearchParams()
  params.set('accessToken', accessToken)
  return `${origin}/chat/connect?${params.toString()}`
}
