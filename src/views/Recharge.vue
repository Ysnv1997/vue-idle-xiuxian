<template>
  <div class="page-view recharge-page">
    <!-- 顶部标题与当前余额 -->
    <header class="page-head">
      <div class="head-main">
        <p class="page-eyebrow">灵脉枯竭 · 功德补给</p>
        <h2 class="page-title">灵石宝库</h2>
      </div>
      <div class="head-balance">
        <div class="balance-card">
          <div class="label">当前库存灵石</div>
          <div class="value">
            <n-icon color="#f0a020"><WalletOutline /></n-icon>
            {{ formatNumber(playerStore.spiritStones) }}
          </div>
        </div>
        <n-button quaternary circle @click="refresh" :loading="rechargeStore.loading">
          <template #icon><n-icon><RefreshOutline /></n-icon></template>
        </n-button>
      </div>
    </header>

    <div class="recharge-content">
      <!-- 充值套餐网格 -->
      <section class="packages-section">
        <div class="section-head">
          <span class="section-title">功德兑换套餐</span>
          <n-tag :bordered="false" type="info" size="small" round>汇率：1 LDC ≈ 100 灵石</n-tag>
        </div>

        <n-spin :show="rechargeStore.loading">
          <div class="packages-grid">
            <div 
              v-for="product in rechargeStore.products" 
              :key="product.code"
              class="package-card"
              :class="getPackageClass(product)"
            >
              <div class="package-accent"></div>
              <div class="bonus-ribbon" v-if="product.bonusRate > 0">
                +{{ (product.bonusRate * 100).toFixed(0) }}% 额外赠送
              </div>
              
              <div class="package-icon">
                <n-icon size="48"><StorefrontOutline /></n-icon>
              </div>
              
              <div class="package-info">
                <div class="stones-amount">
                  <span class="val">{{ formatNumber(product.spiritStones) }}</span>
                  <span class="unit">灵石</span>
                </div>
                <div class="package-code">套餐：{{ product.code }}</div>
              </div>

              <div class="package-footer">
                <div class="cost-area">
                  <span class="cost-val">{{ formatNumber(product.creditAmount) }}</span>
                  <span class="cost-unit">LDC</span>
                </div>
                <n-button
                  type="primary"
                  round
                  block
                  :loading="rechargeStore.submitting && creatingProductCode === product.code"
                  @click="createOrder(product.code)"
                >
                  立即结缘
                </n-button>
              </div>
            </div>
          </div>
          <n-empty v-if="rechargeStore.products.length === 0" description="宝库暂时封闭，请稍后再试" />
        </n-spin>
      </section>

      <!-- 订单记录 -->
      <section class="orders-section">
        <div class="section-head" @click="showOrders = !showMyOrders">
          <span class="section-title">兑换法旨记录</span>
          <n-icon><ChevronUpOutline v-if="!showOrders" /><ChevronDownOutline v-else /></n-icon>
        </div>

        <n-collapse-transition :show="showOrders">
          <div class="orders-table-wrap">
            <n-table striped size="small">
              <thead>
                <tr>
                  <th>单号</th>
                  <th>套餐</th>
                  <th>灵石</th>
                  <th>状态</th>
                  <th>时间</th>
                  <th>操作</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="order in rechargeStore.orders" :key="order.id">
                  <td class="font-mono">#{{ order.id }}</td>
                  <td>{{ order.productCode }}</td>
                  <td class="text-primary">{{ formatNumber(order.spiritStones) }}</td>
                  <td>
                    <n-tag :type="getStatusTagType(order.status)" size="small" round>
                      {{ formatStatus(order.status) }}
                    </n-tag>
                  </td>
                  <td class="text-sub">{{ formatTime(order.createdAt) }}</td>
                  <td>
                    <n-button
                      v-if="canSyncOrder(order)"
                      size="tiny"
                      secondary
                      :loading="rechargeStore.submitting && syncingOrderId === order.id"
                      @click="syncOrder(order.id)"
                    >
                      同步结果
                    </n-button>
                    <span v-else>-</span>
                  </td>
                </tr>
              </tbody>
            </n-table>
            <n-empty v-if="rechargeStore.orders.length === 0" description="暂无历史法旨" style="padding: 20px" />
          </div>
        </n-collapse-transition>
      </section>
    </div>

    <!-- 底部说明 -->
    <footer class="recharge-footer">
      <div class="hint-box">
        <div class="hint-title">充值指引：</div>
        <ul>
          <li>点击“立即结缘”将跳转至外部支付页面。</li>
          <li>支付成功后，灵石将自动发放到您的乾坤袋。</li>
          <li>若长时间未到账，请点击“同步结果”按钮。</li>
          <li>如遇系统异常，请通过 LinuxDO 社区寻求天道指引（联系客服）。</li>
        </ul>
      </div>
    </footer>
  </div>
