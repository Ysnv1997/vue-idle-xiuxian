<template>
  <div class="page-view alchemy-page">
    <!-- 顶部标题区 -->
    <header class="page-head">
      <div class="head-main">
        <p class="page-eyebrow">夺天地造化 · 炼日月精华</p>
        <h2 class="page-title">丹鼎炼制</h2>
      </div>
    </header>

    <div class="alchemy-layout">
      <!-- 左侧：丹方秘籍（书卷风格） -->
      <aside class="recipe-section">
        <div class="section-title">已掌握丹方</div>
        <n-scrollbar class="recipe-scroll">
          <div v-if="unlockedRecipes.length > 0" class="recipe-list">
            <div 
              v-for="recipe in unlockedRecipes" 
              :key="recipe.id"
              class="recipe-item"
              :class="{ 'is-active': selectedRecipe?.id === recipe.id }"
              @click="selectRecipe(recipe)"
            >
              <div class="recipe-mark">{{ pillGrades[recipe.grade].name[0] }}</div>
              <div class="recipe-info">
                <div class="recipe-name">{{ recipe.name }}</div>
                <div class="recipe-meta">{{ pillTypes[recipe.type].name }}</div>
              </div>
              <div class="active-indicator" v-if="selectedRecipe?.id === recipe.id"></div>
            </div>
          </div>
          <n-empty v-else description="暂未掌握丹方" style="padding: 40px 0" />
        </n-scrollbar>
      </aside>

      <!-- 中间：炼丹炉（核心视觉中心） -->
      <main class="furnace-section">
        <div class="furnace-container" :class="{ 'is-crafting': isSubmitting }">
          <!-- 炼丹炉视觉实体 -->
          <div class="furnace-visual">
            <div class="furnace-body">
              <div class="bagua-pattern"></div>
              <div class="furnace-window">
                <div class="fire-fx" v-if="isSubmitting"></div>
                <div class="glow-fx" v-if="showSuccessFx"></div>
              </div>
            </div>
            <div class="furnace-legs">
              <div class="leg"></div>
              <div class="leg"></div>
              <div class="leg"></div>
            </div>
            <!-- 烟雾特效 -->
            <div class="smoke-fx" v-if="isSubmitting">
              <div class="smoke"></div>
              <div class="smoke"></div>
            </div>
          </div>

          <!-- 炼制按钮 -->
          <div class="craft-action-area">
            <div v-if="selectedRecipe" class="craft-odds">
              预计成功率：<strong>{{ (currentEffect?.successRate * 100).toFixed(1) }}%</strong>
            </div>
            <n-button
              class="furnace-craft-btn"
              type="primary"
              size="large"
              round
              :disabled="!selectedRecipe || !checkMaterials(selectedRecipe) || isSubmitting"
              :loading="isSubmitting"
              @click="craftPill"
            >
              <template #icon><n-icon><FlameOutline /></n-icon></template>
              {{ !selectedRecipe ? '请选择丹方' : !checkMaterials(selectedRecipe) ? '材料不足' : '引火炼丹' }}
            </n-button>
          </div>
        </div>
      </main>

      <!-- 右侧：材料与效果详情 -->
      <aside class="detail-section">
        <template v-if="selectedRecipe">
          <!-- 材料网格 -->
          <div class="stats-panel materials-panel">
            <div class="panel-title">所需材料</div>
            <div class="material-grid">
              <div v-for="material in selectedRecipe.materials" :key="material.herb" class="material-item">
                <div 
                  class="material-icon" 
                  :class="{ 'is-insufficient': getOwnedCount(material.herb) < material.count }"
                >
                  <div class="herb-initial">{{ getHerbName(material.herb)[0] }}</div>
                  <div class="count-tag">{{ getMaterialStatus(material) }}</div>
                </div>
                <div class="material-name">{{ getHerbName(material.herb) }}</div>
              </div>
            </div>
          </div>

          <!-- 效果预览 -->
          <div class="stats-panel effects-panel">
            <div class="panel-title">丹药成效</div>
            <div class="effect-details">
              <p class="recipe-desc">{{ selectedRecipe.description }}</p>
              <div class="effect-list">
                <div class="effect-row">
                  <span class="e-label">药效强度</span>
                  <span class="e-value">+{{ (currentEffect.value * 100).toFixed(1) }}%</span>
                </div>
                <div class="effect-row">
                  <span class="e-label">药力持续</span>
                  <span class="e-value">{{ Math.floor(currentEffect.duration / 60) }} 分钟</span>
                </div>
                <div class="effect-row">
                  <span class="e-label">成丹品阶</span>
                  <span class="e-value text-primary">{{ pillGrades[selectedRecipe.grade].name }}</span>
                </div>
              </div>
            </div>
          </div>
        </template>
        <div v-else class="select-hint-panel">
          <n-icon size="48" color="var(--panel-border)"><BookOutline /></n-icon>
          <p>请在左侧翻阅并选择要炼制的丹方</p>
        </div>
      </aside>
    </div>

    <!-- 炼丹日志区 -->
    <footer class="alchemy-footer">
      <div class="section-head">
        <span class="section-title">炼丹心得</span>
      </div>
      <log-panel ref="logRef" title="" />
    </footer>
  </div>
