<template>
  <section class="page-view auction-view">
    <header class="page-head">
      <p class="page-eyebrow">坊市交易</p>
      <h2>坊市商城</h2>
      <p class="page-desc">卖家定价，买家直购。竞价功能已关闭。</p>
    </header>

    <n-card :bordered="false" class="page-card">
      <template #header-extra>
        <n-space>
          <n-button :loading="auctionStore.loading" @click="refresh">刷新</n-button>
        </n-space>
      </template>
      <n-space vertical>
        <n-grid :cols="24" :x-gap="12" :y-gap="12">
          <n-grid-item :span="24">
            <n-card size="small" title="上架商品">
              <n-space align="center" wrap>
                <n-select
                  v-model:value="selectedItemId"
                  :options="tradableItemOptions"
                  placeholder="选择要上架的商品"
                  filterable
                  class="listing-select"
                  :disabled="auctionStore.submitting"
                  @update:value="handleSelectListingItem"
                />
                <n-input-number
                  v-model:value="price"
                  :min="1"
                  :step="10"
                  placeholder="价格(灵石)"
                  class="listing-price"
                  :disabled="auctionStore.submitting"
                />
                <n-button type="primary" :loading="auctionStore.submitting" @click="createOrder">上架</n-button>
              </n-space>
              <n-text depth="3" style="display: block; margin-top: 8px">
                支持装备、灵草、丹药、丹方残页、灵宠上架，成交收取 5% 手续费，上架时效默认 24 小时。
              </n-text>
            </n-card>
          </n-grid-item>

          <n-grid-item :span="24">
            <n-card size="small" title="在售商品">
              <n-space align="center" wrap style="margin-bottom: 10px">
                <n-select
                  v-model:value="filterCategory"
                  :options="categoryOptions"
                  style="width: 180px"
                  placeholder="分类筛选"
                  @update:value="handleCategoryChange"
                />
                <n-select
                  v-if="filterCategory === 'equipment'"
                  v-model:value="filterSubCategory"
                  :options="equipmentSubCategoryOptions"
                  style="width: 180px"
                  placeholder="装备子类"
                />
                <n-button size="small" :loading="auctionStore.loading" @click="refresh">应用筛选</n-button>
              </n-space>
              <n-spin :show="auctionStore.loading || auctionStore.submitting">
                <div class="auction-table-wrap">
                  <n-table striped size="small">
                  <thead>
                    <tr>
                      <th style="width: 70px">订单</th>
                      <th>商品</th>
                      <th style="width: 110px">卖家</th>
                      <th style="width: 120px">分类</th>
                      <th style="width: 90px">价格</th>
                      <th style="width: 80px">状态</th>
                      <th style="width: 140px">过期时间</th>
                      <th style="width: 140px">操作</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-for="order in auctionStore.openOrders" :key="order.id">
                      <td>#{{ order.id }}</td>
                      <td>{{ formatOrderItem(order) }}</td>
                      <td>{{ order.sellerName || '-' }}</td>
                      <td>{{ formatOrderCategory(order) }}</td>
                      <td>{{ order.price }}</td>
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
                            v-if="order.isMine"
                            size="small"
                            secondary
                            :disabled="order.status !== 'open'"
                            :loading="auctionStore.submitting"
                            @click="cancelOrder(order.id)"
                          >
                            取消
                          </n-button>
                        </n-space>
                      </td>
                    </tr>
                    <tr v-if="auctionStore.openOrders.length === 0">
                      <td colspan="8">
                        <n-empty description="暂无在售商品" />
                      </td>
                    </tr>
                  </tbody>
                  </n-table>
                </div>
              </n-spin>
            </n-card>
          </n-grid-item>

          <n-grid-item :span="24">
            <n-card size="small" title="我的订单">
              <div class="auction-table-wrap">
                <n-table striped size="small">
                <thead>
                  <tr>
                    <th style="width: 70px">订单</th>
                    <th>商品</th>
                    <th style="width: 120px">分类</th>
                    <th style="width: 80px">价格</th>
                    <th style="width: 80px">状态</th>
                    <th style="width: 80px">买家</th>
                    <th style="width: 160px">时间</th>
                    <th style="width: 140px">操作</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="order in auctionStore.myOrders" :key="`my-${order.id}`">
                    <td>#{{ order.id }}</td>
                    <td>{{ formatOrderItem(order) }}</td>
                    <td>{{ formatOrderCategory(order) }}</td>
                    <td>{{ order.price }}</td>
                    <td>{{ formatStatus(order) }}</td>
                    <td>{{ order.buyerUserId ? '已成交' : '-' }}</td>
                    <td>{{ formatTime(order.createdAt) }}</td>
                    <td>
                      <n-button
                        size="small"
                        secondary
                        :disabled="!order.isMine || order.status !== 'open'"
                        :loading="auctionStore.submitting"
                        @click="cancelOrder(order.id)"
                      >
                        取消
                      </n-button>
                    </td>
                  </tr>
                  <tr v-if="auctionStore.myOrders.length === 0">
                    <td colspan="8">
                      <n-empty description="暂无我的订单" />
                    </td>
                  </tr>
                </tbody>
                </n-table>
              </div>
            </n-card>
          </n-grid-item>
        </n-grid>
      </n-space>
    </n-card>
  </section>
