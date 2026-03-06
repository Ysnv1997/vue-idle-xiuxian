import { defineStore } from 'pinia'
import {
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
    lastError: '',
    filters: {
      category: '',
      subCategory: ''
    }
  }),
  actions: {
    async loadOpenOrders(limit = 20, filters = null) {
      this.loading = true
      this.lastError = ''
      try {
        if (filters && typeof filters === 'object') {
          this.filters = {
            category: filters.category || '',
            subCategory: filters.subCategory || ''
          }
        }
        const result = await fetchAuctionList({
          limit,
          offset: 0,
          category: this.filters.category,
          subCategory: this.filters.subCategory
        })
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
    async refresh(limit = 20, filters = null) {
      await Promise.all([this.loadOpenOrders(limit, filters), this.loadMyOrders(limit)])
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
    applySnapshot(snapshot) {
      if (!snapshot) return
      const playerStore = usePlayerStore()
      playerStore.applyServerSnapshot(snapshot)
    }
  }
})
