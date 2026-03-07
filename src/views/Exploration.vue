<template>
  <div class="page-view exploration-page">
    <!-- 顶部标题区 -->
    <header class="page-head">
      <div class="head-main">
        <p class="page-eyebrow">云游四海 · 寻觅机缘</p>
        <h2 class="page-title">大世界探索</h2>
      </div>
      <div class="head-status" v-if="isAutoExploring">
        <n-tag type="info" round class="pulse-tag">
          <template #icon><n-icon><CompassOutline /></n-icon></template>
          自动探索中: {{ explorationLocationLabel }}
        </n-tag>
      </div>
    </header>

    <!-- 当前探索实时 HUD -->
    <section class="exploration-hud" :class="{ 'is-active': isAutoExploring }">
      <div class="hud-main">
        <div class="hud-info">
          <div class="info-group">
            <span class="label">累计轮次</span>
            <strong class="value">{{ explorationTotalRuns }}</strong>
          </div>
          <div class="info-group">
            <span class="label">灵力消耗</span>
            <strong class="value">{{ explorationTotalSpiritCost }}</strong>
          </div>
          <div class="info-group wide">
            <span class="label">最新机缘</span>
            <div class="latest-log-hint">{{ explorationLatestMessage }}</div>
          </div>
        </div>
        
        <div class="hud-progress-area">
          <div class="progress-labels">
            <span>{{ explorationProgressText }}</span>
            <strong>{{ explorationProgressPercentDisplay }}%</strong>
          </div>
          <n-progress
            type="line"
            :percentage="explorationProgressPercent"
            :show-indicator="false"
            processing
            :height="12"
            color="var(--accent-primary)"
            rail-color="var(--accent-muted)"
          />
        </div>
      </div>
      
      <div class="hud-actions" v-if="isAutoExploring">
        <n-button type="error" secondary round @click="stopAutoExploration" :loading="isSubmitting">
          停止探索
        </n-button>
      </div>
    </section>

    <!-- 地图网格列表 -->
    <section class="locations-grid-section">
      <div class="section-title">选择历练地点</div>
      <div class="location-cards-container">
        <div 
          v-for="location in locationOptions" 
          :key="location.id"
          class="location-card"
          :class="{ 
            'is-locked': playerStore.level < location.minLevel,
            'is-active': isAutoExploring && explorationRunStatus.locationId === location.id
          }"
        >
          <div class="card-bg-pattern"></div>
          
          <div class="card-header">
            <div class="location-name">{{ location.name }}</div>
            <n-tag 
              v-if="playerStore.level < location.minLevel" 
              size="small" 
              type="error" 
              class="lock-tag"
            >
              <template #icon><n-icon><LockClosedOutline /></n-icon></template>
              {{ getRealmName(location.minLevel).name }}
            </n-tag>
            <n-tag v-else-if="isAutoExploring && explorationRunStatus.locationId === location.id" size="small" type="info" round>
              探索中
            </n-tag>
          </div>

          <p class="location-desc">{{ location.description }}</p>

          <div class="location-meta">
            <div class="meta-item">
              <span class="m-label">消耗灵力</span>
              <span class="m-value">{{ location.spiritCost }}</span>
            </div>
            <div class="meta-item">
              <span class="m-label">产出预览</span>
              <span class="m-value">机缘、灵草</span>
            </div>
          </div>

          <div class="card-actions">
            <n-button
              type="primary"
              secondary
              size="small"
              @click="exploreOnce(location)"
              :disabled="!canExploreOnce(location)"
              :loading="isSubmitting"
            >
              探索一次
            </n-button>

            <n-button
              v-if="!(isAutoExploring && explorationRunStatus.locationId === location.id)"
              type="success"
              size="small"
              round
              @click="startAutoExploration(location)"
              :disabled="!canStartAuto(location)"
              :loading="isSubmitting"
            >
              自动探索
            </n-button>
            <n-button
              v-else
              type="error"
              size="small"
              round
              ghost
              @click="stopAutoExploration"
              :loading="isSubmitting"
            >
              停止自动
            </n-button>
          </div>

          <!-- 锁定遮罩 -->
          <div v-if="playerStore.level < location.minLevel" class="lock-overlay"></div>
        </div>
      </div>
    </section>

    <!-- 探索日志 -->
    <section class="exploration-logs-section">
      <div class="section-head">
        <span class="section-title">历练传书</span>
        <n-button size="tiny" quaternary @click="clearLogPanel">清空</n-button>
      </div>
      <log-panel ref="logRef" title="" />
    </section>

    <!-- 底部统计面板 -->
    <footer class="exploration-footer">
      <div class="stats-ribbon">
        <div class="stat-chip">
          <span class="label">累计探索</span>
          <span class="value">{{ playerStore.explorationCount }} 次</span>
        </div>
        <div class="stat-chip">
          <span class="label">丹方残页</span>
          <span class="value">{{ totalPillFragments }} 片</span>
        </div>
        <div class="stat-chip connection-chip" :class="gameRealtimeStore.connected ? 'is-online' : 'is-offline'">
          <span class="dot"></span>
          <span>{{ realtimeStatusLabel }}</span>
          <n-button 
            v-if="!gameRealtimeStore.connected" 
            text 
            size="tiny" 
            type="primary" 
            @click="reconnectRealtime"
            style="margin-left: 8px"
          >
            重连
          </n-button>
        </div>
      </div>
    </footer>
  </div>