</template>

<script setup>
  import { computed, onMounted, ref } from 'vue'
  import { useMessage } from 'naive-ui'
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

  const categoryOptions = [
    { label: '全部分类', value: '' },
    { label: '装备', value: 'equipment' },
    { label: '灵草', value: 'herb' },
    { label: '丹药', value: 'pill' },
    { label: '丹方残页', value: 'pill_fragment' },
    { label: '灵宠', value: 'pet' }
  ]

  const equipmentTypeNames = {
    weapon: '武器',
    head: '头部',
    body: '衣服',
    legs: '裤子',
    feet: '鞋子',
    shoulder: '肩甲',
    hands: '手套',
    wrist: '护腕',
    necklace: '项链',
    ring1: '戒指1',
    ring2: '戒指2',
    belt: '腰带',
    artifact: '法宝'
  }

  const equipmentSubCategoryOptions = computed(() => [
    { label: '全部子类', value: '' },
    ...Object.entries(equipmentTypeNames).map(([key, name]) => ({
      label: name,
      value: key
    }))
  ])

  const tradableItemOptions = computed(() => {
    const options = []
    const items = Array.isArray(playerStore.items) ? playerStore.items : []
    for (const item of items) {
      const itemType = readString(item?.type)
      if (!isTradableType(itemType)) {
        continue
      }
      const itemName = readString(item?.name) || '未知物品'
      const quality = readString(item?.quality)
      const categoryLabel = formatTypeForListing(itemType)
      options.push({
        label: quality
          ? `${itemName} [${quality}] | ${categoryLabel}`
          : `${itemName} | ${categoryLabel}`,
        value: readString(item?.id),
        recommendPrice: getRecommendPriceByInventoryItem(item)
      })
    }

    const herbs = Array.isArray(playerStore.herbs) ? playerStore.herbs : []
    const herbGroups = new Map()
    for (const herb of herbs) {
      const herbID = readString(herb?.id)
      const herbQuality = readString(herb?.quality)
      if (!herbID || !herbQuality) continue
      const key = `${herbID}:${herbQuality}`
      if (!herbGroups.has(key)) {
        herbGroups.set(key, {
          id: herbID,
          quality: herbQuality,
          name: readString(herb?.name) || herbID,
          count: 0
        })
      }
      herbGroups.get(key).count += 1
    }
    for (const herb of herbGroups.values()) {
      options.push({
        label: `灵草 | ${herb.name} [${formatHerbQuality(herb.quality)}] x${herb.count}`,
        value: `herb:${herb.id}:${herb.quality}`,
        recommendPrice: getHerbRecommendPrice(herb.quality)
      })
    }

    const fragments = playerStore.pillFragments || {}
    for (const [recipeId, countRaw] of Object.entries(fragments)) {
      const count = Number(countRaw || 0)
      if (count <= 0) continue
      options.push({
        label: `丹方残页 | ${resolveRecipeName(recipeId)} x${count}`,
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
    } catch (error) {
      message.error(error?.message || '刷新坊市失败')
    }
  }

  const createOrder = async () => {
    if (!selectedItemId.value) {
      message.warning('请选择要上架的商品')
      return
    }
    if (!price.value || price.value <= 0) {
      message.warning('请输入有效价格')
      return
    }

    try {
      const result = await auctionStore.createOrder({
        itemId: selectedItemId.value,
        price: Math.floor(Number(price.value)),
        durationHours: 24
      })
      message.success(result?.message || '上架成功')
      selectedItemId.value = null
      price.value = 100
    } catch (error) {
      message.error(error?.message || '上架失败')
    }
  }

  const handleSelectListingItem = value => {
    if (!value) {
      price.value = 100
      return
    }
    const option = tradableItemOptions.value.find(item => item.value === value)
    if (!option) {
      return
    }
    const recommendPrice = Math.floor(Number(option.recommendPrice || 0))
    if (recommendPrice > 0) {
      price.value = recommendPrice
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

  const cancelOrder = async orderId => {
    try {
      const result = await auctionStore.cancelOrder(orderId)
      message.success(result?.message || '取消上架成功')
    } catch (error) {
      message.error(error?.message || '取消上架失败')
    }
  }

  const handleCategoryChange = value => {
    if (value !== 'equipment') {
      filterSubCategory.value = ''
    }
  }

  const formatOrderItem = order => {
    const item = order?.item || {}
    const itemType = readString(item?.type)
    if (itemType === 'pill_fragment') {
      return `${readString(item?.name) || '丹方残页'} x${Number(item?.count || 1)}`
    }
    if (itemType === 'herb') {
      return `${readString(item?.name) || '灵草'} [${formatHerbQuality(readString(item?.quality))}]`
    }
    const name = readString(item?.name) || '未知物品'
    const quality = readString(item?.quality)
    return quality ? `${name} [${quality}]` : name
  }

  const formatOrderCategory = order => {
    const { category, subCategory } = resolveOrderCategory(order)
    const categoryText = {
      equipment: '装备',
      herb: '灵草',
      pill: '丹药',
      pill_fragment: '丹方残页',
      pet: '灵宠',
      other: '其他'
    }[category] || '其他'
    if (category !== 'equipment') {
      return categoryText
    }
    return `${categoryText}/${equipmentTypeNames[subCategory] || subCategory || '-'}`
  }

  const formatStatus = order => {
    const status = readString(order?.status)
    if (status === 'open' && isExpired(order?.expiresAt)) {
      return '已过期'
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

  const resolveOrderCategory = order => {
    const category = readString(order?.category)
    const subCategory = readString(order?.subCategory)
    if (category) {
      return { category, subCategory }
    }
    const itemType = readString(order?.item?.type)
    if (Object.prototype.hasOwnProperty.call(equipmentTypeNames, itemType)) {
      return { category: 'equipment', subCategory: itemType }
    }
    if (['herb', 'pill', 'pill_fragment', 'pet'].includes(itemType)) {
      return { category: itemType, subCategory: '' }
    }
    return { category: 'other', subCategory: '' }
  }

  const isTradableType = itemType => {
    return ['pill', 'pet', ...Object.keys(equipmentTypeNames)].includes(itemType)
  }

  const formatTypeForListing = itemType => {
    if (itemType === 'pill') return '丹药'
    if (itemType === 'pet') return '灵宠'
    return equipmentTypeNames[itemType] ? `装备/${equipmentTypeNames[itemType]}` : '其他'
  }

  const getRecommendPriceByInventoryItem = item => {
    const itemType = readString(item?.type)
    if (itemType === 'pet') {
      return getPetRecommendPrice(readString(item?.rarity))
    }
    if (itemType === 'pill') {
      return getPillRecommendPrice(item)
    }
    return getEquipmentRecommendPrice(readString(item?.quality))
  }

  const getEquipmentRecommendPrice = quality => {
    const equipmentBase = {
      common: 80,
      uncommon: 150,
      rare: 300,
      epic: 700,
      legendary: 1500,
      mythic: 3200
    }
    return equipmentBase[quality] || 120
  }

  const getPetRecommendPrice = rarity => {
    const petBase = {
      mortal: 200,
      spiritual: 450,
      mystic: 1000,
      celestial: 2200,
      divine: 5000
    }
    return petBase[rarity] || 300
  }

  const getPillRecommendPrice = item => {
    const grade = readString(item?.grade)
    const byGrade = {
      grade1: 120,
      grade2: 180,
      grade3: 260,
      grade4: 420,
      grade5: 640,
      grade6: 950,
      grade7: 1400,
      grade8: 2100,
      grade9: 3200
    }
    if (byGrade[grade]) {
      return byGrade[grade]
    }
    return getEquipmentRecommendPrice(readString(item?.quality))
  }

  const getHerbRecommendPrice = quality => {
    const herbBase = {
      common: 60,
      uncommon: 120,
      rare: 260,
      epic: 600,
      legendary: 1500
    }
    return herbBase[quality] || 80
  }

  const getFragmentRecommendPrice = recipeId => {
    const recipe = pillRecipes.find(item => item.id === recipeId)
    const grade = readString(recipe?.grade)
    const byGrade = {
      grade1: 100,
      grade2: 150,
      grade3: 220,
      grade4: 320,
      grade5: 460,
      grade6: 650,
      grade7: 900,
      grade8: 1250,
      grade9: 1750
    }
    return byGrade[grade] || 180
  }

  const recipeNameMap = computed(() => {
    const map = {}
    for (const recipe of pillRecipes) {
      map[recipe.id] = recipe.name
    }
    return map
  })

  const resolveRecipeName = recipeId => {
    return recipeNameMap.value[recipeId] || recipeId
  }

  const formatHerbQuality = quality => {
    switch (quality) {
      case 'common':
        return '普通'
      case 'uncommon':
        return '优质'
      case 'rare':
        return '稀有'
      case 'epic':
        return '极品'
      case 'legendary':
        return '仙品'
      default:
        return quality || '未知'
    }
  }

  const readString = value => {
    if (typeof value === 'string') return value
    if (typeof value === 'number') return String(value)
    return ''
  }

  onMounted(() => {
    refresh()
  })
</script>

<style scoped>
  .auction-table-wrap {
    width: 100%;
    overflow-x: auto;
  }

  @media (max-width: 768px) {
    .listing-select,
    .listing-price,
    :deep(.n-space > .n-button) {
      width: 100% !important;
    }

    :deep(.n-base-selection),
    :deep(.n-input-number) {
      width: 100% !important;
    }
  }
</style>
