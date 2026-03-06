import { httpRequest } from '../http'

export async function fetchAdminUsers(limit = 200) {
  const params = new URLSearchParams()
  params.set('limit', String(limit))
  return httpRequest(`/admin/users?${params.toString()}`)
}

export async function upsertAdminUser(linuxDoUserId, role = 'super_admin', note = '') {
  return httpRequest('/admin/users', {
    method: 'POST',
    body: {
      linuxDoUserId,
      role,
      note
    }
  })
}

export async function removeAdminUser(linuxDoUserId) {
  const params = new URLSearchParams()
  params.set('linuxDoUserId', linuxDoUserId)
  return httpRequest(`/admin/users?${params.toString()}`, {
    method: 'DELETE'
  })
}

export async function fetchRuntimeConfigs({ category = '', keyword = '', limit = 300 } = {}) {
  const params = new URLSearchParams()
  if (category) {
    params.set('category', category)
  }
  if (keyword) {
    params.set('q', keyword)
  }
  params.set('limit', String(limit))
  return httpRequest(`/admin/runtime-configs?${params.toString()}`)
}

export async function fetchRuntimeConfigAudits({ key = '', category = '', limit = 200 } = {}) {
  const params = new URLSearchParams()
  if (key) {
    params.set('key', key)
  }
  if (category) {
    params.set('category', category)
  }
  params.set('limit', String(limit))
  return httpRequest(`/admin/runtime-config-audits?${params.toString()}`)
}

export async function upsertRuntimeConfig(payload = {}) {
  return httpRequest('/admin/runtime-configs', {
    method: 'POST',
    body: payload
  })
}
