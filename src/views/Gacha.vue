<template>
  <div class="page-view gacha-page">
    <!-- 顶部标题与资源 -->
    <header class="page-head">
      <div class="head-main">
        <p class="page-eyebrow">机缘天定 · 寻宝觅踪</p>
        <h2 class="page-title">机缘阁</h2>
      </div>
      <div class="head-resource">
        <div class="resource-chip">
          <n-icon><WalletOutline /></n-icon>
          <span>灵石：{{ formatNumber(playerStore.spiritStones) }}</span>
        </div>
      </div>
    </header>

    <div class="gacha-layout">
      <!-- 左侧：卡池选择 -->
      <aside class="pool-selection">
        <div class="section-title">选择机缘池</div>
        <div class="pool-cards">
          <div 
            v-for="(label, key) in poolLabels" 
            :key="key"
            class="pool-card"
            :class="{ 'is-active': gachaType === key, [`pool-${key}`]: true }"
            @click="gachaType = key"
          >
            <div class="pool-icon">
              <n-icon size="32">
                <component :is="poolIcons[key]" />
              </n-icon>
            </div>
            <div class="pool-info">
              <div class="pool-name">{{ label }}</div>
              <div class="pool-cost">100 灵石/次</div>
            </div>
            <div class="active-glow"></div>
          </div>
        </div>

        <!-- 辅助功能按钮 -->
        <div class="utility-actions">
          <n-button quaternary block @click="showProbabilityInfo = true">
            <template #icon><n-icon><HelpCircleOutline /></n-icon></template>
            概率说明
          </n-button>
          <n-button quaternary block @click="showWishlistSettings = true" :type="playerStore.wishlistEnabled ? 'primary' : 'default'">
            <template #icon><n-icon><HeartOutline /></n-icon></template>
            心愿单 {{ playerStore.wishlistEnabled ? '(开启)' : '' }}
          </n-button>
          <n-button quaternary block @click="showAutoSettings = true">
            <template #icon><n-icon><SettingsOutline /></n-icon></template>
            自动处理
          </n-button>
        </div>
      </aside>

      <!-- 中间：仪式感展示区 -->
      <main class="gacha-stage">
        <div class="stage-bg">
          <div class="star-grid"></div>
          <div class="portal-rings" :class="{ 'is-spinning': isDrawing }">
            <div class="ring outer"></div>
            <div class="ring middle"></div>
            <div class="ring inner"></div>
          </div>
        </div>

        <div class="portal-center">
          <div class="gacha-orb" :class="{ 'is-shaking': isShaking, 'is-opening': isOpening }">
            <div class="orb-inner">
              <n-icon size="80" color="#fff">
                <component :is="poolIcons[gachaType]" />
              </n-icon>
            </div>
            <div class="orb-glow"></div>
          </div>
        </div>

        <div class="gacha-actions">
          <div class="action-group">
            <n-button
              v-for="num in [1, 10, 50, 100]"
              :key="num"
              type="primary"
              :secondary="num === 1"
              round
              size="large"
              class="draw-btn"
              @click="performGacha(num)"
              :disabled="!canAfford(num) || isDrawing"
            >
              寻宝 {{ num }} 次
              <template #suffix>
                <span class="cost-tag">{{ formatNumber(getGachaCost(num)) }}</span>
              </template>
            </n-button>
          </div>
          <p class="wishlist-hint" v-if="playerStore.wishlistEnabled">
            心愿单生效中：消耗翻倍，指定品质概率提升
          </p>
        </div>
      </main>
    </div>

    <!-- 抽卡结果全屏层 -->
    <n-modal
      v-model:show="showResult"
      preset="card"
      :style="{ maxWidth: '1000px' }"
      class="result-modal"
      title="所得机缘"
      closable
    >
      <div class="result-container">
        <!-- 结果顶部统计/筛选 -->
        <div class="result-header">
          <n-space align="center">
            <n-text depth="3">本次寻宝共获得 {{ gachaResult?.length }} 件宝物</n-text>
            <n-select
              v-if="gachaType === 'equipment'"
              v-model:value="selectedQuality"
              size="small"
              placeholder="品质筛选"
              :options="equipmentQualityOptions"
              clearable
              style="width: 120px"
            />
            <n-select
              v-if="gachaType === 'pet'"
              v-model:value="selectedRarity"
              size="small"
              placeholder="品质筛选"
              :options="petRarityOptions"
              clearable
              style="width: 120px"
            />
          </n-space>
          <n-button type="primary" secondary size="small" @click="performGacha(gachaNumber)" :disabled="!canAfford(gachaNumber) || isDrawing">
            再寻 {{ gachaNumber }} 次
          </n-button>
        </div>

        <!-- 结果网格 -->
        <n-scrollbar style="max-height: 60vh">
          <div class="result-grid">
            <div
              v-for="(item, index) in currentPageResults"
              :key="item.id || index"
              class="result-card"
              :class="[getItemQualityClass(item), { 'is-wish': isWishItem(item) }]"
              :style="{ '--delay': `${(index % pageSize) * 0.05}s` }"
            >
              <div class="item-rarity-bg"></div>
              <div class="item-content">
                <div class="item-name">{{ item.name }}</div>
                <div class="item-type">{{ getItemTypeName(item) }}</div>
                <div class="item-quality">{{ getItemQualityName(item) }}</div>
              </div>
              <div class="wish-star" v-if="isWishItem(item)">★</div>
            </div>
          </div>
          <n-empty v-if="currentPageResults.length === 0" description="未匹配到符合筛选的宝物" />
        </n-scrollbar>

        <div class="result-footer">
          <n-pagination
            v-model:page="currentPage"
            :page-count="totalPages"
            :page-size="pageSize"
            simple
          />
        </div>
      </div>
    </n-modal>

    <!-- 心愿单、概率、自动处理 Modal (结构微调) -->
    <n-modal v-model:show="showProbabilityInfo" preset="dialog" title="寻宝机缘概率" class="custom-modal" style="width: 600px">
      <n-tabs type="segment" animated>
        <n-tab-pane name="all" tab="综合池">
          <div class="prob-list">
            <div class="prob-group-title">大类概率</div>
            <div class="prob-row"><span>装备 / 灵宠</span> <strong>50% / 50%</strong></div>
            <n-divider />
            <div class="prob-group-title">品质分布 (当前境界)</div>
            <div v-for="(prob, q) in getAllPoolProbabilities().equipment" :key="q" class="prob-row" v-if="q !== 'none'">
              <span :style="{ color: equipmentQualities[q]?.color }">{{ equipmentQualities[q]?.name }}</span>
              <strong>{{ (prob * 100).toFixed(2) }}%</strong>
            </div>
          </div>
        </n-tab-pane>
        <!-- 更多 Tab 保持逻辑... -->
      </n-tabs>
    </n-modal>

    <!-- 心愿单设置 -->
    <n-modal v-model:show="showWishlistSettings" preset="dialog" title="感悟心愿" class="custom-modal" style="width: 500px">
      <div class="wishlist-setup">
        <div class="setup-row">
          <span>启用心愿祈福</span>
          <n-switch v-model:value="playerStore.wishlistEnabled" />
        </div>
        <n-divider />
        <div class="setup-group">
          <div class="label">心仪装备品质</div>
          <n-select v-model:value="playerStore.selectedWishEquipQuality" :options="equipmentQualityOptions" clearable placeholder="请选择" :disabled="!playerStore.wishlistEnabled" />
        </div>
        <div class="setup-group">
          <div class="label">心仪灵宠品质</div>
          <n-select v-model:value="playerStore.selectedWishPetRarity" :options="petRarityOptions" clearable placeholder="请选择" :disabled="!playerStore.wishlistEnabled" />
        </div>
        <div class="setup-hint">开启后寻宝消耗翻倍，但所选品质出现的几率将大幅提升。</div>
      </div>
    </n-modal>

    <!-- 自动处理 -->
    <n-modal v-model:show="showAutoSettings" preset="dialog" title="机缘清理" class="custom-modal" style="width: 500px">
      <div class="auto-setup">
        <div class="setup-group">
          <div class="label">装备自动分解</div>
          <n-checkbox-group v-model:value="playerStore.autoSellQualities" @update:value="handleAutoSellChange">
            <n-space wrap>
              <n-checkbox v-for="(q, key) in equipmentQualities" :key="key" :value="key">
                <span :style="{ color: q.color }">{{ q.name }}</span>
              </n-checkbox>
            </n-space>
          </n-checkbox-group>
        </div>
        <n-divider />
        <div class="setup-group">
          <div class="label">灵宠自动放生</div>
          <n-checkbox-group v-model:value="playerStore.autoReleaseRarities" @update:value="handleAutoReleaseChange">
            <n-space wrap>
              <n-checkbox v-for="(r, key) in petRarities" :key="key" :value="key">
                <span :style="{ color: r.color }">{{ r.name }}</span>
              </n-checkbox>
            </n-space>
          </n-checkbox-group>
        </div>
      </div>
    </n-modal>
  </div>