</template>

<script setup>
  import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
  import { CompassOutline, LockClosedOutline, PlanetOutline } from '@vicons/ionicons5'
  import { usePlayerStore } from '../stores/player'
  import { useGameRealtimeStore } from '../stores/game-realtime'
  import { getRealmName } from '../plugins/realm'
  import { locations } from '../plugins/locations'
  import LogPanel from '../components/LogPanel.vue'
  import {
    getExplorationStatus,
    startAutoExplorationRun,
    startExploration,
    stopAutoExplorationRun
  } from '../api/modules/game'

  const logRef = ref(null)
  const playerStore = usePlayerStore()
  const gameRealtimeStore = useGameRealtimeStore()
  const isSubmitting = ref(false)

  const explorationProgressRefreshIntervalMs = 200
  const explorationStatusSyncIntervalMs = 5000
  const explorationRealtimeStaleMs = 12000
  const explorationProgressTimer = ref(null)
  const explorationStatusSyncTimer = ref(null)
  const explorationProgressNow = ref(Date.now())
  const explorationStatusReceivedAt = ref(0)
  const explorationLastSeenLogSeq = ref(0)
  const explorationLogSeqInitialized = ref(false)
  const isLoadingStatus = ref(false)

  const toFiniteNumber = (value, fallback = 0) => {
    const num = Number(value)
    return Number.isFinite(num) ? num : fallback
  }

  const normalizeExplorationRun = run => {
    const fallback = {
      isActive: false,
      state: 'stopped',
      locationId: '',
      locationName: '',
      totalRuns: 0,
      totalSpiritCost: 0,
      lastLogSeq: 0,
      lastLogMessage: '',
      progressPercent: 0,
      progressRemainingMs: 0,
      progressLabel: '探索进度',
      startedAt: 0
    }

    if (!run || typeof run !== 'object') {
      return fallback
    }

    return {
      isActive: Boolean(run.isActive),
      state: String(run.state || fallback.state),
      locationId: String(run.locationId || ''),
      locationName: String(run.locationName || ''),
      totalRuns: Math.max(0, Math.floor(toFiniteNumber(run.totalRuns, 0))),
      totalSpiritCost: Math.max(0, Math.floor(toFiniteNumber(run.totalSpiritCost, 0))),
      lastLogSeq: Math.max(0, Math.floor(toFiniteNumber(run.lastLogSeq, 0))),
      lastLogMessage: String(run.lastLogMessage || ''),
      progressPercent: Math.max(0, Math.min(100, toFiniteNumber(run.progressPercent, 0))),
      progressRemainingMs: Math.max(0, Math.floor(toFiniteNumber(run.progressRemainingMs, 0))),
      progressLabel: String(run.progressLabel || fallback.progressLabel),
      startedAt: Math.max(0, Math.floor(toFiniteNumber(run.startedAt, 0)))
    }
  }

  const explorationRunStatus = ref(normalizeExplorationRun(null))

  const showMessage = (type, content) => {
    if (!content) return
    return logRef.value?.addLog(type, content)
  }

  const resolveExplorationLogType = message => {
    const text = String(message || '')
    if (!text) return 'info'
    if (text.includes('不足') || text.includes('不存在') || text.includes('失败')) {
      return 'error'
    }
    if (text.includes('停止') || text.includes('结束')) {
      return 'warning'
    }
    if (text.includes('获得') || text.includes('触发') || text.includes('开始') || text.includes('完成')) {
      return 'success'
    }
    return 'info'
  }

  const syncExplorationStatusLog = run => {
    if (!run || typeof run !== 'object') return
    const seq = Math.max(0, Math.floor(toFiniteNumber(run.lastLogSeq, 0)))
    const message = String(run.lastLogMessage || '').trim()

    if (!explorationLogSeqInitialized.value) {
      explorationLastSeenLogSeq.value = seq
      explorationLogSeqInitialized.value = true
      return
    }

    if (seq > explorationLastSeenLogSeq.value && message) {
      showMessage(resolveExplorationLogType(message), message)
    }
    explorationLastSeenLogSeq.value = seq
  }

  const applyServerResult = result => {
    if (result?.snapshot) {
      playerStore.applyServerSnapshot(result.snapshot)
    }
  }

  const applyExplorationActionResult = result => {
    if (result?.run) {
      explorationRunStatus.value = normalizeExplorationRun(result.run)
      explorationStatusReceivedAt.value = Date.now()
    }
    if (result?.message) {
      showMessage(resolveExplorationLogType(result.message), result.message)
    }
  }

  const locationOptions = computed(() => {
    return [...locations].sort((a, b) => a.minLevel - b.minLevel)
  })

  const isAutoExploring = computed(() => {
    return Boolean(explorationRunStatus.value?.isActive)
  })

  const explorationLocationLabel = computed(() => {
    const locationLabel = String(explorationRunStatus.value.locationName || explorationRunStatus.value.locationId || '').trim()
    return locationLabel || (isAutoExploring.value ? '未知地点' : '无')
  })

  const explorationTotalRuns = computed(() => Math.max(0, Math.floor(toFiniteNumber(explorationRunStatus.value?.totalRuns, 0))))
  const explorationTotalSpiritCost = computed(() => Math.max(0, Math.floor(toFiniteNumber(explorationRunStatus.value?.totalSpiritCost, 0))))

  const explorationLatestMessage = computed(() => {
    const message = String(explorationRunStatus.value?.lastLogMessage || '').trim()
    return message || (isAutoExploring.value ? '自动探索进行中' : '暂无新的探索记录')
  })

  const realtimeStatusLabel = computed(() => {
    if (gameRealtimeStore.connected) return '实时在线'
    if (gameRealtimeStore.connecting) return '正在重连'
    return '离线状态'
  })

  const explorationProgressPercent = computed(() => {
    if (!isAutoExploring.value) return 0
    const basePercent = Math.max(0, Math.min(100, toFiniteNumber(explorationRunStatus.value?.progressPercent, 0)))
    const remainingMs = Math.max(0, Math.floor(toFiniteNumber(explorationRunStatus.value?.progressRemainingMs, 0)))
    if (remainingMs <= 0 || explorationStatusReceivedAt.value <= 0) return basePercent
    const elapsedMs = Math.max(0, explorationProgressNow.value - explorationStatusReceivedAt.value)
    const remainRatio = Math.max(0.001, 1 - basePercent / 100)
    const totalMs = Math.max(1, Math.round(remainingMs / remainRatio))
    return Math.max(0, Math.min(100, basePercent + (elapsedMs * 100) / totalMs))
  })

  const explorationProgressPercentDisplay = computed(() => explorationProgressPercent.value.toFixed(1))

  const explorationProgressText = computed(() => {
    if (!isAutoExploring.value) return '未在探索'
    const progressName = explorationRunStatus.value?.progressLabel || '探索进度'
    return progressName
  })

  const totalPillFragments = computed(() => Object.values(playerStore.pillFragments || {}).reduce((a, b) => a + b, 0))

  const canExploreOnce = location => {
    if (!location || isSubmitting.value || isAutoExploring.value) return false
    return playerStore.level >= location.minLevel && playerStore.spirit >= location.spiritCost
  }

  const canStartAuto = location => {
    if (!location || isSubmitting.value) return false
    if (playerStore.level < location.minLevel || playerStore.spirit < location.spiritCost) return false
    if (!isAutoExploring.value) return true
    return explorationRunStatus.value.locationId === location.id
  }

  const exploreOnce = async location => {
    try {
      isSubmitting.value = true
      const result = await startExploration(location.id)
      applyServerResult(result)
      const messages = Array.isArray(result?.messages) ? result.messages : []
      if (messages.length === 0) showMessage('success', '探索完成！')
      else messages.forEach(m => showMessage(resolveExplorationLogType(m), m))
    } catch (error) {
      showMessage('error', error?.message || '探索失败！')
    } finally {
      isSubmitting.value = false
    }
  }

  const startAutoExploration = async location => {
    try {
      isSubmitting.value = true
      const result = await startAutoExplorationRun(location.id)
      applyServerResult(result)
      applyExplorationActionResult(result)
    } catch (error) {
      showMessage('error', error?.message || '开始自动探索失败')
    } finally {
      isSubmitting.value = false
    }
  }

  const stopAutoExploration = async () => {
    try {
      isSubmitting.value = true
      const result = await stopAutoExplorationRun()
      applyServerResult(result)
      applyExplorationActionResult(result)
    } catch (error) {
      showMessage('error', '停止失败')
    } finally {
      isSubmitting.value = false
    }
  }

  const refreshExplorationStatus = async ({ silent = false } = {}) => {
    if (isLoadingStatus.value) return
    try {
      isLoadingStatus.value = true
      const result = await getExplorationStatus()
      explorationRunStatus.value = normalizeExplorationRun(result)
      explorationStatusReceivedAt.value = Date.now()
      syncExplorationStatusLog(explorationRunStatus.value)
    } catch (error) {
      if (!silent) showMessage('error', '同步状态失败')
    } finally {
      isLoadingStatus.value = false
    }
  }

  const loadInitialExplorationStatus = async () => {
    if (gameRealtimeStore.explorationRun) {
      explorationRunStatus.value = normalizeExplorationRun(gameRealtimeStore.explorationRun)
      explorationStatusReceivedAt.value = Date.now()
      explorationLastSeenLogSeq.value = explorationRunStatus.value.lastLogSeq
      explorationLogSeqInitialized.value = true
      return
    }
    await refreshExplorationStatus()
  }

  const reconnectRealtime = () => gameRealtimeStore.connect()

  const clearLogPanel = () => logRef.value?.clearLogs()

  let progressTimer = null
  let syncTimer = null

  onMounted(async () => {
    await loadInitialExplorationStatus()
    progressTimer = setInterval(() => explorationProgressNow.value = Date.now(), 200)
    syncTimer = setInterval(() => {
      const lastSync = Number(gameRealtimeStore.lastSyncAt || 0)
      if (!gameRealtimeStore.connected || Date.now() - lastSync > 12000) refreshExplorationStatus({ silent: true })
    }, 5000)
  })

  onUnmounted(() => {
    clearInterval(progressTimer)
    clearInterval(syncTimer)
  })

  watch(() => gameRealtimeStore.explorationRun, run => {
    if (!run) return
    explorationRunStatus.value = normalizeExplorationRun(run)
    explorationStatusReceivedAt.value = Date.now()
    syncExplorationStatusLog(explorationRunStatus.value)
  }, { immediate: true })

  const qualityOptions = [
    { label: '全部品质', value: 'all' },
    { label: '筑基以上', value: 'higher' }
  ]
  const selectedQuality = ref('all')
