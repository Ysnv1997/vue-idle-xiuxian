<template>
  <div class="page-view cultivation-page">
    <!-- 顶部标题区 -->
    <header class="page-head">
      <div class="head-main">
        <p class="page-eyebrow">历练与参悟</p>
        <h2 class="page-title">境界修炼</h2>
      </div>
      <div class="head-action">
        <n-button 
          type="info" 
          secondary 
          round
          :loading="isBreakthroughSubmitting"
          :disabled="!canBreakthrough"
          @click="handleBreakthrough"
        >
          <template #icon><n-icon><FlashOutline /></n-icon></template>
          手动突破
        </n-button>
      </div>
    </header>

    <!-- 核心交互区 -->
    <main class="cultivation-content">
      <n-tabs v-model:value="activeCultivationTab" type="segment" animated class="custom-tabs">
        <!-- 静室打坐 Tab -->
        <n-tab-pane name="meditation" tab="静室打坐">
          <div class="meditation-container" :class="{ 'is-active': isMeditating }">
            <div class="spirit-focus-area">
              <!-- 灵气汇聚动效 -->
              <div v-if="isMeditating" class="spirit-particles">
                <div v-for="i in 8" :key="i" class="particle"></div>
              </div>
              
              <div class="meditation-circle">
                <n-progress
                  type="circle"
                  :percentage="meditationSpiritPercent"
                  :stroke-width="6"
                  :color="isMeditating ? '#18a058' : '#9ab0c6'"
                >
                  <div class="circle-content">
                    <span class="label">{{ meditationStateLabel }}</span>
                    <span class="value">{{ meditationCurrentSpiritDisplay }}</span>
                    <span class="cap">上限 {{ meditationSpiritCapDisplay }}</span>
                  </div>
                </n-progress>
              </div>
            </div>

            <div class="meditation-stats">
              <div class="stat-box">
                <span class="s-label">恢复速度</span>
                <span class="s-value">{{ meditationCurrentRateDisplay }}/s</span>
              </div>
              <div class="stat-box">
                <span class="s-label">本轮收益</span>
                <span class="s-value text-success">+{{ meditationTotalSpiritGainDisplay }}</span>
              </div>
              <div class="stat-box">
                <span class="s-label">充满预计</span>
                <span class="s-value">{{ meditationFillEstimateLabel }}</span>
              </div>
            </div>

            <div class="action-bar">
              <n-button
                v-if="!isMeditating"
                type="primary"
                size="large"
                block
                round
                :loading="isMeditationSubmitting"
                :disabled="isHuntingRunning"
                @click="startMeditation"
              >
                开启灵脉聚气
              </n-button>
              <n-button
                v-else
                type="warning"
                size="large"
                block
                round
                ghost
                :loading="isMeditationSubmitting"
                @click="stopMeditation()"
              >
                结束打坐
              </n-button>
            </div>
          </div>
        </n-tab-pane>

        <!-- 地图刷怪 Tab -->
        <n-tab-pane name="hunting" tab="外门历练">
          <div class="hunting-container">
            <!-- 地图选择区域 - 改造为滚动卡片 -->
            <div class="map-selector" v-if="!isHuntingRunning">
              <div class="selector-header">选择历练之地</div>
              <div class="map-grid">
                <div 
                  v-for="map in huntingMaps" 
                  :key="map.id"
                  class="map-card"
                  :class="{ 
                    'is-selected': selectedHuntingMapId === map.id,
                    'is-locked': playerStore.level < map.minLevel 
                  }"
                  @click="playerStore.level >= map.minLevel && (selectedHuntingMapId = map.id)"
                >
                  <div class="map-level">Lv.{{ map.minLevel }}</div>
                  <div class="map-name">{{ map.name }}</div>
                  <div class="map-meta">
                    <span>{{ map.monsters?.length || 0 }} 种妖兽</span>
                  </div>
                  <div v-if="playerStore.level < map.minLevel" class="lock-overlay">
                    <n-icon size="24"><LockClosedOutline /></n-icon>
                  </div>
                </div>
              </div>
            </div>

            <!-- 战斗详情区域 -->
            <div v-if="selectedHuntingMap" class="hunting-display" :class="{ 'is-running': isHuntingRunning }">
              <div class="hunting-hero">
                <div class="map-info-header">
                  <h3>{{ selectedHuntingMap.name }}</h3>
                  <p>{{ selectedHuntingMap.description }}</p>
                </div>

                <div class="combat-hud">
                  <div class="combat-unit player">
                    <div class="unit-label">修士</div>
                    <n-progress
                      type="line"
                      :percentage="huntingHpPercent"
                      :show-indicator="false"
                      processing
                      color="#e88080"
                      class="hp-bar"
                    />
                    <div class="hp-text">{{ huntingCurrentHpDisplay }} / {{ huntingMaxHpDisplay }}</div>
                  </div>
                  
                  <div class="combat-vs">VS</div>

                  <div class="combat-unit monster">
                    <div class="unit-label">{{ huntingRunStatus.state === 'reviving' ? '复活中' : '妖兽' }}</div>
                    <n-progress
                      type="line"
                      :percentage="huntingProgressPercent"
                      :show-indicator="false"
                      :color="huntingRunStatus.state === 'reviving' ? '#f0a020' : '#18a058'"
                      class="hp-bar"
                    />
                    <div class="hp-text">{{ huntingProgressDisplayText }}</div>
                  </div>
                </div>

                <div class="hunting-yield">
                  <n-grid :cols="3">
                    <n-gi>
                      <div class="yield-item">
                        <div class="y-label">累计击杀</div>
                        <div class="y-value">{{ huntingKillCount }}</div>
                      </div>
                    </n-gi>
                    <n-gi>
                      <div class="yield-item">
                        <div class="y-label">获得修为</div>
                        <div class="y-value text-primary">{{ huntingTotalCultivationGainDisplay }}</div>
                      </div>
                    </n-gi>
                    <n-gi>
                      <div class="yield-item">
                        <div class="y-label">消耗灵力</div>
                        <div class="y-value">{{ huntingTotalSpiritCostDisplay }}</div>
                      </div>
                    </n-gi>
                  </n-grid>
                </div>

                <div class="action-bar">
                  <n-button
                    v-if="!isHuntingRunning"
                    type="warning"
                    size="large"
                    block
                    round
                    :disabled="!canStartHunting"
                    :loading="isHuntingSubmitting"
                    @click="startHunting"
                  >
                    前往历练
                  </n-button>
                  <n-button 
                    v-else 
                    type="error" 
                    size="large" 
                    block 
                    round
                    ghost
                    :loading="isHuntingSubmitting" 
                    @click="stopHunting()"
                  >
                    撤离地图
                  </n-button>
                </div>
              </div>
            </div>
          </div>
        </n-tab-pane>
      </n-tabs>

      <!-- 日志面板优化 -->
      <footer class="cultivation-logs">
        <log-panel ref="logRef" title="历练传书" />
      </footer>
    </main>
  </div>
