<template>
  <div class="auction-container">
    <n-card title="拍卖行">
      <template #header-extra>
        <n-space>
          <n-button :loading="auctionStore.loading" @click="refresh">刷新</n-button>
        </n-space>
      </template>
      <n-space vertical>
        <n-grid :cols="24" :x-gap="12" :y-gap="12">
          <n-grid-item :span="24">
            <n-card size="small" title="上架物品">
              <n-space align="center" wrap>
                <n-select
                  v-model:value="selectedItemId"
                  :options="tradableItemOptions"
                  placeholder="选择要上架的物品"
                  filterable
                  style="min-width: 260px"
                  :disabled="auctionStore.submitting"
                />
                <n-input-number
                  v-model:value="price"
                  :min="1"
                  :step="10"
                  placeholder="价格(灵石)"
                  style="width: 140px"
                  :disabled="auctionStore.submitting"
                />
                <n-select
                  v-model:value="durationHours"
                  :options="durationOptions"
                  style="width: 120px"
                  :disabled="auctionStore.submitting"
                />
                <n-button type="primary" :loading="auctionStore.submitting" @click="createOrder">上架</n-button>
              </n-space>
              <n-text depth="3" style="display: block; margin-top: 8px">仅支持上架装备与丹药，成交后收取 5% 手续费。</n-text>
            </n-card>
          </n-grid-item>

          <n-grid-item :span="24">
            <n-card size="small" title="在售列表">
              <n-spin :show="auctionStore.loading || auctionStore.submitting">
                <n-table striped size="small">
                  <thead>
                    <tr>
                      <th style="width: 70px">订单</th>
                      <th>物品</th>
                      <th style="width: 110px">卖家</th>
                      <th style="width: 90px">价格</th>
                      <th style="width: 100px">当前最高出价</th>
                      <th style="width: 80px">状态</th>
                      <th style="width: 140px">过期时间</th>
                      <th style="width: 160px">操作</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-for="order in auctionStore.openOrders" :key="order.id">
                      <td>#{{ order.id }}</td>
                      <td>{{ formatOrderItem(order) }}</td>
                      <td>{{ order.sellerName || '-' }}</td>
                      <td>{{ order.price }}</td>
                      <td>{{ order.highestBid || '-' }}</td>
                      <td>{{ formatStatus(order) }}</td>
                      <td>{{ formatTime(order.expiresAt) }}</td>
                      <td>
                        <n-space>
                          <n-button
                            size="small"
                            type="primary"
                            :disabled="order.isMine || order.status !== 'open'"
                            :loading="auctionStore.submitting"
                            @click="buyOrder(order.id)"
                          >
                            购买
                          </n-button>
                          <n-button
                            size="small"
                            secondary
                            :disabled="order.isMine || order.status !== 'open'"
                            :loading="auctionStore.submitting"
                            @click="placeBid(order)"
                          >
                            出价
                          </n-button>
                        </n-space>
                      </td>
                    </tr>
                    <tr v-if="auctionStore.openOrders.length === 0">
                      <td colspan="8">
                        <n-empty description="暂无在售订单" />
                      </td>
                    </tr>
                  </tbody>
                </n-table>
              </n-spin>
            </n-card>
          </n-grid-item>

          <n-grid-item :span="24">
            <n-card size="small" title="我的订单">
              <n-table striped size="small">
                <thead>
                  <tr>
                    <th style="width: 70px">订单</th>
                    <th>物品</th>
                    <th style="width: 80px">价格</th>
                    <th style="width: 100px">最高出价</th>
                    <th style="width: 80px">状态</th>
                    <th style="width: 80px">买家</th>
                    <th style="width: 160px">时间</th>
                    <th style="width: 180px">操作</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="order in auctionStore.myOrders" :key="`my-${order.id}`">
                    <td>#{{ order.id }}</td>
                    <td>{{ formatOrderItem(order) }}</td>
                    <td>{{ order.price }}</td>
                    <td>{{ order.highestBid || '-' }}</td>
                    <td>{{ formatStatus(order) }}</td>
                    <td>{{ order.buyerUserId ? '已成交' : '-' }}</td>
                    <td>{{ formatTime(order.createdAt) }}</td>
                    <td>
                      <n-space>
                        <n-button
                          size="small"
                          type="primary"
                          :disabled="!order.isMine || order.status !== 'open' || Number(order.highestBid || 0) <= 0"
                          :loading="auctionStore.submitting"
                          @click="acceptBid(order.id)"
                        >
                          接受出价
                        </n-button>
                        <n-button
                          size="small"
                          secondary
                          :disabled="!order.isMine || order.status !== 'open'"
                          :loading="auctionStore.submitting"
                          @click="cancelOrder(order.id)"
                        >
                          取消
                        </n-button>
                      </n-space>
                    </td>
                  </tr>
                  <tr v-if="auctionStore.myOrders.length === 0">
                    <td colspan="8">
                      <n-empty description="暂无我的订单" />
                    </td>
                  </tr>
                </tbody>
              </n-table>
            </n-card>
          </n-grid-item>
        </n-grid>
      </n-space>
    </n-card>
  </div>
