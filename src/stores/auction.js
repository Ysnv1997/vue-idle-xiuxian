import { defineStore } from 'pinia'
import {
  acceptAuctionBidOrder,
  bidAuctionOrder,
  buyAuctionOrder,
  cancelAuctionOrder,
  createAuctionOrder,
  fetchAuctionList,
  fetchMyAuctionOrders
} from '../api/modules/auction'
import { usePlayerStore } from './player'

export const useAuctionStore = defineStore('auction', {
  state: () => ({
    openOrders: [],
    myOrders: [],
    loading: false,
    submitting: false,
    lastError: ''
  }),
  actions: {
    async loadOpenOrders(limit = 20) {
      this.loading = true
      this.lastError = ''
      try {
        const result = await fetchAuctionList(limit, 0)
        this.openOrders = Array.isArray(result?.orders) ? result.orders : []
        return this.openOrders
      } catch (error) {
        this.lastError = error?.message || '加载拍卖列表失败'
        throw error
      } finally {
        this.loading = false
      }
    },
    async loadMyOrders(limit = 20) {
      this.loading = true
      this.lastError = ''
      try {
        const result = await fetchMyAuctionOrders(limit)
        this.myOrders = Array.isArray(result?.orders) ? result.orders : []
        return this.myOrders
      } catch (error) {
        this.lastError = error?.message || '加载我的订单失败'
        throw error
      } finally {
        this.loading = false
      }
    },
    async refresh(limit = 20) {
      await Promise.all([this.loadOpenOrders(limit), this.loadMyOrders(limit)])
    },
    async createOrder(payload) {
      this.submitting = true
      this.lastError = ''
      try {
        const result = await createAuctionOrder(payload)
        this.applySnapshot(result?.snapshot)
        await this.refresh()
        return result
      } catch (error) {
        this.lastError = error?.message || '上架失败'
        throw error
      } finally {
        this.submitting = false
      }
    },
    async cancelOrder(orderId) {
      this.submitting = true
      this.lastError = ''
      try {
        const result = await cancelAuctionOrder(orderId)
        this.applySnapshot(result?.snapshot)
        await this.refresh()
        return result
      } catch (error) {
        this.lastError = error?.message || '取消上架失败'
        throw error
      } finally {
        this.submitting = false
      }
    },
    async buyOrder(orderId) {
      this.submitting = true
      this.lastError = ''
      try {
        const idempotencyKey =
          typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function'
            ? crypto.randomUUID()
            : `buy-${orderId}-${Date.now()}`
        const result = await buyAuctionOrder(orderId, idempotencyKey)
        this.applySnapshot(result?.snapshot)
        await this.refresh()
        return result
      } catch (error) {
        this.lastError = error?.message || '购买失败'
        throw error
      } finally {
        this.submitting = false
      }
    },
    async bidOrder(orderId, amount) {
      this.submitting = true
      this.lastError = ''
      try {
        const result = await bidAuctionOrder(orderId, amount)
        await this.refresh()
        return result
      } catch (error) {
        this.lastError = error?.message || '出价失败'
        throw error
      } finally {
        this.submitting = false
      }
    },
    async acceptBidOrder(orderId) {
      this.submitting = true
      this.lastError = ''
      try {
        const result = await acceptAuctionBidOrder(orderId)
        this.applySnapshot(result?.snapshot)
        await this.refresh()
        return result
      } catch (error) {
        this.lastError = error?.message || '接受出价失败'
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