</template>

<script setup>
  import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
  import { NIcon } from 'naive-ui'
  import { 
    BookOutline, 
    FlashOutline, 
    LockClosedOutline, 
    PlanetOutline,
    FlameOutline
  } from '@vicons/ionicons5'
  import LogPanel from '../components/LogPanel.vue'
  import { useGameRealtimeStore } from '../stores/game-realtime'
  import { usePlayerStore } from '../stores/player'
  import { formatScaledGrowth } from '../utils/growth-display'
  import {
    breakthrough as breakthroughApi,
    listHuntingMaps as listHuntingMapsApi,
    startHuntingRun as startHuntingRunApi,
    startMeditation as startMeditationApi,
    stopHuntingRun as stopHuntingRunApi,
    stopMeditation as stopMeditationApi
  } from '../api/modules/game'

  const playerStore = usePlayerStore()
  const gameRealtimeStore = useGameRealtimeStore()
  const logRef = ref(null)

  const activeCultivationTab = ref('meditation')
  const huntingProgressRefreshIntervalMs = 200

  // 基础状态逻辑保持原样，确保功能不失效
  const isMeditationSubmitting = ref(false)
  const isBreakthroughSubmitting = ref(false)
  const meditationLastSeenLogSeq = ref(0)
  const meditationLogSeqInitialized = ref(false)
  const meditationRunStatus = ref({
    isActive: false,
    state: 'stopped',
    currentSpirit: 0,
    spiritCap: 0,
    currentRate: 0,
    totalSpiritGain: 0,
    startedAt: 0,
    lastLogSeq: 0,
    lastLogMessage: ''
  })

  const isLoadingHuntingMaps = ref(false)
  const isLoadingHuntingStatus = ref(false)
  const isHuntingSubmitting = ref(false)
  const huntingMaps = ref([])
  const selectedHuntingMapId = ref('')
  const huntingProgressTimer = ref(null)
  const huntingProgressNow = ref(Date.now())
  const huntingStatusReceivedAt = ref(0)
  const huntingLastSeenLogSeq = ref(0)
  const huntingLogSeqInitialized = ref(false)
  const huntingRunStatus = ref({
    isActive: false,
    state: 'stopped',
    mapId: '',
    mapName: '',
    currentHp: 0,
    maxHp: 0,
    killCount: 0,
    totalSpiritCost: 0,
    totalCultivationGain: 0,
    progressPercent: 0,
    progressLabel: '',
    progressRemainingMs: 0,
    lastLogSeq: 0,
    lastLogMessage: '',
    reviveUntil: 0
  })

  // ---------------- 数据格式化方法 ----------------
  const showMessage = (type, content) => {
    if (!content) return
    return logRef.value?.addLog(type, content)
  }

  const toFiniteNumber = (value, fallback = 0) => {
    const num = Number(value)
    return Number.isFinite(num) ? num : fallback
  }

  const formatGrowthDecimal = (value, digits = 1) => {
    return formatScaledGrowth(value, {
      minimumFractionDigits: digits,
      maximumFractionDigits: digits
    })
  }

  const formatDuration = seconds => {
    const totalSeconds = Math.max(0, Math.ceil(toFiniteNumber(seconds, 0)))
    if (totalSeconds <= 0) return '充满'
    const hours = Math.floor(totalSeconds / 3600)
    const minutes = Math.floor((totalSeconds % 3600) / 60)
    const remainSeconds = totalSeconds % 60
    if (hours > 0) return `${hours}时${minutes}分`
    if (minutes > 0) return `${minutes}分${remainSeconds}秒`
    return `${remainSeconds}秒`
  }

  // ---------------- 计算属性逻辑 (保持原逻辑) ----------------
  const isMeditating = computed(() => Boolean(meditationRunStatus.value?.isActive))
  const meditationCurrentSpirit = computed(() => toFiniteNumber(meditationRunStatus.value?.currentSpirit, playerStore.spirit))
  const meditationSpiritCap = computed(() => Math.max(0, toFiniteNumber(meditationRunStatus.value?.spiritCap, 0)))
  const meditationCurrentRate = computed(() => Math.max(0, toFiniteNumber(meditationRunStatus.value?.currentRate, 0)))
  const meditationTotalSpiritGain = computed(() => Math.max(0, toFiniteNumber(meditationRunStatus.value?.totalSpiritGain, 0)))
  const meditationStateLabel = computed(() => {
    switch (meditationRunStatus.value?.state) {
      case 'running': return '聚气中'
      case 'full': return '圆满'
      default: return '待机'
    }
  })
  const meditationSpiritPercent = computed(() => {
    const spiritCap = meditationSpiritCap.value
    if (spiritCap <= 0) return 0
    return Math.max(0, Math.min(100, (meditationCurrentSpirit.value / spiritCap) * 100))
  })
  const meditationFillEstimateLabel = computed(() => {
    if (meditationCurrentSpirit.value >= meditationSpiritCap.value) return '已满'
    if (meditationCurrentRate.value <= 0) return '--'
    return formatDuration((meditationSpiritCap.value - meditationCurrentSpirit.value) / meditationCurrentRate.value)
  })

  const meditationCurrentSpiritDisplay = computed(() => formatGrowthDecimal(meditationCurrentSpirit.value, 1))
  const meditationSpiritCapDisplay = computed(() => formatGrowthDecimal(meditationSpiritCap.value, 1))
  const meditationCurrentRateDisplay = computed(() => meditationCurrentRate.value.toFixed(2))
  const meditationTotalSpiritGainDisplay = computed(() => formatGrowthDecimal(meditationTotalSpiritGain.value, 1))

  const canBreakthrough = computed(() => {
    return playerStore.cultivation >= playerStore.maxCultivation && !isHuntingRunning.value && !isBreakthroughSubmitting.value
  })

  // Hunting Logic
  const isHuntingRunning = computed(() => Boolean(huntingRunStatus.value?.isActive))
  const selectedHuntingMap = computed(() => huntingMaps.value.find(map => map.id === selectedHuntingMapId.value) || null)
  const huntingCurrentHpDisplay = computed(() => toFiniteNumber(huntingRunStatus.value?.currentHp, 0).toFixed(0))
  const huntingMaxHpDisplay = computed(() => toFiniteNumber(huntingRunStatus.value?.maxHp, 0).toFixed(0))
  const huntingHpPercent = computed(() => {
    const max = toFiniteNumber(huntingRunStatus.value?.maxHp, 0)
    if (max <= 0) return 100
    return Math.floor((toFiniteNumber(huntingRunStatus.value?.currentHp, 0) / max) * 100)
  })
  const huntingKillCount = computed(() => Math.max(0, Math.floor(toFiniteNumber(huntingRunStatus.value?.killCount, 0))))
  const huntingTotalSpiritCostDisplay = computed(() => formatScaledGrowth(huntingRunStatus.value?.totalSpiritCost))
  const huntingTotalCultivationGainDisplay = computed(() => formatScaledGrowth(huntingRunStatus.value?.totalCultivationGain))

  const huntingProgressPercent = computed(() => {
    if (!isHuntingRunning.value) return 0
    const basePercent = Math.max(0, Math.min(100, toFiniteNumber(huntingRunStatus.value?.progressPercent, 0)))
    const remainingMs = Math.max(0, Math.floor(toFiniteNumber(huntingRunStatus.value?.progressRemainingMs, 0)))
    if (remainingMs <= 0 || huntingStatusReceivedAt.value <= 0) return basePercent
    const elapsedMs = Math.max(0, huntingProgressNow.value - huntingStatusReceivedAt.value)
    return Math.max(0, Math.min(100, basePercent + (elapsedMs * 100) / (remainingMs + elapsedMs)))
  })

  const huntingProgressDisplayText = computed(() => {
    const state = huntingRunStatus.value?.state
    if (state === 'reviving') return '魂魄重塑中...'
    return '击杀妖兽中...'
  })

  const canStartHunting = computed(() => {
    if (!selectedHuntingMap.value || playerStore.level < selectedHuntingMap.value.minLevel) return false
    return !isHuntingRunning.value && !isHuntingSubmitting.value
  })

  // ---------------- API 调用方法 (保持功能一致) ----------------
  const startMeditation = async () => {
    if (isMeditationSubmitting.value) return
    try {
      isMeditationSubmitting.value = true
      const result = await startMeditationApi()
      if (result?.snapshot) playerStore.applyServerSnapshot(result.snapshot)
      if (result?.run) {
        meditationRunStatus.value = result.run
        meditationLogSeqInitialized.value = false
      }
    } catch (error) {
      showMessage('error', error?.message || '开启打坐失败')
    } finally {
      isMeditationSubmitting.value = false
    }
  }

  const stopMeditation = async () => {
    try {
      isMeditationSubmitting.value = true
      const result = await stopMeditationApi()
      if (result?.snapshot) playerStore.applyServerSnapshot(result.snapshot)
      meditationRunStatus.value.isActive = false
    } catch (error) {
      showMessage('error', '停止打坐失败')
    } finally {
      isMeditationSubmitting.value = false
    }
  }

  const startHunting = async () => {
    if (!selectedHuntingMapId.value) return
    try {
      isHuntingSubmitting.value = true
      if (isMeditating.value) await stopMeditation()
      const result = await startHuntingRunApi(selectedHuntingMapId.value)
      if (result?.snapshot) playerStore.applyServerSnapshot(result.snapshot)
      if (result?.run) {
        huntingRunStatus.value = result.run
        huntingStatusReceivedAt.value = Date.now()
      }
    } catch (error) {
      showMessage('error', error?.message || '进入地图失败')
    } finally {
      isHuntingSubmitting.value = false
    }
  }

  const stopHunting = async () => {
    try {
      isHuntingSubmitting.value = true
      const result = await stopHuntingRunApi()
      if (result?.snapshot) playerStore.applyServerSnapshot(result.snapshot)
      huntingRunStatus.value.isActive = false
    } catch (error) {
      showMessage('error', '退出地图失败')
    } finally {
      isHuntingSubmitting.value = false
    }
  }

  const handleBreakthrough = async () => {
    try {
      isBreakthroughSubmitting.value = true
      const result = await breakthroughApi()
      if (result?.snapshot) playerStore.applyServerSnapshot(result.snapshot)
      showMessage('success', `突破成功！当前境界：${playerStore.realm}`)
      await loadHuntingMaps()
    } catch (error) {
      showMessage('error', '突破失败')
    } finally {
      isBreakthroughSubmitting.value = false
    }
  }

  const loadHuntingMaps = async () => {
    try {
      const result = await listHuntingMapsApi()
      huntingMaps.value = result?.maps || []
      if (!selectedHuntingMapId.value && huntingMaps.value.length > 0) {
        selectedHuntingMapId.value = huntingMaps.value[0].id
      }
    } catch (error) {
      console.error('加载地图失败')
    }
  }

  // ---------------- 生命周期与监听 ----------------
  watch(() => gameRealtimeStore.meditationRun, run => {
    if (run) {
      meditationRunStatus.value = run
      if (run.lastLogMessage && run.lastLogSeq > meditationLastSeenLogSeq.value) {
        showMessage('info', run.lastLogMessage)
        meditationLastSeenLogSeq.value = run.lastLogSeq
      }
    }
  }, { immediate: true })

  watch(() => gameRealtimeStore.huntingRun, run => {
    if (run) {
      huntingRunStatus.value = run
      huntingStatusReceivedAt.value = Date.now()
      if (run.lastLogMessage && run.lastLogSeq > huntingLastSeenLogSeq.value) {
        const type = run.state === 'reviving' ? 'warning' : 'success'
        showMessage(type, run.lastLogMessage)
        huntingLastSeenLogSeq.value = run.lastLogSeq
      }
    }
  }, { immediate: true })

  onMounted(() => {
    loadHuntingMaps()
    huntingProgressTimer.value = setInterval(() => {
      huntingProgressNow.value = Date.now()
    }, huntingProgressRefreshIntervalMs)
  })

  onUnmounted(() => {
    if (huntingProgressTimer.value) clearInterval(huntingProgressTimer.value)
  })
