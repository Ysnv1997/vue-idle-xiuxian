import { defineStore } from 'pinia'
import {
  createRechargeOrder,
  fetchRechargeOrders,
  fetchRechargeProducts,
  mockRechargeOrderPaid,
  syncRechargeOrder
} from '../api/modules/recharge'
import { usePlayerStore } from './player'

export const useRechargeStore = defineStore('recharge', {
  state: () => ({
    products: [],
    orders: [],
    loading: false,
    submitting: false,
    lastError: ''
  }),
  actions: {
    async loadProducts() {
      this.loading = true
      this.lastError = ''
      try {
        const result = await fetchRechargeProducts()
        this.products = Array.isArray(result?.products) ? result.products : []
        return this.products
      } catch (error) {
        this.lastError = error?.message || '加载充值套餐失败'
        throw error
      } finally {
        this.loading = false
      }
    },
    async loadOrders(limit = 20) {
      this.loading = true
      this.lastError = ''
      try {
        const result = await fetchRechargeOrders(limit)
        this.orders = Array.isArray(result?.orders) ? result.orders : []
        return this.orders
      } catch (error) {
        this.lastError = error?.message || '加载充值订单失败'
        throw error
      } finally {
        this.loading = false
      }
    },
    async refresh(limit = 20) {
      await Promise.all([this.loadProducts(), this.loadOrders(limit)])
    },
    async createOrder(productCode) {
      this.submitting = true
      this.lastError = ''
      try {
        const idempotencyKey =
          typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function'
            ? crypto.randomUUID()
            : `recharge-${Date.now()}`
        const result = await createRechargeOrder(productCode, idempotencyKey)
        this.applySnapshot(result?.snapshot)
        await this.loadOrders()
        return result
      } catch (error) {
        this.lastError = error?.message || '创建充值订单失败'
        throw error
      } finally {
        this.submitting = false
      }
    },
    async mockPaid(orderId) {
      this.submitting = true
      this.lastError = ''
      try {
        const result = await mockRechargeOrderPaid(orderId)
        this.applySnapshot(result?.snapshot)
        await this.loadOrders()
        return result
      } catch (error) {
        this.lastError = error?.message || '模拟支付失败'
        throw error
      } finally {
        this.submitting = false
      }
    },
    async syncOrder(orderId) {
      this.submitting = true
      this.lastError = ''
      try {
        const result = await syncRechargeOrder(orderId)
        this.applySnapshot(result?.snapshot)
        await this.loadOrders()
        return result
      } catch (error) {
        this.lastError = error?.message || '同步订单失败'
        throw error
      } finally {
        this.submitting = false
      }
    },
    applySnapshot(snapshot) {
      if (!snapshot) return
      const playerStore = usePlayerStore()
      playerStore.applyServerSnapshot(snapshot)
    }
  }
})
