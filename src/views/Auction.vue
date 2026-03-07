<template>
  <div class="page-view auction-page">
    <!-- 顶部标题与资源 -->
    <header class="page-head">
      <div class="head-main">
        <p class="page-eyebrow">珍宝互通 · 各取所需</p>
        <h2 class="page-title">坊市交易</h2>
      </div>
      <div class="head-resource">
        <div class="wealth-chip">
          <n-icon><WalletOutlined /></n-icon>
          <span>拥有灵石：{{ formatNumber(playerStore.spiritStones) }}</span>
        </div>
        <n-button quaternary circle @click="refresh" :loading="auctionStore.loading">
          <template #icon><n-icon><RefreshOutline /></n-icon></template>
        </n-button>
      </div>
    </header>

    <div class="auction-layout">
      <!-- 左侧：市场筛选器 -->
      <aside class="market-filter">
        <div class="filter-group">
          <div class="filter-label">大类筛选</div>
          <div class="filter-tags">
            <div 
              v-for="cat in categoryOptions" 
              :key="cat.value"
              class="filter-tag"
              :class="{ 'is-active': filterCategory === cat.value }"
              @click="filterCategory = cat.value; handleCategoryChange(cat.value)"
            >
              {{ cat.label }}
            </div>
          </div>
        </div>

        <div class="filter-group" v-if="filterCategory === 'equipment'">
          <div class="filter-label">部位细分</div>
          <n-select
            v-model:value="filterSubCategory"
            :options="equipmentSubCategoryOptions"
            size="small"
            placeholder="全品类"
            @update:value="refresh"
          />
        </div>

        <!-- 快速上架区域 -->
        <div class="quick-listing-box">
          <div class="box-title">我要摆摊</div>
          <div class="listing-form">
            <n-select
              v-model:value="selectedItemId"
              :options="tradableItemOptions"
              placeholder="选择手中宝物"
              filterable
              class="listing-select"
              :disabled="auctionStore.submitting"
              @update:value="handleSelectListingItem"
            />
            <div class="price-input-row">
              <n-input-number
                v-model:value="price"
                :min="1"
                placeholder="定价"
                class="listing-price"
                :disabled="auctionStore.submitting"
              >
                <template #suffix>灵石</template>
              </n-input-number>
              <n-button type="primary" :loading="auctionStore.submitting" @click="createOrder">上架</n-button>
            </div>
            <p class="fee-hint">※ 成交扣除 5% 印花税</p>
          </div>
        </div>
      </aside>

      <!-- 右侧：商品网格展示 -->
      <main class="market-display">
        <n-spin :show="auctionStore.loading">
          <div class="market-grid">
            <div 
              v-for="order in auctionStore.openOrders" 
              :key="order.id" 
              class="order-card"
              :class="[getItemQualityClass(order.item), { 'is-mine': order.isMine }]"
            >
              <div class="card-rarity-bg"></div>
              
              <!-- 商品头部 -->
              <div class="order-item-header">
                <div class="item-name">{{ formatOrderItem(order) }}</div>
                <div class="item-cat-tag">{{ formatOrderCategory(order) }}</div>
              </div>

              <!-- 商品详情预览 -->
              <div class="item-preview">
                <div class="item-icon-wrap">
                  <div class="item-mark">{{ order.item?.name?.[0] || '宝' }}</div>
                </div>
                <div class="item-seller">
                  <span class="label">卖家</span>
                  <span class="value">{{ order.sellerName || '神秘修士' }}</span>
                </div>
              </div>

              <!-- 价格与购买 -->
              <div class="order-footer">
                <div class="price-area">
                  <n-icon color="#f0a020"><WalletOutlined /></n-icon>
                  <span class="price-val">{{ formatNumber(order.price) }}</span>
                </div>
                
                <div class="action-area">
                  <n-button
                    v-if="!order.isMine"
                    type="primary"
                    size="small"
                    round
                    :disabled="order.status !== 'open' || playerStore.spiritStones < order.price"
                    :loading="auctionStore.submitting"
                    @click="buyOrder(order.id)"
                  >
                    购买
                  </n-button>
                  <n-button
                    v-else
                    secondary
                    type="error"
                    size="small"
                    round
                    :loading="auctionStore.submitting"
                    @click="cancelOrder(order.id)"
                  >
                    撤回
                  </n-button>
                </div>
              </div>

              <!-- 过期倒计时 -->
              <div class="expire-hint" v-if="order.status === 'open'">
                {{ getRemainingTimeLabel(order.expiresAt) }}
              </div>
              <div class="mine-badge" v-if="order.isMine">我的摊位</div>
            </div>
          </div>
          <n-empty v-if="auctionStore.openOrders.length === 0" description="坊市今日萧条，暂无此类货色" style="margin-top: 100px" />
        </n-spin>
      </main>
    </div>

    <!-- 底部：我的摊位管理 -->
    <footer class="my-stalls-section">
      <div class="section-head" @click="showMyOrders = !showMyOrders">
        <span class="section-title">我的交易记录 ({{ auctionStore.myOrders.length }})</span>
        <n-icon><ChevronUpOutline v-if="!showMyOrders" /><ChevronDownOutline v-else /></n-icon>
      </div>
      
      <n-collapse-transition :show="showMyOrders">
        <div class="stalls-content">
          <n-table striped size="small" class="stalls-table">
            <thead>
              <tr>
                <th>商品</th>
                <th>价格</th>
                <th>状态</th>
                <th>时间</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="order in auctionStore.myOrders" :key="`my-${order.id}`">
                <td>{{ formatOrderItem(order) }}</td>
                <td class="text-primary">{{ order.price }}</td>
                <td>
                  <n-tag :type="getStatusTagType(order)" size="small" round>
                    {{ formatStatus(order) }}
                  </n-tag>
                </td>
                <td>{{ formatTime(order.createdAt) }}</td>
                <td>
                  <n-button
                    v-if="order.status === 'open'"
                    size="tiny"
                    secondary
                    type="error"
                    @click="cancelOrder(order.id)"
                  >
                    撤回
                  </n-button>
                  <span v-else>-</span>
                </td>
              </tr>
            </tbody>
          </n-table>
        </div>
      </n-collapse-transition>
    </footer>
  </div>