</template>

<script setup>
  import { computed, onMounted, ref } from 'vue'
  import { useMessage } from 'naive-ui'
  import { useAuctionStore } from '../stores/auction'
  import { usePlayerStore } from '../stores/player'

  const message = useMessage()
  const auctionStore = useAuctionStore()
  const playerStore = usePlayerStore()

  const selectedItemId = ref(null)
  const price = ref(100)
  const durationHours = ref(24)

  const durationOptions = [
    { label: '6小时', value: 6 },
    { label: '12小时', value: 12 },
    { label: '24小时', value: 24 }
  ]

  const tradableItemOptions = computed(() => {
    const items = Array.isArray(playerStore.items) ? playerStore.items : []
    return items
      .filter(item => isTradableType(readString(item?.type)))
      .map(item => ({
        label: `${readString(item?.name) || '未知物品'} (${readString(item?.type) || '-'})`,
        value: readString(item?.id)
      }))
  })

  const refresh = async () => {
    try {
      await auctionStore.refresh()
    } catch (error) {
      message.error(error?.message || '刷新拍卖列表失败')
    }
  }

  const createOrder = async () => {
    if (!selectedItemId.value) {
      message.warning('请选择要上架的物品')
      return
    }
    if (!price.value || price.value <= 0) {
      message.warning('请输入有效价格')
      return
    }

    try {
      const result = await auctionStore.createOrder({
        itemId: selectedItemId.value,
        price: Number(price.value),
        durationHours: Number(durationHours.value || 24)
      })
      message.success(result?.message || '上架成功')
      selectedItemId.value = null
    } catch (error) {
      message.error(error?.message || '上架失败')
    }
  }

  const buyOrder = async orderId => {
    try {
      const result = await auctionStore.buyOrder(orderId)
      message.success(result?.message || '购买成功')
    } catch (error) {
      message.error(error?.message || '购买失败')
    }
  }

  const placeBid = async order => {
    const minimum = Math.max(Number(order?.price || 0), Number(order?.highestBid || 0) + 1)
    const raw = window.prompt(`请输入出价（最低 ${minimum}）`, String(minimum))
    if (raw === null) return
    const amount = Number(raw)
    if (!Number.isFinite(amount) || amount <= 0) {
      message.warning('请输入有效的出价金额')
      return
    }

    try {
      const result = await auctionStore.bidOrder(order.id, Math.floor(amount))
      message.success(result?.message || '出价成功')
    } catch (error) {
      message.error(error?.message || '出价失败')
    }
  }

  const cancelOrder = async orderId => {
    try {
      const result = await auctionStore.cancelOrder(orderId)
      message.success(result?.message || '取消上架成功')
    } catch (error) {
      message.error(error?.message || '取消上架失败')
    }
  }

  const acceptBid = async orderId => {
    try {
      const result = await auctionStore.acceptBidOrder(orderId)
      message.success(result?.message || '接受出价成功')
    } catch (error) {
      message.error(error?.message || '接受出价失败')
    }
  }

  const formatOrderItem = order => {
    const item = order?.item || {}
    const name = readString(item?.name) || '未知物品'
    const quality = readString(item?.quality)
    return quality ? `${name} [${quality}]` : name
  }

  const formatStatus = order => {
    const status = readString(order?.status)
    if (status === 'open' && isExpired(order?.expiresAt)) {
      return '已过期(可取消)'
    }
    switch (status) {
      case 'open':
        return '在售'
      case 'sold':
        return '已成交'
      case 'cancelled':
        return '已取消'
      case 'expired':
        return '已过期'
      default:
        return status || '-'
    }
  }

  const formatTime = value => {
    if (!value) return '-'
    const date = new Date(value)
    if (Number.isNaN(date.getTime())) return '-'
    return date.toLocaleString()
  }

  const isExpired = value => {
    if (!value) return false
    const date = new Date(value)
    if (Number.isNaN(date.getTime())) return false
    return date.getTime() <= Date.now()
  }

  const readString = value => {
    if (typeof value === 'string') return value
    if (typeof value === 'number') return String(value)
    return ''
  }

  const isTradableType = itemType => {
    return [
      'pill',
      'weapon',
      'head',
      'body',
      'legs',
      'feet',
      'shoulder',
      'hands',
      'wrist',
      'necklace',
      'ring1',
      'ring2',
      'belt',
      'artifact'
    ].includes(itemType)
  }

  onMounted(() => {
    refresh()
  })
</script>

<style scoped>
  .auction-container {
    margin: 0 auto;
  }
</style>
