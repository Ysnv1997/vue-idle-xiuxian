<template>
  <div class="page-view inventory-page">
    <!-- 顶部标题与快速筛选 -->
    <header class="page-head">
      <div class="head-main">
        <p class="page-eyebrow">洞府藏物</p>
        <h2 class="page-title">乾坤袋</h2>
      </div>
      <div class="head-action">
        <n-button-group round>
          <n-button 
            v-for="cat in categories" 
            :key="cat.key"
            :secondary="activeCategory !== cat.key"
            :type="activeCategory === cat.key ? 'primary' : 'default'"
            @click="activeCategory = cat.key"
          >
            {{ cat.label }}
          </n-button>
        </n-button-group>
      </div>
    </header>

    <div class="inventory-layout">
      <!-- 左侧：修士化身（装备位） -->
      <aside class="character-section">
        <div class="character-doll">
          <div class="doll-overlay">
            <div class="doll-title">道躯穿戴</div>
            <div class="spirit-stones-tag">
              <n-icon><WalletOutlined /></n-icon>
              {{ formatNumber(playerStore.spiritStones) }} 灵石
            </div>
          </div>
          
          <div class="equipment-slots">
            <!-- 左右分布的槽位 -->
            <div class="slot-column left">
              <div v-for="slot in leftSlots" :key="slot" class="item-slot-wrapper">
                <div 
                  class="item-slot" 
                  :class="[getSlotQualityClass(slot), { 'is-empty': !playerStore.equippedArtifacts[slot] }]"
                  @click="handleSlotClick(slot)"
                >
                  <div class="slot-placeholder" v-if="!playerStore.equippedArtifacts[slot]">
                    {{ equipmentTypes[slot].slice(0, 1) }}
                  </div>
                  <div class="slot-icon" v-else>
                    <!-- 这里未来可以放图标 -->
                    <div class="slot-mark">{{ equipmentTypes[slot].slice(0, 1) }}</div>
                  </div>
                  <div class="slot-label">{{ equipmentTypes[slot] }}</div>
                </div>
              </div>
            </div>

            <div class="slot-column center">
              <div class="character-silhouette">
                <!-- 修仙者剪影或立绘 -->
                <div class="silhouette-placeholder">
                  <n-icon size="120" color="var(--panel-border)"><PersonOutline /></n-icon>
                </div>
              </div>
              <!-- 法宝/特殊位 -->
              <div class="item-slot-wrapper artifact-slot">
                <div 
                  class="item-slot" 
                  :class="[getSlotQualityClass('artifact'), { 'is-empty': !playerStore.equippedArtifacts.artifact }]"
                  @click="handleSlotClick('artifact')"
                >
                  <div class="slot-placeholder" v-if="!playerStore.equippedArtifacts.artifact">法</div>
                  <div class="slot-label">法宝</div>
                </div>
              </div>
            </div>

            <div class="slot-column right">
              <div v-for="slot in rightSlots" :key="slot" class="item-slot-wrapper">
                <div 
                  class="item-slot" 
                  :class="[getSlotQualityClass(slot), { 'is-empty': !playerStore.equippedArtifacts[slot] }]"
                  @click="handleSlotClick(slot)"
                >
                  <div class="slot-placeholder" v-if="!playerStore.equippedArtifacts[slot]">
                    {{ equipmentTypes[slot].slice(0, 1) }}
                  </div>
                  <div class="slot-mark" v-else>{{ equipmentTypes[slot].slice(0, 1) }}</div>
                  <div class="slot-label">{{ equipmentTypes[slot] }}</div>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- 强化材料统计 -->
        <div class="material-stats">
          <div class="m-item">
            <span class="label">强化石</span>
            <strong class="value">{{ formatNumber(playerStore.reinforceStones) }}</strong>
          </div>
          <div class="m-item">
            <span class="label">洗练石</span>
            <strong class="value">{{ formatNumber(playerStore.refinementStones) }}</strong>
          </div>
          <div class="m-item">
            <span class="label">灵宠精华</span>
            <strong class="value">{{ formatNumber(playerStore.petEssence) }}</strong>
          </div>
        </div>
      </aside>

      <!-- 右侧：物品网格 -->
      <main class="bag-section">
        <div class="bag-header">
          <div class="bag-info">
            容量：{{ playerStore.items.length }} / 500
          </div>
          <div class="bag-actions">
            <n-button size="small" quaternary @click="openBatchSellConfirm" v-if="activeCategory === 'equipment'">
              批量分解
            </n-button>
            <n-select 
              v-model:value="selectedQuality" 
              :options="qualityOptions" 
              size="small" 
              style="width: 100px"
              v-if="activeCategory === 'equipment'"
            />
          </div>
        </div>

        <n-scrollbar class="grid-scrollbar">
          <div class="item-grid">
            <!-- 物品格子 -->
            <div 
              v-for="item in filteredBagList" 
              :key="item.id" 
              class="grid-item"
              :class="[getItemQualityClass(item), { 'is-equipped': isItemEquipped(item) }]"
              @click="showItemDetails(item)"
            >
              <div class="item-icon-wrap">
                <div class="item-mark" v-if="item.type === 'pet'">宠</div>
                <div class="item-mark" v-else-if="item.type === 'pill'">丹</div>
                <div class="item-mark" v-else>{{ equipmentTypes[item.type]?.slice(0, 1) }}</div>
                
                <div class="item-count" v-if="item.count > 1">{{ item.count }}</div>
                <div class="item-enhance" v-if="item.enhanceLevel > 0">+{{ item.enhanceLevel }}</div>
              </div>
              <div class="item-name">{{ item.name }}</div>
              <div class="equipped-badge" v-if="isItemEquipped(item)">已装备</div>
            </div>
            
            <!-- 空格子填充，保持对齐 -->
            <div v-for="i in emptyGridFill" :key="'empty-' + i" class="grid-item is-empty"></div>
          </div>
          <n-empty v-if="filteredBagList.length === 0" description="袋中空空如也" style="padding: 40px 0" />
        </n-scrollbar>
      </main>
    </div>

    <!-- 物品详情侧边抽屉 -->
    <n-drawer v-model:show="showDetailDrawer" :width="min(450, '100%')" placement="right" class="detail-drawer">
      <n-drawer-content closable>
        <template #header>
          <div class="detail-header" v-if="selectedItem">
            <div class="detail-title-row">
              <h3 :style="{ color: getItemQualityColor(selectedItem) }">{{ selectedItem.name }}</h3>
              <n-tag :bordered="false" :color="{ textColor: getItemQualityColor(selectedItem), color: 'transparent' }">
                {{ getItemQualityName(selectedItem) }}
              </n-tag>
            </div>
            <div class="detail-meta">
              <span v-if="selectedItem.type === 'pet'">灵宠</span>
              <span v-else-if="selectedItem.type === 'pill'">丹药</span>
              <span v-else>装备 / {{ equipmentTypes[selectedItem.type] }}</span>
              <span class="realm-req" :class="{ 'is-met': playerStore.level >= (selectedItem.requiredRealm || 0) }">
                需求：{{ getRealmName(selectedItem.requiredRealm || 0).name }}
              </span>
            </div>
          </div>
        </template>

        <div class="detail-content" v-if="selectedItem">
          <p class="item-description">{{ selectedItem.description }}</p>

          <!-- 装备属性 -->
          <div class="item-stats-section" v-if="selectedItem.stats">
            <div class="section-title">附加属性</div>
            <div class="stats-grid">
              <div v-for="(val, key) in selectedItem.stats" :key="key" class="stat-row">
                <span class="s-label">{{ getStatName(key) }}</span>
                <span class="s-value">+{{ formatStatValue(key, val) }}</span>
              </div>
            </div>
          </div>

          <!-- 装备对比 -->
          <div class="comparison-section" v-if="itemComparison">
            <div class="section-title">属性对比</div>
            <div class="comp-table">
              <div v-for="(comp, key) in itemComparison" :key="key" class="comp-row">
                <span class="c-label">{{ getStatName(key) }}</span>
                <div class="c-vals">
                  <span class="c-current">{{ formatStatValue(key, comp.current) }}</span>
                  <n-icon><ArrowForwardOutline /></n-icon>
                  <span class="c-next" :class="comp.isPositive ? 'text-success' : 'text-error'">
                    {{ formatStatValue(key, comp.selected) }}
                  </span>
                </div>
              </div>
            </div>
          </div>

          <!-- 灵宠专属 -->
          <div class="pet-section" v-if="selectedItem.type === 'pet'">
            <div class="section-title">修为加成</div>
            <n-descriptions bordered size="small" :column="1">
              <n-descriptions-item label="等级 / 星级">{{ selectedItem.level || 1 }}级 / {{ selectedItem.star || 0 }}星</n-descriptions-item>
              <n-descriptions-item label="攻击加成">+{{ (getPetBonus(selectedItem).attack * 100).toFixed(1) }}%</n-descriptions-item>
              <n-descriptions-item label="防御加成">+{{ (getPetBonus(selectedItem).defense * 100).toFixed(1) }}%</n-descriptions-item>
            </n-descriptions>
          </div>
        </div>

        <template #footer>
          <div class="detail-footer" v-if="selectedItem">
            <n-space justify="end">
              <!-- 装备操作 -->
              <template v-if="isEquipment(selectedItem)">
                <n-button type="primary" @click="handleEquip(selectedItem)" v-if="!isItemEquipped(selectedItem)">装备</n-button>
                <n-button type="warning" @click="handleUnequip(selectedItem.type)" v-else>卸下</n-button>
                <n-button secondary type="info" @click="openEnhanceModal">强化</n-button>
                <n-button secondary type="info" @click="handleReforge">洗练</n-button>
                <n-button secondary type="warning" @click="handleQuickAuction">上架</n-button>
                <n-button secondary type="error" @click="confirmSellEquipment(selectedItem)">出售</n-button>
              </template>

              <!-- 丹药操作 -->
              <template v-else-if="selectedItem.type === 'pill'">
                <n-button type="primary" @click="usePill(selectedItem)">服用</n-button>
              </template>

              <!-- 灵宠操作 -->
              <template v-else-if="selectedItem.type === 'pet'">
                <n-button type="primary" @click="useItem(selectedItem)">
                  {{ playerStore.activePet?.id === selectedItem.id ? '召回' : '出战' }}
                </n-button>
                <n-button secondary type="info" @click="showPetModal = true">养成</n-button>
                <n-button secondary type="error" @click="confirmReleasePet(selectedItem)">放生</n-button>
              </template>
            </n-space>
          </div>
        </template>
      </n-drawer-content>
    </n-drawer>

    <!-- 原有功能的 Modals 保持不变，但视觉微调 -->
    <!-- 灵宠养成、强化确认、批量出售等 Modal... -->
    <n-modal v-model:show="showEnhanceModal" preset="dialog" title="装备强化" class="custom-modal">
      <div class="enhance-modal-body" v-if="selectedItem">
        <p>消耗 <strong class="text-primary">{{ ((selectedItem.enhanceLevel || 0) + 1) * 10 }}</strong> 强化石</p>
        <p>当前拥有：{{ playerStore.reinforceStones }}</p>
      </div>
      <template #action>
        <n-button @click="showEnhanceModal = false">取消</n-button>
        <n-button type="primary" @click="doEnhance" :disabled="playerStore.reinforceStones < ((selectedItem?.enhanceLevel || 0) + 1) * 10">
          确认强化
        </n-button>
      </template>
    </n-modal>

    <!-- 其它原有的 Modal 逻辑... -->
    <n-modal v-model:show="showPetModal" preset="dialog" title="灵宠养成" style="width: 600px">
       <div v-if="selectedItem" class="pet-upgrade-area">
          <n-tabs type="segment">
            <n-tab-pane name="level" tab="升级">
               <n-space vertical align="center" style="padding: 20px 0">
                  <n-text>消耗 {{ getUpgradeCost(selectedItem) }} 灵宠精华</n-text>
                  <n-button type="primary" @click="upgradePet(selectedItem)" :disabled="!canUpgrade(selectedItem)">
                    提升等级
                  </n-button>
               </n-space>
            </n-tab-pane>
            <n-tab-pane name="star" tab="升星">
               <n-space vertical>
                  <n-select v-model:value="selectedFoodPet" :options="getAvailableFoodPets(selectedItem)" placeholder="选择同名同品质灵宠" />
                  <n-button block type="warning" @click="evolvePet(selectedItem)" :disabled="!selectedFoodPet">
                    提升星级
                  </n-button>
               </n-space>
            </n-tab-pane>
          </n-tabs>
       </div>
    </n-modal>

    <n-modal v-model:show="showBatchSellConfirm" preset="dialog" title="批量分解装备">
      <p>确定要分解所有选中的 <strong class="text-error">{{ filteredBagList.length }}</strong> 件装备吗？</p>
      <template #action>
        <n-button @click="showBatchSellConfirm = false">取消</n-button>
        <n-button type="error" @click="batchSellEquipments">确认分解</n-button>
      </template>
    </n-modal>

    <!-- 拍卖行上架确认 -->
    <n-modal v-model:show="showAuctionListConfirm" preset="dialog" title="上架坊市">
      <n-space vertical v-if="selectedItem">
        <n-input-number v-model:value="auctionListingPrice" :min="1" placeholder="设定灵石价格" style="width: 100%" />
        <n-text depth="3">推荐价格：{{ defaultAuctionPrice(selectedItem) }} 灵石</n-text>
      </n-space>
      <template #action>
        <n-button @click="showAuctionListConfirm = false">取消</n-button>
        <n-button type="primary" @click="confirmAuctionListing">确认上架</n-button>
      </template>
    </n-modal>
  </div>
