import { httpRequest } from '../http'

export async function fetchAuctionList(limit = 20, offset = 0) {
  const params = new URLSearchParams()
  params.set('limit', String(limit))
  params.set('offset', String(offset))
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

export async function bidAuctionOrder(orderId, amount) {
  return httpRequest('/auction/bid', {
    method: 'POST',
    body: { orderId, amount }
  })
}

export async function acceptAuctionBidOrder(orderId) {
  return httpRequest('/auction/accept-bid', {
    method: 'POST',
    body: { orderId }
  })
}