</template>

<script setup>
  import { onMounted, ref } from 'vue'
  import { useMessage } from 'naive-ui'
  import { 
    WalletOutline, 
    RefreshOutline, 
    ChevronUpOutline, 
    ChevronDownOutline,
    StorefrontOutline,
    ShieldCheckmarkOutline
  } from '@vicons/ionicons5'
  import { useRechargeStore } from '../stores/recharge'
  import { usePlayerStore } from '../stores/player'

  const message = useMessage()
  const rechargeStore = useRechargeStore()
  const playerStore = usePlayerStore()

  const creatingProductCode = ref('')
  const syncingOrderId = ref(0)
  const showOrders = ref(true)

  const refresh = async () => {
    try {
      await rechargeStore.refresh()
    } catch (error) { message.error('加载宝库数据失败') }
  }

  const createOrder = async productCode => {
    creatingProductCode.value = productCode
    try {
      const result = await rechargeStore.createOrder(productCode)
      message.success('法旨已颁布，请前往外部结缘')
      const checkoutUrl = result?.checkoutUrl || ''
      if (checkoutUrl) {
        window.open(checkoutUrl, '_blank', 'noopener,noreferrer')
      }
    } catch (error) { message.error('颁布法旨失败') }
    finally { creatingProductCode.value = '' }
  }

  const syncOrder = async orderId => {
    syncingOrderId.value = orderId
    try {
      const result = await rechargeStore.syncOrder(orderId)
      message.success(result?.message || '状态同步完成')
    } catch (error) { message.error('同步失败') }
    finally { syncingOrderId.value = 0 }
  }

  const canSyncOrder = order => String(order?.status || '').toLowerCase() !== 'paid'

  const getStatusTagType = status => {
    const s = String(status).toLowerCase()
    if (s === 'paid') return 'success'
    if (s === 'pending') return 'warning'
    return 'default'
  }

  const formatStatus = status => {
    const map = { paid: '已结缘', pending: '待感悟', failed: '失败', cancelled: '取消' }
    return map[String(status).toLowerCase()] || status
  }

  const formatTime = v => v ? new Date(v).toLocaleString() : '-'
  const formatNumber = v => Number(v || 0).toLocaleString()

  const getPackageClass = p => {
    if (p.creditAmount >= 1000) return 'is-epic'
    if (p.creditAmount >= 500) return 'is-rare'
    return ''
  }

  onMounted(() => { refresh() })
</script>

<style scoped>
.recharge-page {
  display: flex;
  flex-direction: column;
  height: 100%;
  max-width: 1000px;
  margin: 0 auto;
}

.page-head {
  display: flex;
  justify-content: space-between;
  align-items: flex-end;
  margin-bottom: 32px;
}