</script>

<style scoped>
.cultivation-page {
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
  margin-bottom: 24px;
}

.page-title {
  font-size: 32px;
  font-family: var(--font-display);
  margin: 0;
}

.custom-tabs :deep(.n-tabs-nav) {
  margin-bottom: 20px;
}

/* 打坐样式 */
.meditation-container {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 32px;
  padding: 40px 20px;
  background: var(--panel-bg);
  border-radius: 24px;
  transition: all 0.5s ease;
}

.meditation-container.is-active {
  box-shadow: inset 0 0 60px var(--accent-muted);
}

.meditation-circle {
  width: 240px;
  height: 240px;
}

.circle-content {
  display: flex;
  flex-direction: column;
  align-items: center;
}

.circle-content .label { font-size: 14px; color: var(--ink-sub); }
.circle-content .value { font-size: 32px; font-family: var(--font-display); color: var(--accent-primary); margin: 4px 0; }
.circle-content .cap { font-size: 12px; color: var(--ink-sub); opacity: 0.6; }

.meditation-stats {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  width: 100%;
  gap: 16px;
}

.stat-box {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 16px;
  background: rgba(255, 255, 255, 0.05);
  border-radius: 16px;
  border: 1px solid var(--panel-border);
}

.s-label { font-size: 12px; color: var(--ink-sub); margin-bottom: 4px; }
.s-value { font-size: 16px; font-weight: 600; }