</script>

<style scoped>
.exploration-page {
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

.pulse-tag {
  animation: pulse 2s infinite;
}

@keyframes pulse {
  0% { opacity: 1; }
  50% { opacity: 0.7; }
  100% { opacity: 1; }
}

/* HUD 样式 */
.exploration-hud {
  background: var(--panel-bg);
  border: 1px solid var(--panel-border);
  border-radius: 24px;
  padding: 24px;
  margin-bottom: 32px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  transition: all 0.3s ease;
  opacity: 0.6;
}

.exploration-hud.is-active {
  opacity: 1;
  border-color: var(--accent-primary);
  box-shadow: 0 8px 32px var(--accent-muted);
}

.hud-main { flex: 1; display: flex; flex-direction: column; gap: 20px; margin-right: 40px; }

.hud-info { display: grid; grid-template-columns: 100px 100px 1fr; gap: 24px; }
.info-group { display: flex; flex-direction: column; gap: 4px; }
.info-group .label { font-size: 12px; color: var(--ink-sub); }
.info-group .value { font-size: 18px; font-family: var(--font-display); }
.latest-log-hint { font-size: 14px; color: var(--accent-primary); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }

.hud-progress-area { display: flex; flex-direction: column; gap: 8px; }
.progress-labels { display: flex; justify-content: space-between; font-size: 13px; }

/* 地图卡片 */
.locations-grid-section { margin-bottom: 32px; }
.section-title { font-size: 16px; font-weight: bold; margin-bottom: 16px; font-family: var(--font-display); opacity: 0.8; }

.location-cards-container {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 16px;
}

.location-card {
  position: relative;
  background: var(--panel-bg);
  border: 1px solid var(--panel-border);
  border-radius: 20px;
  padding: 24px;
  overflow: hidden;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.location-card:hover:not(.is-locked) {
  transform: translateY(-4px);
  border-color: var(--accent-primary);
  box-shadow: 0 12px 24px rgba(0,0,0,0.05);
}

.location-card.is-active {
  border-color: var(--accent-primary);
  background: var(--accent-muted);
}

.card-bg-pattern {
  position: absolute;
  top: -20px;
  right: -20px;
  width: 100px;
  height: 100px;
  background: radial-gradient(circle, var(--accent-muted) 0%, transparent 70%);
  opacity: 0.3;
  pointer-events: none;
}

.card-header { display: flex; justify-content: space-between; align-items: center; }
.location-name { font-size: 20px; font-family: var(--font-display); font-weight: bold; }

.location-desc { font-size: 13px; color: var(--ink-sub); line-height: 1.6; height: 40px; overflow: hidden; }

.location-meta { display: flex; gap: 20px; padding: 12px 0; border-top: 1px dashed var(--panel-border); }
.meta-item { display: flex; flex-direction: column; gap: 2px; }
.m-label { font-size: 11px; color: var(--ink-sub); }
.m-value { font-size: 13px; font-weight: bold; }

.card-actions { display: grid; grid-template-columns: 1fr 1fr; gap: 10px; margin-top: auto; }

.lock-overlay {
  position: absolute;
  inset: 0;
  background: rgba(255,255,255,0.4);
  backdrop-filter: grayscale(1);
  z-index: 1;
}
.location-card.is-locked { opacity: 0.7; }

/* 日志部分 */
.exploration-logs-section {
  background: var(--panel-bg);
  border: 1px solid var(--panel-border);
  border-radius: 20px;
  padding: 20px;
  margin-bottom: 80px;
}
.section-head { display: flex; justify-content: space-between; align-items: center; margin-bottom: 12px; }

/* 底部统计 */
.exploration-footer {
  position: fixed;
  bottom: 80px; /* 留出 App 底部 Tabbar 空间 */
  left: 50%;
  transform: translateX(-50%);
  width: calc(100% - 40px);
  max-width: 800px;
  z-index: 100;
}

.stats-ribbon {
  background: color-mix(in srgb, var(--panel-bg) 90%, transparent);
  backdrop-filter: blur(12px);
  border: 1px solid var(--panel-border);
  border-radius: 999px;
  padding: 8px 24px;
  display: flex;
  justify-content: space-around;
  align-items: center;
  box-shadow: 0 10px 30px rgba(0,0,0,0.1);
}

.stat-chip { display: flex; align-items: center; gap: 8px; font-size: 13px; }
.stat-chip .label { color: var(--ink-sub); }
.stat-chip .value { font-weight: bold; color: var(--accent-primary); }

.connection-chip { gap: 6px; }
.dot { width: 6px; height: 6px; border-radius: 50%; background: #9ab0c6; }
.is-online .dot { background: #18a058; box-shadow: 0 0 8px #18a058; }
.is-offline .dot { background: #d03050; }

@media (max-width: 1080px) {
  .exploration-footer { bottom: 100px; width: 90%; }
}

@media (max-width: 768px) {
  .exploration-hud { flex-direction: column; gap: 20px; text-align: center; }
  .hud-main { margin-right: 0; width: 100%; }
  .hud-info { grid-template-columns: 1fr 1fr; }
  .info-group.wide { grid-column: span 2; }
  .stats-ribbon { flex-direction: column; border-radius: 20px; padding: 16px; gap: 10px; }
}
</style>