</template>

<script setup>
  import { usePlayerStore } from '../stores/player'
  import { computed, ref, watch } from 'vue'
  import { useMessage } from 'naive-ui'
  import { 
    WalletOutline, 
    CompassOutline, 
    GiftOutline, 
    SparklesOutline,
    HelpCircleOutline,
    HeartOutline,
    SettingsOutline,
    CubeOutline,
    EggOutline
  } from '@vicons/ionicons5'
  import { drawGacha } from '../api/modules/game'

  const playerStore = usePlayerStore()
  const message = useMessage()

  // 抽卡类型
  const gachaType = ref('all') 
  const isShaking = ref(false)
  const isOpening = ref(false)
  const showResult = ref(false)
  const gachaResult = ref([])
  const showProbabilityInfo = ref(false)
  const isDrawing = ref(false)

  // 结果弹窗相关
  const currentPage = ref(1)
  const pageSize = ref(20)
  const selectedQuality = ref(null) 
  const selectedRarity = ref(null) 
  const showAutoSettings = ref(false) 
  const showWishlistSettings = ref(false) 

  const poolLabels = { all: '包罗万象池', equipment: '神兵利器池', pet: '奇珍异兽池' }
  const poolIcons = { all: GiftOutline, equipment: CubeOutline, pet: EggOutline }

  // 装备与灵宠配置
  const equipmentQualities = {
    common: { name: '凡品', color: '#94a3b8' },
    uncommon: { name: '下品', color: '#18a058' },
    rare: { name: '中品', color: '#2080f0' },
    epic: { name: '上品', color: '#a042ff' },
    legendary: { name: '极品', color: '#f0a020' },
    mythic: { name: '仙品', color: '#d03050' }
  }

  const petRarities = {
    mortal: { name: '凡品', color: '#94a3b8', probability: 0.23 },
    spiritual: { name: '灵品', color: '#18a058', probability: 0.1 },
    mystic: { name: '玄品', color: '#2080f0', probability: 0.02 },
    celestial: { name: '仙品', color: '#f0a020', probability: 0.0012 },
    divine: { name: '神品', color: '#d03050', probability: 0.0003 }
  }

  const getEquipProbabilities = {
    common: 0.38, uncommon: 0.24, rare: 0.08, epic: 0.015, legendary: 0.0015, mythic: 0.0005
  }

  const formatNumber = val => Number(val || 0).toLocaleString()
  const getGachaCost = times => playerStore.wishlistEnabled ? times * 200 : times * 100
  const canAfford = times => playerStore.spiritStones >= getGachaCost(times)

  const gachaNumber = ref(1)

  const performGacha = async times => {
    gachaNumber.value = times
    const cost = getGachaCost(times)
    if (!canAfford(times)) return message.error('灵石不足！')
    
    if (gachaType.value !== 'equipment' && playerStore.items.filter(i => i.type === 'pet').length >= 100) {
      return message.error('灵宠背包已满')
    }

    try {
      isDrawing.value = true
      isShaking.value = true
      await new Promise(resolve => setTimeout(resolve, 800))
      isShaking.value = false
      isOpening.value = true
      await new Promise(resolve => setTimeout(resolve, 500))
      
      const result = await drawGacha({
        gachaType: gachaType.value,
        times,
        wishlistEnabled: playerStore.wishlistEnabled,
        selectedWishEquipQuality: playerStore.selectedWishEquipQuality,
        selectedWishPetRarity: playerStore.selectedWishPetRarity,
        autoSellQualities: playerStore.autoSellQualities,
        autoReleaseRarities: playerStore.autoReleaseRarities
      })

      if (result?.snapshot) playerStore.applyServerSnapshot(result.snapshot)
      
      gachaResult.value = result?.results || []
      currentPage.value = 1
      isOpening.value = false
      showResult.value = true
      
      if (result?.autoSoldCount) message.success(`自动分解了 ${result.autoSoldCount} 件装备`)
    } catch (error) {
      message.error(error?.message || '抽奖失败')
      isOpening.value = false
    } finally {
      isDrawing.value = false
    }
  }

  const isWishItem = item => {
    if (!playerStore.wishlistEnabled) return false
    return (item.quality === playerStore.selectedWishEquipQuality) || (item.rarity === playerStore.selectedWishPetRarity)
  }

  const getItemQualityClass = item => `q-${item.quality || item.rarity || 'none'}`
  const getItemQualityName = item => {
    const q = item.quality || item.rarity
    return (equipmentQualities[q] || petRarities[q])?.name || '普通'
  }
  const getItemTypeName = item => item.type === 'pet' ? '灵宠' : '装备'

  const filteredResults = computed(() => {
    return gachaResult.value.filter(item => {
      if (item.type === 'none') return false
      if (gachaType.value === 'equipment' && selectedQuality.value && item.quality !== selectedQuality.value) return false
      if (gachaType.value === 'pet' && selectedRarity.value && item.rarity !== selectedRarity.value) return false
      return true
    })
  })

  const currentPageResults = computed(() => {
    const start = (currentPage.value - 1) * pageSize.value
    return filteredResults.value.slice(start, start + pageSize.value)
  })

  const totalPages = computed(() => Math.ceil(filteredResults.value.length / pageSize.value))

  const equipmentQualityOptions = computed(() => Object.entries(equipmentQualities).map(([k, v]) => ({ label: v.name, value: k })))
  const petRarityOptions = computed(() => Object.entries(petRarities).map(([k, v]) => ({ label: v.name, value: k })))

  const handleAutoSellChange = v => playerStore.autoSellQualities = v
  const handleAutoReleaseChange = v => playerStore.autoReleaseRarities = v

  const getAllPoolProbabilities = () => {
    const totalEquipProb = 0.5
    const totalPetProb = 0.5
    const adjustedEquipProbs = {}
    Object.entries(getEquipProbabilities).forEach(([q, p]) => adjustedEquipProbs[q] = p * totalEquipProb)
    return { equipment: adjustedEquipProbs }
  }