/* 刷怪地图样式 */
.map-selector {
  margin-bottom: 24px;
}

.selector-header {
  font-size: 14px;
  color: var(--ink-sub);
  margin-bottom: 12px;
}

.map-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(160px, 1fr));
  gap: 12px;
}

.map-card {
  position: relative;
  padding: 20px 16px;
  background: var(--panel-bg);
  border: 1px solid var(--panel-border);
  border-radius: 16px;
  cursor: pointer;
  transition: all 0.3s ease;
  overflow: hidden;
}

.map-card:hover {
  transform: translateY(-2px);
  border-color: var(--accent-primary);
}

.map-card.is-selected {
  border-color: var(--accent-primary);
  background: var(--accent-muted);
}

.map-card.is-locked {
  filter: grayscale(1);
  opacity: 0.6;
  cursor: not-allowed;
}

.map-level { font-size: 10px; color: var(--accent-primary); font-weight: bold; }
.map-name { font-size: 18px; font-family: var(--font-display); margin: 4px 0; }
.map-meta { font-size: 12px; color: var(--ink-sub); }

.lock-overlay {
  position: absolute;
  inset: 0;
  background: rgba(0,0,0,0.1);
  display: grid;
  place-items: center;
}

/* 战斗 HUD */
.combat-hud {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 20px;
  margin: 32px 0;
  padding: 24px;
  background: rgba(0,0,0,0.02);
  border-radius: 20px;
}