.balance-card {
  background: var(--panel-bg);
  border: 1px solid var(--panel-border);
  padding: 12px 24px;
  border-radius: 20px;
  display: flex;
  flex-direction: column;
  gap: 4px;
  box-shadow: 0 8px 24px rgba(0,0,0,0.05);
}
.balance-card .label { font-size: 11px; color: var(--ink-sub); text-transform: uppercase; letter-spacing: 1px; }
.balance-card .value { font-size: 20px; font-weight: 900; color: #f0a020; display: flex; align-items: center; gap: 8px; }

.recharge-content { display: flex; flex-direction: column; gap: 40px; }

.section-head { display: flex; justify-content: space-between; align-items: center; margin-bottom: 20px; }
.section-title { font-size: 16px; font-weight: bold; font-family: var(--font-display); }

/* 套餐网格 */
.packages-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 20px;
}

.package-card {
  position: relative;
  background: var(--panel-bg);
  border: 1px solid var(--panel-border);
  border-radius: 24px;
  padding: 32px 24px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 24px;
  overflow: hidden;
  transition: all 0.3s ease;
}

.package-card:hover { transform: translateY(-8px); border-color: #f0a020; box-shadow: 0 12px 32px rgba(240, 160, 32, 0.1); }

.package-accent {
  position: absolute;
  top: 0; left: 0; right: 0; height: 4px;
  background: var(--panel-border);
}
.is-rare .package-accent { background: linear-gradient(90deg, #2080f0, #a042ff); }
.is-epic .package-accent { background: linear-gradient(90deg, #f0a020, #d03050); }

.bonus-ribbon {
  position: absolute;
  top: 12px; right: -30px;
  background: #d03050;
  color: white;
  font-size: 10px;
  padding: 4px 40px;
  transform: rotate(45deg);
  font-weight: bold;
}

.package-icon {
  width: 80px; height: 80px;
  background: var(--accent-muted);
  border-radius: 20px;
  display: grid; place-items: center;
  color: var(--accent-primary);
}

.package-info { text-align: center; }
.stones-amount { display: flex; align-items: baseline; gap: 6px; }
.stones-amount .val { font-size: 32px; font-weight: 900; color: var(--ink-main); }
.stones-amount .unit { font-size: 14px; color: var(--ink-sub); font-family: var(--font-display); }
.package-code { font-size: 11px; opacity: 0.4; margin-top: 4px; }

.package-footer { width: 100%; display: flex; flex-direction: column; gap: 16px; }
.cost-area { text-align: center; font-family: var(--font-display); }
.cost-val { font-size: 24px; font-weight: bold; color: var(--accent-primary); }
.cost-unit { font-size: 12px; margin-left: 4px; opacity: 0.6; }

/* 订单部分 */
.orders-section {
  background: var(--panel-bg);
  border: 1px solid var(--panel-border);
  border-radius: 24px;
  overflow: hidden;
}
.orders-section .section-head { padding: 16px 24px; cursor: pointer; background: rgba(0,0,0,0.02); margin-bottom: 0; }
.orders-table-wrap { padding: 20px; }

.font-mono { font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace; }
.text-primary { color: var(--accent-primary); font-weight: bold; }
.text-sub { font-size: 12px; opacity: 0.6; }

/* 底部说明 */
.recharge-footer { margin-top: 40px; margin-bottom: 100px; }
.hint-box {
  background: rgba(0,0,0,0.02);
  border: 1px dashed var(--panel-border);
  border-radius: 16px;
  padding: 24px;
}
.hint-title { font-weight: bold; margin-bottom: 12px; color: var(--ink-sub); }
.hint-box ul { padding-left: 20px; display: flex; flex-direction: column; gap: 8px; }
.hint-box li { font-size: 13px; color: var(--ink-sub); }

@media (max-width: 768px) {
  .page-head { flex-direction: column; align-items: flex-start; gap: 20px; }
  .head-balance { width: 100%; justify-content: space-between; }
  .packages-grid { grid-template-columns: 1fr; }
  .orders-section { margin-bottom: 120px; }
}
</style>
