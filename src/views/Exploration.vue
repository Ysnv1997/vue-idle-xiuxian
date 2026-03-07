<template>
  <section class="page-view exploration-view">
    <header class="page-head">
      <p class="page-eyebrow">外出历练</p>
      <h2>探索</h2>
      <p class="page-desc">探索各处秘境，寻找机缘造化。自动探索由后端持续推进，离开页面也不会中断。</p>
    </header>

    <n-card :bordered="false" class="page-card">
      <n-space vertical>
        <n-alert type="info" show-icon>
          <template #icon>
            <n-icon>
              <compass-outline />
            </n-icon>
          </template>
          自动探索按 3 秒一轮结算，离线最长累计 12 小时；探索会与打坐、刷图、炼丹、秘境互斥。
        </n-alert>

        <n-card size="small" embedded>
          <n-space vertical>
            <n-space justify="space-between" align="center">
              <n-space align="center" size="small">
                <n-tag :type="realtimeStatusTagType" size="small" bordered>
                  {{ realtimeStatusLabel }}
                </n-tag>
                <n-text depth="3">{{ realtimeStatusDetail }}</n-text>
              </n-space>

              <n-button
                v-if="!gameRealtimeStore.connected"
                size="small"
                secondary
                :disabled="gameRealtimeStore.connecting"
                @click="reconnectRealtime"
              >
                {{ gameRealtimeStore.connecting ? '连接中' : '重连实时状态' }}
              </n-button>
            </n-space>

            <n-progress
              type="line"
              :percentage="explorationProgressPercent"
              :show-indicator="false"
              :height="14"
              color="#2080f0"
            />

            <n-descriptions bordered :column="2" label-placement="left">
              <n-descriptions-item label="当前状态">{{ explorationStateLabel }}</n-descriptions-item>
              <n-descriptions-item label="当前地点">{{ explorationLocationLabel }}</n-descriptions-item>
              <n-descriptions-item label="累计探索">{{ explorationTotalRuns }}</n-descriptions-item>
              <n-descriptions-item label="累计消耗">{{ explorationTotalSpiritCost }} 灵力</n-descriptions-item>
              <n-descriptions-item :span="2" label="最近动态">
                {{ explorationLatestMessage }}
              </n-descriptions-item>
            </n-descriptions>

            <div v-if="isAutoExploring" class="exploration-progress-row">
              <n-text depth="3">{{ explorationProgressText }}</n-text>
              <n-text class="exploration-progress-percent">{{ explorationProgressPercentDisplay }}%</n-text>
            </div>
          </n-space>
        </n-card>

        <n-grid :cols="2" :x-gap="12" :y-gap="12">
          <n-grid-item v-for="location in locationOptions" :key="location.id">
            <n-card :title="location.name" size="small">
              <n-space vertical>
                <n-text depth="3">{{ location.description }}</n-text>
                <n-space justify="space-between">
                  <n-text>消耗灵力：{{ location.spiritCost }}</n-text>
                  <n-text>最低境界：{{ getRealmName(location.minLevel).name }}</n-text>
                </n-space>

                <n-space justify="space-between">
                  <n-tag v-if="playerStore.level >= location.minLevel" size="small" type="success" bordered>已解锁</n-tag>
                  <n-tag v-else size="small" type="warning" bordered>未解锁</n-tag>
                  <n-tag
                    v-if="isAutoExploring && explorationRunStatus.locationId === location.id"
                    size="small"
                    type="info"
                    bordered
                  >
                    自动探索中
                  </n-tag>
                </n-space>

                <n-space>
                  <n-button
                    type="primary"
                    @click="exploreOnce(location)"
                    :disabled="!canExploreOnce(location)"
                    :loading="isSubmitting"
                  >
                    探索一次
                  </n-button>

                  <n-button
                    v-if="isAutoExploring && explorationRunStatus.locationId === location.id"
                    type="warning"
                    @click="stopAutoExploration"
                    :loading="isSubmitting"
                  >
                    停止自动
                  </n-button>
                  <n-button
                    v-else
                    type="success"
                    @click="startAutoExploration(location)"
                    :disabled="!canStartAuto(location)"
                    :loading="isSubmitting"
                  >
                    开始自动
                  </n-button>
                </n-space>
              </n-space>
            </n-card>
          </n-grid-item>
        </n-grid>

        <n-divider>探索统计</n-divider>
        <n-descriptions :column="2" bordered>
          <n-descriptions-item label="探索次数">
            {{ playerStore.explorationCount }}
          </n-descriptions-item>
          <n-descriptions-item label="灵石数量">
            {{ playerStore.spiritStones }}
          </n-descriptions-item>
          <n-descriptions-item label="灵草数量">
            {{ playerStore.herbs.length }}
          </n-descriptions-item>
          <n-descriptions-item label="丹方残页">
            {{ Object.values(playerStore.pillFragments || {}).reduce((a, b) => a + b, 0) }}
          </n-descriptions-item>
        </n-descriptions>
      </n-space>
    </n-card>

    <n-card :bordered="false" class="page-card log-card">
      <n-space justify="end" style="margin-bottom: 8px">
        <n-button size="small" @click="clearLogPanel" type="error" secondary>清空日志</n-button>
      </n-space>
      <log-panel ref="logRef" title="探索日志" />
    </n-card>
  </section>
