<template>
  <section class="page-view recharge-view">
    <header class="page-head">
      <p class="page-eyebrow">灵石补给</p>
      <h2>充值中心</h2>
      <p class="page-desc">查看套餐、创建订单并跟踪支付状态。</p>
    </header>

    <n-card :bordered="false" class="page-card">
      <template #header-extra>
        <n-space>
          <n-button :loading="rechargeStore.loading" @click="refresh">刷新</n-button>
        </n-space>
      </template>

      <n-space vertical>
        <n-alert type="info" :show-icon="false">
          当前灵石：<strong>{{ formatNumber(playerStore.spiritStones) }}</strong>
        </n-alert>

        <n-card size="small" title="充值套餐">
          <n-spin :show="rechargeStore.loading">
            <n-table striped size="small">
              <thead>
                <tr>
                  <th>套餐代码</th>
                  <th>积分(LDC)</th>
                  <th>灵石到账</th>
                  <th>加成率</th>
                  <th style="width: 140px">操作</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="product in rechargeStore.products" :key="product.code">
                  <td>{{ product.code }}</td>
                  <td>{{ formatNumber(product.creditAmount) }}</td>
                  <td>{{ formatNumber(product.spiritStones) }}</td>
                  <td>{{ formatBonusRate(product.bonusRate) }}</td>
                  <td>
                    <n-button
                      type="primary"
                      size="small"
                      :loading="rechargeStore.submitting && creatingProductCode === product.code"
                      @click="createOrder(product.code)"
                    >
                      创建订单
                    </n-button>
                  </td>
                </tr>
                <tr v-if="rechargeStore.products.length === 0">
                  <td colspan="5">
                    <n-empty description="暂无可用充值套餐" />
                  </td>
                </tr>
              </tbody>
            </n-table>
          </n-spin>
        </n-card>

        <n-card size="small" title="我的充值订单">
          <n-spin :show="rechargeStore.loading">
            <n-table striped size="small">
              <thead>
                <tr>
                  <th style="width: 80px">订单ID</th>
                  <th>套餐</th>
                  <th>状态</th>
                  <th>灵石</th>
                  <th>创建时间</th>
                  <th>支付时间</th>
                  <th>外部单号</th>
                  <th style="width: 120px">操作</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="order in rechargeStore.orders" :key="order.id">
                  <td>#{{ order.id }}</td>
                  <td>{{ order.productCode }}</td>
                  <td>{{ formatStatus(order.status) }}</td>
                  <td>{{ formatNumber(order.spiritStones) }}</td>
                  <td>{{ formatTime(order.createdAt) }}</td>
                  <td>{{ formatTime(order.paidAt) }}</td>
                  <td>{{ order.externalOrderId || '-' }}</td>
                  <td>
                    <n-button
                      v-if="canSyncOrder(order)"
                      size="small"
                      tertiary
                      :loading="rechargeStore.submitting && syncingOrderId === order.id"
                      @click="syncOrder(order.id)"
                    >
                      同步支付结果
                    </n-button>
                    <span v-else>-</span>
                  </td>
                </tr>
                <tr v-if="rechargeStore.orders.length === 0">
                  <td colspan="8">
                    <n-empty description="暂无充值订单" />
                  </td>
                </tr>
              </tbody>
            </n-table>
          </n-spin>
        </n-card>
      </n-space>
    </n-card>
  </section>
</template>

<script setup>
  import { onMounted, ref } from 'vue'
  import { useMessage } from 'naive-ui'
  import { useRechargeStore } from '../stores/recharge'
  import { usePlayerStore } from '../stores/player'

  const message = useMessage()
  const rechargeStore = useRechargeStore()
  const playerStore = usePlayerStore()

  const creatingProductCode = ref('')
  const syncingOrderId = ref(0)

  const refresh = async () => {
    try {
      await rechargeStore.refresh()
    } catch (error) {
      message.error(error?.message || '加载充值数据失败')
    }
  }

  const createOrder = async productCode => {
    creatingProductCode.value = productCode
    try {
      const result = await rechargeStore.createOrder(productCode)
      message.success('充值订单已创建')

      const checkoutUrl = result?.checkoutUrl || ''
      if (checkoutUrl) {
        window.open(checkoutUrl, '_blank', 'noopener,noreferrer')
      }
    } catch (error) {
      message.error(error?.message || '创建充值订单失败')
    } finally {
      creatingProductCode.value = ''
    }
  }

  const syncOrder = async orderId => {
    syncingOrderId.value = orderId
    try {
      const result = await rechargeStore.syncOrder(orderId)
      message.success(result?.message || '订单同步成功')
    } catch (error) {
      message.error(error?.message || '同步订单失败')
    } finally {
      syncingOrderId.value = 0
    }
  }

  const canSyncOrder = order => {
    return String(order?.status || '').toLowerCase() !== 'paid'
  }

  const formatStatus = status => {
    const normalized = String(status || '').toLowerCase()
    switch (normalized) {
      case 'paid':
        return '已支付'
      case 'pending':
        return '待支付'
      case 'failed':
        return '支付失败'
      case 'cancelled':
        return '已取消'
      default:
        return normalized || '-'
    }
  }

  const formatTime = value => {
    if (!value) return '-'
    const date = new Date(value)
    if (Number.isNaN(date.getTime())) return '-'
    return date.toLocaleString()
  }

  const formatNumber = value => Number(value || 0).toLocaleString()
  const formatBonusRate = value => `${(Number(value || 0) * 100).toFixed(0)}%`

  onMounted(() => {
    refresh()
  })
</script>