.combat-unit { flex: 1; }
.unit-label { font-size: 12px; margin-bottom: 8px; color: var(--ink-sub); }
.hp-bar { margin-bottom: 6px; }
.hp-text { font-size: 12px; font-variant-numeric: tabular-nums; }
.combat-vs { font-family: var(--font-display); font-size: 24px; opacity: 0.3; }

.yield-item {
  text-align: center;
  padding: 12px;
}
.y-label { font-size: 11px; color: var(--ink-sub); }
.y-value { font-size: 16px; font-weight: bold; margin-top: 4px; }

.action-bar {
  width: 100%;
  margin-top: 24px;
}

.cultivation-logs {
  margin-top: 32px;
  border-top: 1px dashed var(--panel-border);
  padding-top: 24px;
}

/* 灵气粒子动效 */
.spirit-particles {
  position: absolute;
  width: 300px;
  height: 300px;
}

.particle {
  position: absolute;
  width: 4px;
  height: 4px;
  background: var(--accent-primary);
  border-radius: 50%;
  filter: blur(1px);
  animation: gather 3s infinite ease-in;
}

@keyframes gather {
  0% { transform: translate(var(--tw-tx, 0), var(--tw-ty, 0)) scale(0); opacity: 0; }
  20% { opacity: 0.6; }
  100% { transform: translate(0, 0) scale(1); opacity: 0; }
}