</template>

<script setup>
  import { computed, onMounted, ref, watch } from 'vue'
  import { useMessage } from 'naive-ui'
  import { 
    WalletOutline, 
    RefreshOutline, 
    ChevronUpOutline, 
    ChevronDownOutline,
    StorefrontOutline,
    SearchOutline
  } from '@vicons/ionicons5'
  import { useAuctionStore } from '../stores/auction'
  import { usePlayerStore } from '../stores/player'
  import { pillRecipes } from '../plugins/pills'

  const message = useMessage()
  const auctionStore = useAuctionStore()
  const playerStore = usePlayerStore()

  const selectedItemId = ref(null)
  const price = ref(100)
  const filterCategory = ref('')
  const filterSubCategory = ref('')
  const showMyOrders = ref(false)

  const categoryOptions = [
    { label: '全部', value: '' },
    { label: '装备', value: 'equipment' },
    { label: '灵草', value: 'herb' },
    { label: '丹药', value: 'pill' },
    { label: '丹方', value: 'pill_fragment' },
    { label: '灵宠', value: 'pet' }
  ]

  const equipmentTypeNames = {
    weapon: '武器', head: '头部', body: '衣服', legs: '裤子', feet: '鞋子',
    shoulder: '肩甲', hands: '手套', wrist: '护腕', necklace: '项链',
    ring1: '戒指1', ring2: '戒指2', belt: '腰带', artifact: '法宝'
  }

  const equipmentSubCategoryOptions = computed(() => [
    { label: '全部部位', value: '' },
    ...Object.entries(equipmentTypeNames).map(([key, name]) => ({ label: name, value: key }))
  ])

  const tradableItemOptions = computed(() => {
    const options = []
    const items = Array.isArray(playerStore.items) ? playerStore.items : []
    for (const item of items) {
      if (!isTradableType(item?.type)) continue
      options.push({
        label: `${item.name} [${getItemQualityName(item)}]`,
        value: String(item.id),
        recommendPrice: getRecommendPriceByInventoryItem(item)
      })
    }

    // 灵草、丹方等逻辑同原代码，这里集成
    playerStore.herbs.forEach(herb => {
      options.push({
        label: `[草] ${herb.name} (${formatHerbQuality(herb.quality)})`,
        value: `herb:${herb.id}:${herb.quality}`,
        recommendPrice: getHerbRecommendPrice(herb.quality)
      })
    })

    const fragments = playerStore.pillFragments || {}
    for (const [recipeId, count] of Object.entries(fragments)) {
      if (Number(count) <= 0) continue
      options.push({
        label: `[方] ${resolveRecipeName(recipeId)} 残页`,
        value: `fragment:${recipeId}`,
        recommendPrice: getFragmentRecommendPrice(recipeId)
      })
    }

    return options
  })

  const refresh = async () => {
    try {
      await auctionStore.refresh(100, {
        category: filterCategory.value || '',
        subCategory: filterSubCategory.value || ''
      })
    } catch (error) { message.error('刷新坊市失败') }
  }

  const createOrder = async () => {
    if (!selectedItemId.value) return message.warning('请选择商品')
    try {
      await auctionStore.createOrder({
        itemId: selectedItemId.value,
        price: Math.floor(Number(price.value)),
        durationHours: 24
      })
      message.success('上架成功')
      selectedItemId.value = null
    } catch (error) { message.error('上架失败') }
  }

  const handleSelectListingItem = value => {
    if (!value) return (price.value = 100)
    const opt = tradableItemOptions.value.find(i => i.value === value)
    if (opt) price.value = Math.floor(opt.recommendPrice || 100)
  }

  const buyOrder = async id => {
    try {
      await auctionStore.buyOrder(id)
      message.success('购买成功，已收入乾坤袋')
    } catch (e) { message.error('购买失败') }
  }

  const cancelOrder = async id => {
    try {
      await auctionStore.cancelOrder(id)
      message.success('已撤回商品')
    } catch (e) { message.error('撤回失败') }
  }

  const handleCategoryChange = v => {
    if (v !== 'equipment') filterSubCategory.value = ''
    refresh()
  }

  // ---------------- 格式化辅助 ----------------
  const formatNumber = val => Number(val || 0).toLocaleString()
  
  const formatOrderItem = order => {
    const item = order?.item || {}
    if (item.type === 'pill_fragment') return `${item.name || '丹方残页'} x${item.count || 1}`
    return item.name || '珍宝'
  }

  const formatOrderCategory = order => {
    const cat = order.category || order.item?.type
    const map = { equipment: '装备', herb: '灵草', pill: '丹药', pill_fragment: '丹方', pet: '灵宠' }
    return map[cat] || '奇物'
  }

  const getItemQualityClass = item => `q-${item?.quality || item?.rarity || 'common'}`
  const getItemQualityName = item => {
    const q = item?.quality || item?.rarity
    const map = { common: '凡', uncommon: '下', rare: '中', epic: '上', legendary: '极', mythic: '仙' }
    return map[q] || '凡'
  }

  const getStatusTagType = order => {
    if (order.status === 'sold') return 'success'
    if (order.status === 'cancelled') return 'default'
    if (isExpired(order.expiresAt)) return 'warning'
    return 'primary'
  }

  const formatStatus = order => {
    if (order.status === 'open' && isExpired(order.expiresAt)) return '已过期'
    const map = { open: '在售', sold: '已成交', cancelled: '已取消', expired: '已过期' }
    return map[order.status] || order.status
  }

  const getRemainingTimeLabel = expiresAt => {
    const remain = new Date(expiresAt).getTime() - Date.now()
    if (remain <= 0) return '已过期'
    const hours = Math.floor(remain / 3600000)
    const mins = Math.floor((remain % 3600000) / 60000)
    return `剩 ${hours}h ${mins}m`
  }

  const formatTime = v => v ? new Date(v).toLocaleString() : '-'
  const isExpired = v => v ? new Date(v).getTime() <= Date.now() : false

  const isTradableType = type => ['pill', 'pet', ...Object.keys(equipmentTypeNames)].includes(type)

  const resolveRecipeName = id => pillRecipes.find(r => r.id === id)?.name || id
  
  const formatHerbQuality = q => {
    const map = { common: '普通', uncommon: '优质', rare: '稀有', epic: '极品', legendary: '仙品' }
    return map[q] || q
  }

  // 推荐价格逻辑 (集成原代码逻辑)
  const getRecommendPriceByInventoryItem = item => {
    if (item.type === 'pet') return getPetRecommendPrice(item.rarity)
    if (item.type === 'pill') return getPillRecommendPrice(item)
    return getEquipmentRecommendPrice(item.quality)
  }
  const getEquipmentRecommendPrice = q => ({ common: 80, uncommon: 150, rare: 300, epic: 700, legendary: 1500, mythic: 3200 }[q] || 120)
  const getPetRecommendPrice = r => ({ mortal: 200, spiritual: 450, mystic: 1000, celestial: 2200, divine: 5000 }[r] || 300)
  const getPillRecommendPrice = i => ({ grade1: 120, grade2: 180, grade3: 260, grade4: 420, grade5: 640, grade6: 950, grade7: 1400, grade8: 2100, grade9: 3200 }[i.grade] || 120)
  const getHerbRecommendPrice = q => ({ common: 60, uncommon: 120, rare: 260, epic: 600, legendary: 1500 }[q] || 80)
  const getFragmentRecommendPrice = id => {
    const g = pillRecipes.find(r => r.id === id)?.grade
    return ({ grade1: 100, grade2: 150, grade3: 220, grade4: 320, grade5: 460, grade6: 650, grade7: 900, grade8: 1250, grade9: 1750 }[g] || 180)
  }

  onMounted(() => { refresh() })