</template>

<script setup>
  import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
  import { CompassOutline } from '@vicons/ionicons5'
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

  const formatExplorationState = state => {
    switch (state) {
      case 'running':
        return '自动探索中'
      case 'insufficient_spirit':
        return '灵力不足'
      case 'offline_timeout':
        return '离线结束'
      case 'invalid_location':
        return '地点失效'
      case 'stopped':
      default:
        return '未探索'
    }
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

  const explorationStateLabel = computed(() => {
    return formatExplorationState(explorationRunStatus.value?.state)
  })

  const explorationLocationLabel = computed(() => {
    const locationLabel = String(explorationRunStatus.value.locationName || explorationRunStatus.value.locationId || '').trim()
    if (locationLabel) {
      return locationLabel
    }
    return isAutoExploring.value ? '未知地点' : '无'
  })

  const explorationTotalRuns = computed(() => {
    return Math.max(0, Math.floor(toFiniteNumber(explorationRunStatus.value?.totalRuns, 0)))
  })

  const explorationTotalSpiritCost = computed(() => {
    return Math.max(0, Math.floor(toFiniteNumber(explorationRunStatus.value?.totalSpiritCost, 0)))
  })

  const explorationLatestMessage = computed(() => {
    const message = String(explorationRunStatus.value?.lastLogMessage || '').trim()
    if (message) {
      return message
    }
    if (isAutoExploring.value) {
      return '自动探索进行中'
    }
    return '暂无新的探索记录'
  })

  const realtimeStatusTagType = computed(() => {
    if (gameRealtimeStore.connected) {
      return 'success'
    }
    if (gameRealtimeStore.connecting) {
      return 'warning'
    }
    return 'default'
  })

  const realtimeStatusLabel = computed(() => {
    if (gameRealtimeStore.connected) {
      return '实时同步中'
    }
    if (gameRealtimeStore.connecting) {
      return '实时重连中'
    }
    return '实时已断开'
  })

  const realtimeStatusDetail = computed(() => {
    const now = explorationProgressNow.value
    if (gameRealtimeStore.connected) {
      const lastSyncAt = Number(gameRealtimeStore.lastSyncAt || 0)
      if (lastSyncAt <= 0) {
        return '等待首个服务端状态'
      }
      const seconds = Math.max(0, Math.floor((now - lastSyncAt) / 1000))
      return seconds <= 1 ? '刚刚收到服务端状态' : `${seconds}秒前收到服务端状态`
    }
    if (gameRealtimeStore.connecting) {
      return '正在重连游戏实时状态'
    }
    return '当前页面会自动回退到状态补拉'
  })

  const explorationProgressPercent = computed(() => {
    if (!isAutoExploring.value) return 0
    const basePercent = Math.max(0, Math.min(100, toFiniteNumber(explorationRunStatus.value?.progressPercent, 0)))
    const remainingMs = Math.max(0, Math.floor(toFiniteNumber(explorationRunStatus.value?.progressRemainingMs, 0)))
    if (remainingMs <= 0 || explorationStatusReceivedAt.value <= 0) {
      return basePercent
    }
    const elapsedMs = Math.max(0, explorationProgressNow.value - explorationStatusReceivedAt.value)
    const remainRatio = Math.max(0.001, 1 - basePercent / 100)
    const totalMs = Math.max(1, Math.round(remainingMs / remainRatio))
    const percent = basePercent + (elapsedMs * 100) / totalMs
    return Math.max(0, Math.min(100, percent))
  })

  const explorationProgressPercentDisplay = computed(() => {
    return explorationProgressPercent.value.toFixed(2)
  })

  const explorationProgressRemainingMs = computed(() => {
    if (!isAutoExploring.value) return 0
    const baseRemaining = Math.max(0, Math.floor(toFiniteNumber(explorationRunStatus.value?.progressRemainingMs, 0)))
    if (baseRemaining <= 0 || explorationStatusReceivedAt.value <= 0) {
      return baseRemaining
    }
    const elapsedMs = Math.max(0, explorationProgressNow.value - explorationStatusReceivedAt.value)
    return Math.max(0, baseRemaining - elapsedMs)
  })

  const explorationProgressText = computed(() => {
    const progressName = explorationRunStatus.value?.progressLabel || '探索进度'
    const remainingSeconds = Math.max(0, Math.ceil(explorationProgressRemainingMs.value / 1000))
    return `${progressName}：预计${remainingSeconds}秒内完成当前轮次`
  })

  const canExploreOnce = location => {
    if (!location || isSubmitting.value) return false
    if (isAutoExploring.value) return false
    if (playerStore.level < Number(location.minLevel || 0)) return false
    return playerStore.spirit >= Number(location.spiritCost || 0)
  }

  const canStartAuto = location => {
    if (!location || isSubmitting.value) return false
    if (playerStore.level < Number(location.minLevel || 0)) return false
    if (playerStore.spirit < Number(location.spiritCost || 0)) return false
    if (!isAutoExploring.value) return true
    return explorationRunStatus.value.locationId === location.id
  }

  const exploreOnce = async location => {
    if (!canExploreOnce(location)) {
      if (isAutoExploring.value) {
        showMessage('warning', '自动探索进行中，请先停止自动探索。')
      } else if (playerStore.spirit < location.spiritCost) {
        showMessage('error', '灵力不足！')
      }
      return
    }

    try {
      isSubmitting.value = true
      const result = await startExploration(location.id)
      applyServerResult(result)
      const messages = Array.isArray(result?.messages) ? result.messages : []
      if (messages.length === 0) {
        showMessage('success', '探索完成！')
      } else {
        for (const message of messages) {
          showMessage(resolveExplorationLogType(message), message)
        }
      }
    } catch (error) {
      if (error?.payload?.error === 'insufficient spirit') {
        showMessage('error', '灵力不足！')
        return
      }
      if (error?.payload?.error === 'location locked') {
        showMessage('error', `境界不足，需达到${error.payload.requiredLevel}级`)
        return
      }
      if (error?.payload?.error === 'invalid location') {
        showMessage('error', '地点不存在，请刷新后重试')
        return
      }
      if (error?.payload?.error === 'activity conflict' && error?.payload?.conflict === 'dungeon') {
        showMessage('warning', '秘境进行中，无法探索。')
        return
      }
      if (error?.payload?.error === 'activity conflict' && error?.payload?.conflict === 'exploration') {
        showMessage('warning', '自动探索进行中，请先停止自动探索。')
        return
      }
      showMessage('error', error?.message || '探索失败！')
    } finally {
      isSubmitting.value = false
    }
  }

  const startAutoExploration = async location => {
    if (!canStartAuto(location)) {
      if (isAutoExploring.value && explorationRunStatus.value.locationId !== location.id) {
        showMessage('warning', '已有自动探索在进行，请先停止当前自动探索。')
      }
      return
    }

    try {
      isSubmitting.value = true
      const result = await startAutoExplorationRun(location.id)
      applyServerResult(result)
      applyExplorationActionResult(result)
    } catch (error) {
      if (error?.payload?.error === 'insufficient spirit') {
        showMessage('error', '灵力不足，无法开始自动探索。')
        return
      }
      if (error?.payload?.error === 'location locked') {
        showMessage('error', `境界不足，需达到${error.payload.requiredLevel}级`)
        return
      }
      if (error?.payload?.error === 'invalid location') {
        showMessage('error', '地点不存在，请刷新后重试')
        return
      }
      if (error?.payload?.error === 'activity conflict' && error?.payload?.conflict === 'dungeon') {
        showMessage('warning', '秘境进行中，无法开始自动探索。')
        return
      }
      if (error?.payload?.error === 'activity conflict' && error?.payload?.conflict === 'exploration') {
        showMessage('warning', '自动探索进行中，请先停止当前自动探索。')
        return
      }
      showMessage('error', error?.message || '开始自动探索失败')
    } finally {
      isSubmitting.value = false
    }
  }

  const stopAutoExploration = async () => {
    if (isSubmitting.value) return
    try {
      isSubmitting.value = true
      const result = await stopAutoExplorationRun()
      applyServerResult(result)
      applyExplorationActionResult(result)
    } catch (error) {
      showMessage('error', error?.message || '停止自动探索失败')
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
      if (!silent) {
        showMessage('error', error?.message || '加载探索状态失败')
      }
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

  const clearExplorationProgressTimer = () => {
    if (!explorationProgressTimer.value) return
    clearInterval(explorationProgressTimer.value)
    explorationProgressTimer.value = null
  }

  const startExplorationProgressTimer = () => {
    if (explorationProgressTimer.value) return
    explorationProgressTimer.value = setInterval(() => {
      explorationProgressNow.value = Date.now()
    }, explorationProgressRefreshIntervalMs)
  }

  const clearExplorationStatusSyncTimer = () => {
    if (!explorationStatusSyncTimer.value) return
    clearInterval(explorationStatusSyncTimer.value)
    explorationStatusSyncTimer.value = null
  }

  const startExplorationStatusSyncTimer = () => {
    if (explorationStatusSyncTimer.value) return
    explorationStatusSyncTimer.value = setInterval(() => {
      const lastSyncAt = Number(gameRealtimeStore.lastSyncAt || 0)
      const realtimeIsFresh =
        gameRealtimeStore.connected && lastSyncAt > 0 && Date.now() - lastSyncAt <= explorationRealtimeStaleMs
      if (realtimeIsFresh) {
        return
      }
      void refreshExplorationStatus({ silent: true })
    }, explorationStatusSyncIntervalMs)
  }

  const reconnectRealtime = () => {
    gameRealtimeStore.connect()
  }

  watch(
    () => gameRealtimeStore.explorationRun,
    run => {
      if (!run || typeof run !== 'object') return
      explorationRunStatus.value = normalizeExplorationRun(run)
      explorationStatusReceivedAt.value = Date.now()
      syncExplorationStatusLog(explorationRunStatus.value)
    },
    { immediate: true }
  )

  onMounted(async () => {
    await loadInitialExplorationStatus()
    startExplorationProgressTimer()
    startExplorationStatusSyncTimer()
  })

  onUnmounted(() => {
    clearExplorationProgressTimer()
    clearExplorationStatusSyncTimer()
  })

  const clearLogPanel = () => {
    logRef.value?.clearLogs()
  }
</script>

<style scoped>
  :deep(.n-space) {
    width: 100%;
  }

  .exploration-progress-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
  }

  .exploration-progress-percent {
    min-width: 56px;
    text-align: right;
  }

  @media (max-width: 768px) {
    :deep(.n-grid) {
      grid-template-columns: minmax(0, 1fr) !important;
    }

    :deep(.n-descriptions) {
      --n-td-padding: 8px;
    }

    .exploration-progress-row {
      flex-direction: column;
      align-items: flex-start;
    }

    .exploration-progress-percent {
      min-width: 0;
      text-align: left;
    }
  }
</style>