</template>

<script setup>
  import { ref, computed, onMounted } from 'vue'
  import { usePlayerStore } from '../stores/player'
  import { useMessage } from 'naive-ui'
  import { 
    PersonOutline, 
    WalletOutline, 
    ArrowForwardOutline,
    ShieldCheckmarkOutline
  } from '@vicons/ionicons5'
  import { getStatName, formatStatValue } from '../plugins/stats'
  import { getRealmName } from '../plugins/realm'
  import { createAuctionOrder } from '../api/modules/auction'
  import {
    inventorySellEquipment,
    inventoryBatchSellEquipment,
    inventoryReleasePet,
    inventoryUpgradePet,
    inventoryEvolvePet,
    gameUseItem,
    inventoryEquipEquipment,
    inventoryUnequipEquipment,
    inventoryEnhanceEquipment,
    inventoryReforgeEquipment
  } from '../api/modules/game'

  const playerStore = usePlayerStore()
  const message = useMessage()

  // 状态
  const activeCategory = ref('equipment')
  const selectedQuality = ref('all')
  const showDetailDrawer = ref(false)
  const selectedItem = ref(null)
  const showEnhanceModal = ref(false)
  const showPetModal = ref(false)
  const showBatchSellConfirm = ref(false)
  const showAuctionListConfirm = ref(false)
  const auctionListingPrice = ref(100)
  const selectedFoodPet = ref(null)

  // 常量配置
  const categories = [
    { label: '装备', key: 'equipment' },
    { label: '灵宠', key: 'pet' },
    { label: '丹药', key: 'pill' },
    { label: '素材', key: 'material' }
  ]

  const equipmentTypes = {
    weapon: '武器', head: '头部', body: '衣服', legs: '裤子', feet: '鞋子',
    shoulder: '肩甲', hands: '手套', wrist: '护腕', necklace: '项链',
    ring1: '戒指1', ring2: '戒指2', belt: '腰带', artifact: '法宝'
  }

  const leftSlots = ['weapon', 'head', 'body', 'legs', 'feet', 'shoulder']
  const rightSlots = ['hands', 'wrist', 'necklace', 'ring1', 'ring2', 'belt']

  // ---------------- 计算属性 ----------------
  const filteredBagList = computed(() => {
    return playerStore.items.filter(item => {
      // 分类过滤
      if (activeCategory.value === 'equipment') {
        if (item.type === 'pet' || item.type === 'pill' || item.type === 'material') return false
        if (selectedQuality.value !== 'all' && item.quality !== selectedQuality.value) return false
      } else {
        if (item.type !== activeCategory.value) return false
      }
      return true
    })
  })

  const emptyGridFill = computed(() => {
    const count = filteredBagList.value.length
    return count < 30 ? 30 - count : 0
  })

  const qualityOptions = [
    { label: '全部品质', value: 'all' },
    { label: '凡品', value: 'common' },
    { label: '下品', value: 'uncommon' },
    { label: '中品', value: 'rare' },
    { label: '上品', value: 'epic' },
    { label: '极品', value: 'legendary' },
    { label: '仙品', value: 'mythic' }
  ]

  const itemComparison = computed(() => {
    if (!selectedItem.value || !isEquipment(selectedItem.value)) return null
    const current = playerStore.equippedArtifacts[selectedItem.value.type]
    if (!current || current.id === selectedItem.value.id) return null
    
    const comparison = {}
    const stats = new Set([...Object.keys(selectedItem.value.stats || {}), ...Object.keys(current.stats || {})])
    stats.forEach(s => {
      const sVal = selectedItem.value.stats[s] || 0
      const cVal = current.stats[s] || 0
      comparison[s] = { current: cVal, selected: sVal, isPositive: sVal > cVal }
    })
    return comparison
  })

  // ---------------- 方法 ----------------
  const formatNumber = val => Number(val || 0).toLocaleString()
  const isEquipment = item => !['pet', 'pill', 'material'].includes(item.type)
  const isItemEquipped = item => {
    const equipped = playerStore.equippedArtifacts[item.type]
    return equipped && String(equipped.id) === String(item.id)
  }

  const getItemQualityClass = item => `q-${item.quality || item.rarity || 'common'}`
  const getSlotQualityClass = slot => {
    const item = playerStore.equippedArtifacts[slot]
    return item ? `q-${item.quality || 'common'}` : ''
  }

  const getItemQualityColor = item => {
    const q = item.quality || item.rarity
    const colors = { 
      common: '#9ab0c6', mortal: '#9ab0c6', 
      uncommon: '#18a058', spiritual: '#18a058',
      rare: '#2080f0', mystic: '#2080f0',
      epic: '#a042ff', celestial: '#a042ff',
      legendary: '#f0a020', divine: '#d03050',
      mythic: '#ff4d4f'
    }
    return colors[q] || colors.common
  }

  const getItemQualityName = item => {
    const q = item.quality || item.rarity
    const names = { 
      common: '凡品', uncommon: '下品', rare: '中品', epic: '上品', legendary: '极品', mythic: '仙品',
      mortal: '凡阶', spiritual: '灵阶', mystic: '玄阶', celestial: '仙阶', divine: '神阶'
    }
    return names[q] || '未知'
  }

  const showItemDetails = item => {
    selectedItem.value = item
    showDetailDrawer.value = true
  }

  const handleSlotClick = slot => {
    const equipped = playerStore.equippedArtifacts[slot]
    if (equipped) {
      showItemDetails(equipped)
    } else {
      activeCategory.value = 'equipment'
      // 可以在这里加个自动筛选部位的逻辑
    }
  }

  // ---------------- 核心逻辑 API 调用 (对接原 logic) ----------------
  const applyResult = res => {
    if (result?.snapshot) playerStore.applyServerSnapshot(result.snapshot)
  }

  const handleEquip = async item => {
    try {
      const res = await inventoryEquipEquipment(String(item.id))
      playerStore.applyServerSnapshot(res.snapshot)
      message.success('穿戴成功')
      showDetailDrawer.value = false
    } catch (e) { message.error(e.message || '装备失败') }
  }

  const handleUnequip = async slot => {
    try {
      const res = await inventoryUnequipEquipment(slot)
      playerStore.applyServerSnapshot(res.snapshot)
      message.success('已卸下')
      showDetailDrawer.value = false
    } catch (e) { message.error(e.message || '卸下失败') }
  }

  const openEnhanceModal = () => { showEnhanceModal.value = true }
  const doEnhance = async () => {
    try {
      const res = await inventoryEnhanceEquipment(String(selectedItem.value.id))
      playerStore.applyServerSnapshot(res.snapshot)
      message.success('强化成功')
      showEnhanceModal.value = false
      selectedItem.value = playerStore.items.find(i => i.id === selectedItem.value.id)
    } catch (e) { message.error(e.message || '强化失败') }
  }

  const handleReforge = async () => {
    try {
      const res = await inventoryReforgeEquipment(String(selectedItem.value.id))
      playerStore.applyServerSnapshot(res.snapshot)
      message.success('洗练完成')
      selectedItem.value = playerStore.items.find(i => i.id === selectedItem.value.id)
    } catch (e) { message.error(e.message || '洗练失败') }
  }

  const usePill = async pill => {
    try {
      const res = await gameUseItem(String(pill.id))
      playerStore.applyServerSnapshot(res.snapshot)
      message.success('服用成功')
      showDetailDrawer.value = false
    } catch (e) { message.error(e.message || '服用失败') }
  }

  const useItem = async item => {
    try {
      const res = await gameUseItem(String(item.id))
      playerStore.applyServerSnapshot(res.snapshot)
      message.success('操作成功')
    } catch (e) { message.error(e.message || '操作失败') }
  }

  const batchSellEquipments = async () => {
    try {
      const res = await inventoryBatchSellEquipment({
        quality: selectedQuality.value === 'all' ? '' : selectedQuality.value,
        equipmentType: ''
      })
      playerStore.applyServerSnapshot(res.snapshot)
      message.success('批量分解成功')
      showBatchSellConfirm.value = false
    } catch (e) { message.error('操作失败') }
  }

  const handleQuickAuction = () => {
    auctionListingPrice.value = defaultAuctionPrice(selectedItem.value)
    showAuctionListConfirm.value = true
  }

  const confirmAuctionListing = async () => {
    try {
      const res = await createAuctionOrder({
        itemId: String(selectedItem.value.id),
        price: Math.floor(auctionListingPrice.value),
        durationHours: 24
      })
      playerStore.applyServerSnapshot(res.snapshot)
      message.success('上架成功')
      showAuctionListConfirm.value = false
      showDetailDrawer.value = false
    } catch (e) { message.error('上架失败') }
  }

  // ---------------- 灵宠逻辑 ----------------
  const getPetBonus = pet => {
    if (!pet) return { attack: 0, defense: 0, health: 0 }
    const qualityBonusMap = { divine: 0.5, celestial: 0.3, mystic: 0.2, spiritual: 0.1, mortal: 0.05 }
    const baseBonus = qualityBonusMap[pet.rarity] || 0.05
    const starBonus = (pet.star || 0) * 0.05
    const final = baseBonus + starBonus
    return { attack: final, defense: final, health: final }
  }
  const getUpgradeCost = pet => (pet.level || 1) * 10
  const canUpgrade = pet => playerStore.petEssence >= getUpgradeCost(pet)
  const upgradePet = async pet => {
    try {
      const res = await inventoryUpgradePet(String(pet.id))
      playerStore.applyServerSnapshot(res.snapshot)
      message.success('升级成功')
      selectedItem.value = playerStore.items.find(i => i.id === pet.id)
    } catch (e) { message.error('升级失败') }
  }
  const getAvailableFoodPets = pet => playerStore.items
    .filter(i => i.type === 'pet' && i.id !== pet.id && i.rarity === pet.rarity && i.name === pet.name)
    .map(i => ({ label: `${i.name} (${i.level}级)`, value: i.id }))
  
  const evolvePet = async pet => {
    try {
      const res = await inventoryEvolvePet(String(pet.id), String(selectedFoodPet.value))
      playerStore.applyServerSnapshot(res.snapshot)
      message.success('升星成功')
      selectedFoodPet.value = null
      selectedItem.value = playerStore.items.find(i => i.id === pet.id)
    } catch (e) { message.error('升星失败') }
  }

  const defaultAuctionPrice = item => {
    const base = { common: 100, uncommon: 200, rare: 500, epic: 1200, legendary: 3000, mythic: 8000 }
    return base[item.quality] || 100
  }

  const confirmSellEquipment = item => {
    // 简化直接卖
    inventorySellEquipment(String(item.id)).then(res => {
      playerStore.applyServerSnapshot(res.snapshot)
      message.success('出售成功')
      showDetailDrawer.value = false
    })
  }

  const confirmReleasePet = item => {
    inventoryReleasePet(String(item.id)).then(res => {
      playerStore.applyServerSnapshot(res.snapshot)
      message.success('已放生')
      showDetailDrawer.value = false
    })
  }

  const min = (a, b) => (a < b ? a : b)
