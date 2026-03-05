import { httpRequest } from '../http'

export async function fetchRechargeProducts() {
  return httpRequest('/recharge/products')
}

export async function fetchRechargeOrders(limit = 20) {
  const params = new URLSearchParams()
  params.set('limit', String(limit))
  return httpRequest(`/recharge/orders?${params.toString()}`)
}

export async function createRechargeOrder(productCode, idempotencyKey = '') {
  return httpRequest('/recharge/orders', {
    method: 'POST',
    body: {
      productCode,
      idempotencyKey
    },
    idempotencyKey
  })
}

export async function mockRechargeOrderPaid(orderId) {
  return httpRequest('/recharge/orders/mock-paid', {
    method: 'POST',
    body: {
      orderId
    }
  })
}

export async function syncRechargeOrder(orderId) {
  return httpRequest('/recharge/orders/sync', {
    method: 'POST',
    body: {
      orderId
    }
  })
}