</template>

<script setup>
  import { ref, computed } from 'vue'
  import { FlameOutline, BookOutline } from '@vicons/ionicons5'
  import { usePlayerStore } from '../stores/player'
  import { pillRecipes, pillGrades, pillTypes, calculatePillEffect } from '../plugins/pills'
  import { herbs } from '../plugins/herbs'
  import LogPanel from '../components/LogPanel.vue'
  import { craftAlchemyPill } from '../api/modules/game'

  const playerStore = usePlayerStore()
  const logRef = ref(null)
  const isSubmitting = ref(false)
  const showSuccessFx = ref(false)

  // 当前选择的丹方
  const selectedRecipe = ref(null)

  // 已解锁的丹方列表
  const unlockedRecipes = computed(() => {
    return pillRecipes.filter(recipe => playerStore.pillRecipes.includes(recipe.id))
  })

  // 选择丹方
  const selectRecipe = recipe => {
    selectedRecipe.value = recipe
  }

  // 获取拥有数量
  const getOwnedCount = herbId => {
    return playerStore.herbs.filter(h => h.id === herbId).length
  }

  // 检查材料是否充足
  const checkMaterials = recipe => {
    if (!recipe) return false
    return recipe.materials.every(material => {
      return getOwnedCount(material.herb) >= material.count
    })
  }

  // 获取材料状态文本
  const getMaterialStatus = material => {
    return `${getOwnedCount(material.herb)}/${material.count}`
  }

  // 获取灵草名称
  const getHerbName = herbId => {
    const herb = herbs.find(h => h.id === herbId)
    return herb ? herb.name : herbId
  }

  // 计算当前效果
  const currentEffect = computed(() => {
    if (!selectedRecipe.value) return null
    return calculatePillEffect(selectedRecipe.value, playerStore.level)
  })

  // 炼制丹药
  const craftPill = async () => {
    if (!selectedRecipe.value) return
    try {
      isSubmitting.value = true
      const result = await craftAlchemyPill(selectedRecipe.value.id)
      if (result?.snapshot) {
        playerStore.applyServerSnapshot(result.snapshot)
      }
      if (result?.success) {
        logRef.value?.addLog('success', result.message || '丹成！品质上佳。')
        showSuccessFx.value = true
        setTimeout(() => showSuccessFx.value = false, 2000)
      } else {
        logRef.value?.addLog('error', `炸炉了：${result?.message || '药力不稳'}`)
      }
    } catch (error) {
      logRef.value?.addLog('error', `炼制失败：${error?.message || '未知错误'}`)
    } finally {
      isSubmitting.value = false
    }
  }
</script>

<style scoped>
.alchemy-page {
  display: flex;
  flex-direction: column;
  height: 100%;
  max-width: 1200px;
  margin: 0 auto;
}

.alchemy-layout {
  display: grid;
  grid-template-columns: 260px 1fr 320px;
  gap: 20px;
  margin-top: 20px;
  flex: 1;
}

.section-title {
  font-size: 14px;
  font-weight: bold;
  color: var(--ink-sub);
  margin-bottom: 12px;
  opacity: 0.8;
}