</script>

<style scoped>
.gacha-page {
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

.resource-chip {
  background: var(--panel-bg);
  border: 1px solid var(--panel-border);
  padding: 8px 16px;
  border-radius: 99px;
  display: flex;
  align-items: center;
  gap: 8px;
  font-weight: bold;
  color: var(--accent-primary);
}

.gacha-layout {
  display: grid;
  grid-template-columns: 280px 1fr;
  gap: 24px;
  flex: 1;
}

/* 卡池选择 */
.pool-cards {
  display: flex;
  flex-direction: column;
  gap: 16px;
  margin-bottom: 32px;
}

.pool-card {
  position: relative;
  background: var(--panel-bg);
  border: 1px solid var(--panel-border);
  border-radius: 20px;
  padding: 20px;
  cursor: pointer;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  display: flex;
  align-items: center;
  gap: 16px;
  overflow: hidden;
}

.pool-card:hover { transform: translateX(8px); border-color: var(--accent-primary); }
.pool-card.is-active { border-color: var(--accent-primary); background: var(--accent-muted); }

.pool-icon {
  width: 56px;
  height: 56px;
  background: rgba(0,0,0,0.05);
  border-radius: 14px;
  display: grid;
  place-items: center;
  color: var(--accent-primary);
}

.pool-name { font-size: 18px; font-family: var(--font-display); font-weight: bold; }
.pool-cost { font-size: 12px; opacity: 0.6; }

.active-glow {
  position: absolute;
  inset: 0;
  background: radial-gradient(circle at center, var(--accent-primary) 0%, transparent 70%);
  opacity: 0;
  transition: opacity 0.3s;
}
.is-active .active-glow { opacity: 0.1; }

.utility-actions { display: flex; flex-direction: column; gap: 8px; }

/* 舞台展示区 */
.gacha-stage {
  position: relative;
  background: var(--panel-bg);
  border: 1px solid var(--panel-border);
  border-radius: 32px;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 40px;
}

.stage-bg {
  position: absolute;
  inset: 0;
  background: radial-gradient(circle at center, #1a2533 0%, #0f172a 100%);
  z-index: 0;
}

.star-grid {
  position: absolute;
  inset: 0;
  background-image: radial-gradient(white 1px, transparent 1px);
  background-size: 40px 40px;
  opacity: 0.1;
}

.portal-rings {
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  width: 400px;
  height: 400px;
}

.ring {
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  border: 1px solid rgba(47, 107, 109, 0.3);
  border-radius: 50%;
}
.ring.outer { width: 100%; height: 100%; border-style: dashed; animation: rotate 20s linear infinite; }
.ring.middle { width: 75%; height: 75%; border-width: 2px; animation: rotate 15s linear infinite reverse; }
.ring.inner { width: 50%; height: 50%; border-style: dotted; animation: rotate 10s linear infinite; }

.is-spinning .ring { border-color: var(--accent-primary); border-width: 3px; filter: blur(1px); }

@keyframes rotate { from { transform: translate(-50%, -50%) rotate(0deg); } to { transform: translate(-50%, -50%) rotate(360deg); } }

.portal-center { position: relative; z-index: 2; margin-bottom: 60px; }

.gacha-orb {
  width: 160px;
  height: 160px;
  background: linear-gradient(145deg, var(--accent-primary), #134e4a);
  border-radius: 50%;
  display: grid;
  place-items: center;
  box-shadow: 0 0 40px var(--accent-muted), inset 0 0 20px rgba(255,255,255,0.2);
  position: relative;
  transition: all 0.5s ease;
}

.orb-glow {
  position: absolute;
  inset: -20px;
  background: radial-gradient(circle, var(--accent-primary) 0%, transparent 70%);
  opacity: 0.4;
  animation: pulse 2s infinite;
}

@keyframes pulse { 0%, 100% { transform: scale(1); opacity: 0.4; } 50% { transform: scale(1.2); opacity: 0.6; } }

.is-shaking { animation: shake 0.1s infinite; }
@keyframes shake { 0%, 100% { transform: translate(0,0); } 25% { transform: translate(4px,4px); } 75% { transform: translate(-4px,-4px); } }

.is-opening { transform: scale(0); opacity: 0; filter: blur(20px); }

.gacha-actions { position: relative; z-index: 2; width: 100%; }
.action-group { display: grid; grid-template-columns: repeat(2, 1fr); gap: 16px; width: 100%; max-width: 500px; margin: 0 auto; }
.draw-btn { height: 60px !important; }
.cost-tag { font-size: 11px; opacity: 0.7; margin-left: 8px; }
.wishlist-hint { margin-top: 16px; font-size: 12px; color: #f0a020; text-align: center; }

/* 结果网格 */
.result-container { padding: 10px; }
.result-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 20px; }
.result-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(160px, 1fr));
  gap: 12px;
  padding: 10px;
}

.result-card {
  position: relative;
  background: var(--panel-bg);
  border: 1px solid var(--panel-border);
  border-radius: 16px;
  padding: 20px;
  text-align: center;
  overflow: hidden;
  animation: pop-in 0.4s cubic-bezier(0.175, 0.885, 0.32, 1.275) both;
  animation-delay: var(--delay);
}

@keyframes pop-in { from { transform: scale(0.5); opacity: 0; } to { transform: scale(1); opacity: 1; } }

.item-rarity-bg { position: absolute; inset: 0; opacity: 0.05; transition: opacity 0.3s; }
.q-legendary .item-rarity-bg { background: #f0a020; opacity: 0.1; }
.q-mythic .item-rarity-bg { background: #d03050; opacity: 0.1; }

.item-name { font-weight: bold; font-size: 15px; margin-bottom: 4px; }
.item-type { font-size: 11px; opacity: 0.6; }
.item-quality { font-size: 12px; font-weight: bold; margin-top: 8px; }

.q-common { border-color: #94a3b8; }
.q-uncommon { border-color: #18a058; }
.q-rare { border-color: #2080f0; }
.q-epic { border-color: #a042ff; }
.q-legendary { border-color: #f0a020; box-shadow: 0 0 15px rgba(240, 160, 32, 0.2); }
.q-mythic { border-color: #d03050; box-shadow: 0 0 20px rgba(208, 48, 80, 0.3); }

.is-wish { border-style: dashed; border-width: 2px; }
.wish-star { position: absolute; top: 4px; right: 8px; color: #f0a020; font-size: 18px; }

.result-footer { margin-top: 24px; display: flex; justify-content: center; }

@media (max-width: 1080px) {
  .gacha-layout { grid-template-columns: 1fr; }
  .pool-cards { flex-direction: row; overflow-x: auto; padding-bottom: 10px; }
  .pool-card { min-width: 200px; }
  .gacha-stage { min-height: 500px; }
}

@media (max-width: 768px) {
  .portal-rings { width: 280px; height: 280px; }
  .action-group { grid-template-columns: 1fr; }
  .draw-btn { height: 50px !important; }
}
</style>