</script>

<style scoped>
.inventory-page {
  display: flex;
  flex-direction: column;
  height: 100%;
}

.inventory-layout {
  display: grid;
  grid-template-columns: 380px 1fr;
  gap: 20px;
  margin-top: 20px;
  flex: 1;
}

/* 角色部分 */
.character-section {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.character-doll {
  position: relative;
  background: var(--panel-bg);
  border: 1px solid var(--panel-border);
  border-radius: 24px;
  padding: 24px;
  height: 520px;
  display: flex;
  flex-direction: column;
}

.doll-overlay {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}

.doll-title { font-family: var(--font-display); font-size: 20px; }
.spirit-stones-tag { font-size: 13px; color: var(--accent-primary); display: flex; align-items: center; gap: 4px; }

.equipment-slots {
  display: flex;
  justify-content: space-between;
  flex: 1;
}

.slot-column {
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  gap: 8px;
}

.slot-column.center {
  justify-content: center;
  align-items: center;
}

.character-silhouette {
  width: 140px;
  height: 300px;
  display: grid;
  place-items: center;
}

.item-slot {
  width: 52px;
  height: 52px;
  background: rgba(0,0,0,0.05);
  border: 1px solid var(--panel-border);
  border-radius: 12px;
  position: relative;
  cursor: pointer;
  transition: all 0.2s ease;
  display: grid;
  place-items: center;
}

.item-slot:hover { border-color: var(--accent-primary); transform: scale(1.05); }
.item-slot.is-empty { border-style: dashed; }

.slot-placeholder { font-size: 18px; color: var(--ink-sub); opacity: 0.3; font-family: var(--font-display); }
.slot-label { position: absolute; bottom: -18px; left: 0; right: 0; text-align: center; font-size: 10px; color: var(--ink-sub); }
.slot-mark { font-family: var(--font-display); font-size: 20px; opacity: 0.8; }

/* 物品网格 */
.bag-section {
  background: var(--panel-bg);
  border: 1px solid var(--panel-border);
  border-radius: 24px;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.bag-header {
  padding: 16px 20px;
  border-bottom: 1px solid var(--panel-border);
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.grid-scrollbar { flex: 1; }

.item-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(72px, 1fr));
  gap: 12px;
  padding: 20px;
}

.grid-item {
  aspect-ratio: 1;
  background: rgba(0,0,0,0.03);
  border: 1px solid var(--panel-border);
  border-radius: 14px;
  cursor: pointer;
  transition: all 0.2s ease;
  position: relative;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 4px;
}

.grid-item:hover { border-color: var(--accent-primary); background: var(--accent-muted); }
.grid-item.is-empty { cursor: default; opacity: 0.3; }
.grid-item.is-equipped::after {
  content: 'E';
  position: absolute;
  top: 4px;
  right: 4px;
  background: var(--accent-primary);
  color: white;
  font-size: 8px;
  width: 14px;
  height: 14px;
  border-radius: 4px;
  display: grid;
  place-items: center;
  font-weight: bold;
}

.item-icon-wrap { width: 100%; height: 100%; display: grid; place-items: center; position: relative; }
.item-mark { font-family: var(--font-display); font-size: 24px; opacity: 0.6; }
.item-count { position: absolute; bottom: 2px; right: 4px; font-size: 10px; font-weight: bold; }
.item-enhance { position: absolute; top: 2px; left: 4px; font-size: 10px; color: var(--accent-primary); }
.item-name { font-size: 10px; text-align: center; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; width: 100%; margin-top: 2px; }

/* 品质颜色 */
.q-common { border-color: #9ab0c6; }
.q-uncommon { border-color: #18a058; }
.q-rare { border-color: #2080f0; }
.q-epic { border-color: #a042ff; }
.q-legendary { border-color: #f0a020; }
.q-mythic { border-color: #ff4d4f; }
.q-divine { border-color: #d03050; box-shadow: 0 0 10px rgba(208, 48, 80, 0.2); }

/* 材料统计 */
.material-stats {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 10px;
}
.m-item {
  background: var(--panel-bg);
  border: 1px solid var(--panel-border);
  border-radius: 16px;
  padding: 12px;
  display: flex;
  flex-direction: column;
  align-items: center;
}
.m-item .label { font-size: 11px; color: var(--ink-sub); }
.m-item .value { font-size: 14px; font-weight: bold; color: var(--accent-primary); margin-top: 2px; }

/* 详情侧边 */
.detail-drawer {
  --n-drawer-border-radius: 24px 0 0 24px;
}

.detail-title-row { display: flex; align-items: center; gap: 12px; margin-bottom: 4px; }
.detail-title-row h3 { margin: 0; font-family: var(--font-display); font-size: 24px; }
.detail-meta { font-size: 13px; color: var(--ink-sub); display: flex; gap: 16px; }
.realm-req.is-met { color: var(--accent-primary); }

.detail-content { display: flex; flex-direction: column; gap: 24px; padding: 10px 0; }
.section-title { font-size: 14px; font-weight: bold; margin-bottom: 12px; color: var(--ink-main); border-left: 4px solid var(--accent-primary); padding-left: 10px; }

.stats-grid { display: grid; grid-template-columns: repeat(2, 1fr); gap: 12px; }
.stat-row { background: rgba(0,0,0,0.02); padding: 10px; border-radius: 10px; display: flex; justify-content: space-between; }
.s-label { font-size: 12px; color: var(--ink-sub); }
.s-value { font-size: 14px; font-weight: bold; }

.comp-table { display: flex; flex-direction: column; gap: 8px; }
.comp-row { display: flex; justify-content: space-between; align-items: center; padding: 8px 12px; background: rgba(0,0,0,0.02); border-radius: 10px; }
.c-vals { display: flex; align-items: center; gap: 12px; }
.c-current { font-size: 12px; opacity: 0.6; }
.c-next { font-weight: bold; }

@media (max-width: 1080px) {
  .inventory-layout { grid-template-columns: 1fr; }
  .character-section { order: 2; }
  .bag-section { order: 1; height: 500px; }
  .character-doll { height: auto; }
}

@media (max-width: 768px) {
  .equipment-slots { flex-direction: column; align-items: center; gap: 40px; }
  .slot-column { flex-direction: row; flex-wrap: wrap; justify-content: center; }
  .character-silhouette { display: none; }
  .item-slot { width: 48px; height: 48px; }
}
</style>