/* 丹方列表 */
.recipe-section {
  background: var(--panel-bg);
  border: 1px solid var(--panel-border);
  border-radius: 20px;
  padding: 16px;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.recipe-scroll { flex: 1; }

.recipe-list { display: flex; flex-direction: column; gap: 10px; }

.recipe-item {
  padding: 14px;
  background: rgba(0,0,0,0.03);
  border: 1px solid var(--panel-border);
  border-radius: 12px;
  cursor: pointer;
  transition: all 0.3s ease;
  display: flex;
  align-items: center;
  gap: 12px;
  position: relative;
}

.recipe-item:hover { border-color: var(--accent-primary); background: var(--accent-muted); }
.recipe-item.is-active { border-color: var(--accent-primary); background: var(--accent-muted); }

.recipe-mark {
  width: 32px;
  height: 32px;
  background: var(--accent-primary);
  color: white;
  border-radius: 8px;
  display: grid;
  place-items: center;
  font-family: var(--font-display);
  font-size: 18px;
}

.recipe-name { font-size: 15px; font-weight: bold; }
.recipe-meta { font-size: 11px; color: var(--ink-sub); }

.active-indicator {
  position: absolute;
  right: 10px;
  width: 6px;
  height: 6px;
  background: var(--accent-primary);
  border-radius: 50%;
  box-shadow: 0 0 8px var(--accent-primary);
}

/* 炼丹炉视觉区 */
.furnace-section {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 40px;
}

.furnace-container {
  width: 100%;
  max-width: 400px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 40px;
}

.furnace-visual {
  position: relative;
  width: 240px;
  height: 240px;
}

.furnace-body {
  width: 100%;
  height: 80%;
  background: linear-gradient(145deg, #4a4a4a, #2d2d2d);
  border: 4px solid #1a1a1a;
  border-radius: 50% 50% 40% 40%;
  position: relative;
  box-shadow: inset 0 -10px 20px rgba(0,0,0,0.5), 0 20px 40px rgba(0,0,0,0.2);
  z-index: 2;
}

.bagua-pattern {
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  width: 120px;
  height: 120px;
  border: 1px solid rgba(255,255,255,0.1);
  border-radius: 50%;
  opacity: 0.3;
}

.furnace-window {
  position: absolute;
  top: 40%;
  left: 50%;
  transform: translate(-50%, -50%);
  width: 60px;
  height: 60px;
  background: #1a1a1a;
  border: 3px solid #333;
  border-radius: 50%;
  overflow: hidden;
}

.fire-fx {
  position: absolute;
  inset: 0;
  background: radial-gradient(circle, #ff5722, #e91e63, transparent);
  animation: fire-flicker 0.2s infinite alternate;
}

@keyframes fire-flicker {
  from { opacity: 0.6; transform: scale(0.9); }
  to { opacity: 1; transform: scale(1.1); }
}

.glow-fx {
  position: absolute;
  inset: 0;
  background: radial-gradient(circle, #4caf50, transparent 70%);
  animation: success-glow 2s ease-out;
}

@keyframes success-glow {
  0% { opacity: 0; transform: scale(0.5); }
  50% { opacity: 1; transform: scale(1.5); }
  100% { opacity: 0; transform: scale(2); }
}

.furnace-legs {
  display: flex;
  justify-content: space-between;
  width: 180px;
  margin-top: -10px;
}

.leg {
  width: 30px;
  height: 40px;
  background: #2d2d2d;
  border-radius: 0 0 10px 10px;
  border: 2px solid #1a1a1a;
}

.is-crafting { animation: shake 0.5s infinite; }
@keyframes shake {
  0%, 100% { transform: translate(0, 0) rotate(0deg); }
  25% { transform: translate(2px, 2px) rotate(1deg); }
  75% { transform: translate(-2px, 2px) rotate(-1deg); }
}

.smoke-fx {
  position: absolute;
  top: -40px;
  left: 50%;
  transform: translateX(-50%);
}

.smoke {
  position: absolute;
  width: 20px;
  height: 20px;
  background: rgba(200,200,200,0.4);
  border-radius: 50%;
  filter: blur(10px);
  animation: rise 2s infinite ease-out;
}
.smoke:nth-child(2) { animation-delay: 1s; left: 20px; }

@keyframes rise {
  0% { transform: translateY(0) scale(1); opacity: 0; }
  20% { opacity: 1; }
  100% { transform: translateY(-100px) scale(3); opacity: 0; }
}

.craft-action-area { text-align: center; }
.craft-odds { font-size: 13px; color: var(--ink-sub); margin-bottom: 12px; }
.furnace-craft-btn { width: 240px; height: 52px; font-size: 18px; font-family: var(--font-display); }

/* 详情面板 */
.detail-section { display: flex; flex-direction: column; gap: 20px; }

.stats-panel {
  background: var(--panel-bg);
  border: 1px solid var(--panel-border);
  border-radius: 20px;
  padding: 20px;
}

.panel-title { font-size: 13px; font-weight: bold; margin-bottom: 16px; color: var(--accent-primary); }

.material-grid { display: grid; grid-template-columns: repeat(2, 1fr); gap: 12px; }

.material-item { display: flex; flex-direction: column; align-items: center; gap: 6px; }

.material-icon {
  width: 56px;
  height: 56px;
  background: rgba(0,0,0,0.03);
  border: 1px solid var(--panel-border);
  border-radius: 12px;
  display: grid;
  place-items: center;
  position: relative;
}

.material-icon.is-insufficient { border-color: #d03050; background: rgba(208, 48, 80, 0.05); }

.herb-initial { font-family: var(--font-display); font-size: 24px; opacity: 0.6; }
.count-tag { position: absolute; bottom: -6px; background: var(--panel-bg); border: 1px solid var(--panel-border); border-radius: 8px; font-size: 10px; padding: 0 6px; font-weight: bold; }

.material-name { font-size: 12px; color: var(--ink-sub); text-align: center; }

.recipe-desc { font-size: 13px; color: var(--ink-sub); line-height: 1.6; margin-bottom: 16px; font-style: italic; }

.effect-list { display: flex; flex-direction: column; gap: 10px; }
.effect-row { display: flex; justify-content: space-between; font-size: 13px; }
.e-label { color: var(--ink-sub); }
.e-value { font-weight: bold; }

.select-hint-panel { flex: 1; display: flex; flex-direction: column; align-items: center; justify-content: center; text-align: center; opacity: 0.5; gap: 16px; }

.alchemy-footer {
  margin-top: 32px;
  background: var(--panel-bg);
  border: 1px solid var(--panel-border);
  border-radius: 20px;
  padding: 20px;
}

@media (max-width: 1080px) {
  .alchemy-layout { grid-template-columns: 1fr; }
  .recipe-section { height: 300px; }
  .furnace-section { order: -1; }
}

@media (max-width: 768px) {
  .furnace-visual { width: 180px; height: 180px; }
  .material-grid { grid-template-columns: repeat(3, 1fr); }
}
</style>