</script>

<style scoped>
.auction-page {
  display: flex;
  flex-direction: column;
  height: 100%;
  max-width: 1200px;
  margin: 0 auto;
}

.page-head {
  display: flex;
  justify-content: space-between;
  align-items: flex-end;
  margin-bottom: 24px;
}

.wealth-chip {
  background: var(--panel-bg);
  border: 1px solid var(--panel-border);
  padding: 8px 16px;
  border-radius: 99px;
  display: flex;
  align-items: center;
  gap: 8px;
  font-weight: bold;
  color: #f0a020;
}

.auction-layout {
  display: grid;
  grid-template-columns: 300px 1fr;
  gap: 24px;
  flex: 1;
}

/* 侧边筛选与上架 */
.market-filter {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.filter-group {
  background: var(--panel-bg);
  border: 1px solid var(--panel-border);
  border-radius: 20px;
  padding: 20px;
}

.filter-label { font-size: 13px; font-weight: bold; margin-bottom: 12px; color: var(--accent-primary); }

.filter-tags { display: flex; flex-wrap: wrap; gap: 8px; }
.filter-tag {
  padding: 6px 12px;
  background: rgba(0,0,0,0.03);
  border: 1px solid var(--panel-border);
  border-radius: 8px;
  font-size: 12px;
  cursor: pointer;
  transition: all 0.2s;
}
.filter-tag:hover { border-color: var(--accent-primary); }
.filter-tag.is-active { background: var(--accent-primary); color: white; border-color: var(--accent-primary); }

.quick-listing-box {
  background: var(--panel-bg);
  border: 1px solid var(--panel-border);
  border-radius: 20px;
  padding: 20px;
}
.box-title { font-size: 15px; font-weight: bold; margin-bottom: 16px; font-family: var(--font-display); }
.listing-form { display: flex; flex-direction: column; gap: 12px; }
.price-input-row { display: flex; gap: 8px; }
.fee-hint { font-size: 11px; opacity: 0.5; margin-top: 4px; }

/* 市场网格 */
.market-display { min-height: 500px; }
.market-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
  gap: 16px;
}