/* 随机粒子位置 */
.particle:nth-child(1) { --tw-tx: 100px; --tw-ty: 100px; animation-delay: 0s; }
.particle:nth-child(2) { --tw-tx: -100px; --tw-ty: 100px; animation-delay: 0.4s; }
.particle:nth-child(3) { --tw-tx: 100px; --tw-ty: -100px; animation-delay: 0.8s; }
.particle:nth-child(4) { --tw-tx: -100px; --tw-ty: -100px; animation-delay: 1.2s; }
.particle:nth-child(5) { --tw-tx: 150px; --tw-ty: 0px; animation-delay: 1.6s; }
.particle:nth-child(6) { --tw-tx: -150px; --tw-ty: 0px; animation-delay: 2.0s; }
.particle:nth-child(7) { --tw-tx: 0px; --tw-ty: 150px; animation-delay: 2.4s; }
.particle:nth-child(8) { --tw-tx: 0px; --tw-ty: -150px; animation-delay: 2.8s; }

@media (max-width: 768px) {
  .meditation-circle { width: 180px; height: 180px; }
  .meditation-stats { grid-template-columns: 1fr; }
  .combat-hud { flex-direction: column; gap: 12px; }
  .combat-vs { transform: rotate(90deg); margin: 4px 0; }
}
</style>
