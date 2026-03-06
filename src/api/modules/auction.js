import { httpRequest } from '../http'

export async function fetchAuctionList({ limit = 20, offset = 0, category = '', subCategory = '' } = {}) {
  const params = new URLSearchParams()
  params.set('limit', String(limit))
  params.set('offset', String(offset))
  if (category) {
    params.set('category', category)
  }
  if (subCategory) {
    params.set('subCategory', subCategory)
  }
  return httpRequest(`/auction/list?${params.toString()}`)
}

export async function fetchMyAuctionOrders(limit = 20) {
  const params = new URLSearchParams()
  params.set('limit', String(limit))
  return httpRequest(`/auction/my-orders?${params.toString()}`)
}

export async function createAuctionOrder(payload) {
  return httpRequest('/auction/create', {
    method: 'POST',
    body: payload
  })
}

export async function cancelAuctionOrder(orderId) {
  return httpRequest('/auction/cancel', {
    method: 'POST',
    body: { orderId }
  })
}

export async function buyAuctionOrder(orderId, idempotencyKey = '') {
  return httpRequest('/auction/buy', {
    method: 'POST',
    body: { orderId },
    idempotencyKey
  })
}