.order-card {
  position: relative;
  background: var(--panel-bg);
  border: 1px solid var(--panel-border);
  border-radius: 20px;
  padding: 20px;
  overflow: hidden;
  transition: all 0.3s ease;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.order-card:hover { transform: translateY(-4px); border-color: var(--accent-primary); box-shadow: 0 8px 24px rgba(0,0,0,0.05); }

.card-rarity-bg { position: absolute; inset: 0; opacity: 0.03; pointer-events: none; }
.q-epic .card-rarity-bg { background: #a042ff; }
.q-legendary .card-rarity-bg { background: #f0a020; opacity: 0.06; }
.q-mythic .card-rarity-bg { background: #d03050; opacity: 0.08; }

.order-item-header { display: flex; flex-direction: column; gap: 4px; }
.item-name { font-weight: bold; font-size: 16px; z-index: 1; }
.item-cat-tag { font-size: 10px; opacity: 0.5; text-transform: uppercase; letter-spacing: 1px; }

.item-preview {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px;
  background: rgba(0,0,0,0.02);
  border-radius: 12px;
}

.item-icon-wrap {
  width: 44px; height: 44px;
  background: var(--accent-muted);
  border-radius: 10px;
  display: grid; place-items: center;
  font-family: var(--font-display);
  font-size: 20px;
  color: var(--accent-primary);
}

.item-seller { display: flex; flex-direction: column; }
.item-seller .label { font-size: 10px; opacity: 0.5; }
.item-seller .value { font-size: 12px; font-weight: bold; }

.order-footer {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-top: auto;
}

.price-area { display: flex; align-items: center; gap: 6px; }
.price-val { font-size: 18px; font-weight: 900; font-variant-numeric: tabular-nums; color: #f0a020; }

.expire-hint {
  position: absolute;
  top: 12px;
  right: 12px;
  font-size: 10px;
  opacity: 0.4;
}

.is-mine { border-style: dashed; border-width: 2px; }
.mine-badge {
  position: absolute;
  top: 0; right: 0;
  background: var(--accent-primary);
  color: white;
  font-size: 9px;
  padding: 2px 8px;
  border-radius: 0 0 0 12px;
}

/* 底部我的摊位 */
.my-stalls-section {
  margin-top: 40px;
  background: var(--panel-bg);
  border: 1px solid var(--panel-border);
  border-radius: 24px;
  overflow: hidden;
  margin-bottom: 100px;
}

.section-head {
  padding: 16px 24px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  cursor: pointer;
  background: rgba(0,0,0,0.02);
}

.stalls-content { padding: 20px; }
.text-primary { color: var(--accent-primary); font-weight: bold; }

@media (max-width: 1080px) {
  .auction-layout { grid-template-columns: 1fr; }
  .market-filter { flex-direction: row; flex-wrap: wrap; }
  .filter-group, .quick-listing-box { flex: 1; min-width: 280px; }
}

@media (max-width: 768px) {
  .market-grid { grid-template-columns: 1fr; }
  .my-stalls-section { margin-bottom: 120px; }
}
</style>
