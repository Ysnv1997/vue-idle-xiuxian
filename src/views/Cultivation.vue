<template>
  <section class="page-view cultivation-view">
    <header class="page-head">
      <p class="page-eyebrow">静室打坐</p>
      <h2>修炼</h2>
      <p class="page-desc">打坐用于恢复灵力，地图刷怪用于消耗灵力并获得修为与掉落。</p>
    </header>

    <n-card :bordered="false" class="page-card">
      <n-space vertical>
        <n-alert type="info" show-icon>
          <template #icon>
            <n-icon>
              <book-outline />
            </n-icon>
          </template>
          打坐与刷图互斥；打坐和离线恢复灵力最多累计 12 小时，刷图会持续消耗灵力并自动推进战斗。
        </n-alert>

        <n-tabs v-model:value="activeCultivationTab" type="line" animated>
          <n-tab-pane name="meditation" tab="静室打坐">
            <n-space vertical>
              <n-card size="small" embedded>
                <n-space vertical>
                  <n-progress
                    type="line"
                    :percentage="meditationSpiritPercent"
                    :show-indicator="false"
                    :height="14"
                    color="#18a058"
                  />

                  <n-descriptions bordered :column="1">
                    <n-descriptions-item label="当前状态">{{ meditationStateLabel }}</n-descriptions-item>
                    <n-descriptions-item label="当前灵力">
                      {{ formatDecimal(meditationCurrentSpirit, 1) }} / {{ formatDecimal(meditationSpiritCap, 1) }}
                    </n-descriptions-item>
                    <n-descriptions-item label="打坐恢复速度">
                      {{ formatDecimal(meditationCurrentRate, 2) }} / 秒
                    </n-descriptions-item>
                    <n-descriptions-item label="本轮累计恢复">
                      {{ formatDecimal(meditationTotalSpiritGain, 1) }}
                    </n-descriptions-item>
                    <n-descriptions-item label="预计充满时间">
                      {{ meditationFillEstimateLabel }}
                    </n-descriptions-item>
                  </n-descriptions>

                  <n-space justify="space-between">
                    <n-button
                      v-if="!isMeditating"
                      type="primary"
                      size="large"
                      :loading="isMeditationSubmitting"
                      :disabled="isHuntingRunning"
                      @click="startMeditation"
                    >
                      开始打坐
                    </n-button>
                    <n-button
                      v-else
                      type="warning"
                      size="large"
                      :loading="isMeditationSubmitting"
                      @click="stopMeditation()"
                    >
                      停止打坐
                    </n-button>

                    <n-button
                      type="info"
                      size="large"
                      :loading="isBreakthroughSubmitting"
                      :disabled="!canBreakthrough"
                      @click="handleBreakthrough"
                    >
                      手动突破
                    </n-button>
                  </n-space>

                  <n-text depth="3">
                    打坐由后端持续推进，离开修炼页面后仍可结算；若灵力已经回满，打坐会自动结束。
                  </n-text>
                </n-space>
              </n-card>
            </n-space>
          </n-tab-pane>

          <n-tab-pane name="hunting" tab="地图刷怪">
            <n-space vertical>
              <n-select
                v-model:value="selectedHuntingMapId"
                :options="huntingMapOptions"
                placeholder="选择刷怪地图"
                filterable
                :loading="isLoadingHuntingMaps || isLoadingHuntingStatus"
                :disabled="isHuntingRunning || isHuntingSubmitting"
              />

              <n-card v-if="selectedHuntingMap" size="small" embedded>
                <n-space vertical>
                  <n-text depth="3">{{ selectedHuntingMap.description }}</n-text>

                  <n-space justify="space-between">
                    <n-text>最低等级：{{ selectedHuntingMap.minLevel }}</n-text>
                    <n-text>推荐效率：{{ huntingEstimatedPerHour }} 修为/小时</n-text>
                  </n-space>

                  <n-space justify="space-between">
                    <n-space align="center" size="small">
                      <n-text>推荐战力：{{ selectedMapRecommendedPower }}</n-text>
                      <n-tag :type="isHuntingPowerRecommendedMet ? 'success' : 'warning'" size="small" bordered>
                        {{ isHuntingPowerRecommendedMet ? '当前达标' : '当前不足' }}
                      </n-tag>
                    </n-space>
                    <n-space align="center" size="small">
                      <n-text>推荐生命：{{ selectedMapRecommendedHealth }}</n-text>
                      <n-tag :type="isHuntingHealthRecommendedMet ? 'success' : 'warning'" size="small" bordered>
                        {{ isHuntingHealthRecommendedMet ? '当前达标' : '当前不足' }}
                      </n-tag>
                    </n-space>
                  </n-space>

                  <n-space justify="space-between">
                    <n-text>当前战力：{{ currentHuntingPower }}</n-text>
                    <n-text>当前生命：{{ currentHuntingHealth }}</n-text>
                  </n-space>

                  <n-text v-if="!isHuntingEntryRecommended" depth="3">
                    当前不满足推荐条件，实战死亡率会明显升高。
                  </n-text>

                  <n-space justify="space-between">
                    <n-text>单次消耗：{{ huntingEstimatedSpiritCost }} 灵力</n-text>
                    <n-text>单次收益：{{ huntingEstimatedGain }} 修为</n-text>
                  </n-space>

                  <n-space justify="space-between">
                    <n-text>当前状态：{{ huntingStateLabel }}</n-text>
                    <n-text>生命值：{{ huntingCurrentHpDisplay }} / {{ huntingMaxHpDisplay }} ({{ huntingHpPercent }}%)</n-text>
                  </n-space>

                  <n-space justify="space-between">
                    <n-text>累计击杀：{{ huntingKillCount }}</n-text>
                    <n-text>累计消耗：{{ huntingTotalSpiritCost }} 灵力</n-text>
                    <n-text>累计修为：{{ huntingTotalCultivationGain }}</n-text>
                  </n-space>

                  <n-space>
                    <n-tag
                      v-for="monster in selectedHuntingMap.monsters"
                      :key="monster"
                      size="small"
                      type="warning"
                      bordered
                    >
                      {{ monster }}
                    </n-tag>
                  </n-space>

                  <n-button
                    v-if="!isHuntingRunning"
                    type="warning"
                    size="large"
                    block
                    :disabled="!canStartHunting"
                    :loading="isHuntingSubmitting"
                    @click="startHunting"
                  >
                    开始刷怪（{{ selectedHuntingMap.name }}）
                  </n-button>
                  <n-button v-else type="error" size="large" block :loading="isHuntingSubmitting" @click="stopHunting()">
                    退出地图
                  </n-button>

                  <n-space v-if="isHuntingRunning" vertical size="small" class="hunting-progress-block">
                    <n-text depth="3">{{ huntingProgressDisplayText }}</n-text>
                    <div class="hunting-progress-row">
                      <n-progress
                        type="line"
                        :percentage="huntingProgressPercent"
                        :show-indicator="false"
                        :height="14"
                        :color="huntingRunStatus.state === 'reviving' ? '#f0a020' : '#18a058'"
                      />
                      <n-text class="hunting-progress-percent">{{ huntingProgressPercentDisplay }}%</n-text>
                    </div>
                  </n-space>

                  <n-text depth="3">刷怪由后端持续推进，离开本页面也会继续，最长离线收益 12 小时。</n-text>
                </n-space>
              </n-card>
            </n-space>
          </n-tab-pane>
        </n-tabs>

        <log-panel ref="logRef" title="修炼日志" />
      </n-space>
    </n-card>
  </section>
</template>

<script setup>
  import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
  import { NIcon } from 'naive-ui'
  import { BookOutline } from '@vicons/ionicons5'
  import LogPanel from '../components/LogPanel.vue'
  import { useGameRealtimeStore } from '../stores/game-realtime'
  import { usePlayerStore } from '../stores/player'
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

  const showMessage = (type, content) => {
    if (!content) return
    return logRef.value?.addLog(type, content)
  }

  const toFiniteNumber = (value, fallback = 0) => {
    const num = Number(value)
    return Number.isFinite(num) ? num : fallback
  }

  const formatDecimal = (value, digits = 1) => {
    return toFiniteNumber(value, 0).toFixed(digits)
  }

  const formatDuration = seconds => {
    const totalSeconds = Math.max(0, Math.ceil(toFiniteNumber(seconds, 0)))
    if (totalSeconds <= 0) return '即将充满'
    const hours = Math.floor(totalSeconds / 3600)
    const minutes = Math.floor((totalSeconds % 3600) / 60)
    const remainSeconds = totalSeconds % 60
    if (hours > 0) {
      return `${hours}小时${minutes}分`
    }
    if (minutes > 0) {
      return `${minutes}分${remainSeconds}秒`
    }
    return `${remainSeconds}秒`
  }

  const applyServerResult = result => {
    if (result?.snapshot) {
      playerStore.applyServerSnapshot(result.snapshot)
    }
  }

  const normalizeMeditationRun = run => {
    const fallback = {
      isActive: false,
      state: 'stopped',
      currentSpirit: 0,
      spiritCap: 0,
      currentRate: 0,
      totalSpiritGain: 0,
      startedAt: 0,
      lastLogSeq: 0,
      lastLogMessage: ''
    }
    if (!run || typeof run !== 'object') {
      return fallback
    }
    return {
      isActive: Boolean(run.isActive),
      state: String(run.state || fallback.state),
      currentSpirit: Math.max(0, toFiniteNumber(run.currentSpirit, 0)),
      spiritCap: Math.max(0, toFiniteNumber(run.spiritCap, 0)),
      currentRate: Math.max(0, toFiniteNumber(run.currentRate, 0)),
      totalSpiritGain: Math.max(0, toFiniteNumber(run.totalSpiritGain, 0)),
      startedAt: Math.max(0, Math.floor(toFiniteNumber(run.startedAt, 0))),
      lastLogSeq: Math.max(0, Math.floor(toFiniteNumber(run.lastLogSeq, 0))),
      lastLogMessage: String(run.lastLogMessage || '')
    }
  }

  const normalizeHuntingRun = run => {
    const fallback = {
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
    }
    if (!run || typeof run !== 'object') {
      return fallback
    }
    return {
      isActive: Boolean(run.isActive),
      state: String(run.state || fallback.state),
      mapId: String(run.mapId || ''),
      mapName: String(run.mapName || ''),
      currentHp: Math.max(0, toFiniteNumber(run.currentHp, 0)),
      maxHp: Math.max(0, toFiniteNumber(run.maxHp, 0)),
      killCount: Math.max(0, Math.floor(toFiniteNumber(run.killCount, 0))),
      totalSpiritCost: Math.max(0, Math.floor(toFiniteNumber(run.totalSpiritCost, 0))),
      totalCultivationGain: Math.max(0, Math.floor(toFiniteNumber(run.totalCultivationGain, 0))),
      progressPercent: Math.max(0, Math.min(100, toFiniteNumber(run.progressPercent, 0))),
      progressLabel: String(run.progressLabel || ''),
      progressRemainingMs: Math.max(0, Math.floor(toFiniteNumber(run.progressRemainingMs, 0))),
      lastLogSeq: Math.max(0, Math.floor(toFiniteNumber(run.lastLogSeq, 0))),
      lastLogMessage: String(run.lastLogMessage || ''),
      reviveUntil: Math.max(0, Math.floor(toFiniteNumber(run.reviveUntil, 0)))
    }
  }

  const formatMeditationState = state => {
    switch (state) {
      case 'running':
        return '打坐中'
      case 'full':
        return '灵力已满'
      case 'offline_timeout':
        return '离线结束'
      case 'stopped':
      default:
        return '未打坐'
    }
  }

  const formatHuntingState = state => {
    switch (state) {
      case 'running':
        return '战斗中'
      case 'defeat':
        return '已战败'
      case 'reviving':
        return '复活中'
      case 'exhausted':
        return '灵力耗尽'
      case 'offline_timeout':
        return '离线结束'
      case 'stopped':
        return '已退出'
      default:
        return '待命'
    }
  }

  const resolveMeditationLogType = message => {
    const text = String(message || '')
    if (!text) return 'info'
    if (text.includes('结束') || text.includes('停止')) {
      return 'warning'
    }
    if (text.includes('恢复') || text.includes('开始')) {
      return 'success'
    }
    return 'info'
  }

  const resolveHuntingLogType = message => {
    const text = String(message || '')
    if (!text) return 'info'
    if (text.includes('战死') || text.includes('耗尽') || text.includes('暂停') || text.includes('结束')) {
      return 'error'
    }
    if (text.includes('复活')) {
      return 'warning'
    }
    if (text.includes('击杀') || text.includes('获得')) {
      return 'success'
    }
    return 'info'
  }

  const syncMeditationStatusLog = run => {
    if (!run || typeof run !== 'object') return
    const seq = Math.max(0, Math.floor(toFiniteNumber(run.lastLogSeq, 0)))
    const message = String(run.lastLogMessage || '').trim()

    if (!meditationLogSeqInitialized.value) {
      meditationLastSeenLogSeq.value = seq
      meditationLogSeqInitialized.value = true
      return
    }

    if (seq > meditationLastSeenLogSeq.value && message) {
      showMessage(resolveMeditationLogType(message), message)
    }
    meditationLastSeenLogSeq.value = seq
  }

  const syncHuntingStatusLog = run => {
    if (!run || typeof run !== 'object') return
    const seq = Math.max(0, Math.floor(toFiniteNumber(run.lastLogSeq, 0)))
    const message = String(run.lastLogMessage || '').trim()

    if (!huntingLogSeqInitialized.value) {
      huntingLastSeenLogSeq.value = seq
      huntingLogSeqInitialized.value = true
      return
    }

    if (seq > huntingLastSeenLogSeq.value && message) {
      showMessage(resolveHuntingLogType(message), message)
    }
    huntingLastSeenLogSeq.value = seq
  }

  const isMeditating = computed(() => {
    return Boolean(meditationRunStatus.value?.isActive)
  })

  const meditationCurrentSpirit = computed(() => {
    return toFiniteNumber(meditationRunStatus.value?.currentSpirit, playerStore.spirit)
  })

  const meditationSpiritCap = computed(() => {
    return Math.max(0, toFiniteNumber(meditationRunStatus.value?.spiritCap, 0))
  })

  const meditationCurrentRate = computed(() => {
    return Math.max(0, toFiniteNumber(meditationRunStatus.value?.currentRate, 0))
  })

  const meditationTotalSpiritGain = computed(() => {
    return Math.max(0, toFiniteNumber(meditationRunStatus.value?.totalSpiritGain, 0))
  })

  const meditationStateLabel = computed(() => {
    return formatMeditationState(meditationRunStatus.value?.state)
  })

  const meditationSpiritPercent = computed(() => {
    const spiritCap = meditationSpiritCap.value
    if (spiritCap <= 0) return 0
    const percent = (meditationCurrentSpirit.value / spiritCap) * 100
    return Math.max(0, Math.min(100, percent))
  })

  const meditationFillEstimateLabel = computed(() => {
    const spiritCap = meditationSpiritCap.value
    const currentSpirit = meditationCurrentSpirit.value
    const currentRate = meditationCurrentRate.value
    if (spiritCap <= 0) return '未解锁'
    if (currentSpirit >= spiritCap) return '灵力已满'
    if (currentRate <= 0) return '无法恢复'
    return formatDuration((spiritCap - currentSpirit) / currentRate)
  })

  const canBreakthrough = computed(() => {
    return (
      playerStore.cultivation >= playerStore.maxCultivation &&
      !isHuntingRunning.value &&
      !isBreakthroughSubmitting.value &&
      !isMeditationSubmitting.value
    )
  })

  const applyMeditationStatus = run => {
    meditationRunStatus.value = normalizeMeditationRun(run)
    playerStore.spirit = meditationRunStatus.value.currentSpirit
  }

  const applyMeditationActionResult = (result, { silentMessage = false } = {}) => {
    if (result?.run) {
      applyMeditationStatus(result.run)
      meditationLastSeenLogSeq.value = meditationRunStatus.value.lastLogSeq
      meditationLogSeqInitialized.value = true
    }
    if (!silentMessage && result?.message) {
      showMessage(resolveMeditationLogType(result.message), result.message)
    }
  }

  const startMeditation = async () => {
    if (isMeditationSubmitting.value) return
    if (isHuntingRunning.value) {
      activeCultivationTab.value = 'hunting'
      showMessage('warning', '刷怪进行中，请先退出地图。')
      return
    }

    try {
      isMeditationSubmitting.value = true
      const result = await startMeditationApi()
      applyServerResult(result)
      applyMeditationActionResult(result)
    } catch (error) {
      if (error?.payload?.error === 'spirit already full') {
        showMessage('warning', '当前灵力已满，无需继续打坐。')
        return
      }
      if (error?.payload?.error === 'meditation conflict' && error?.payload?.conflict === 'hunting') {
        activeCultivationTab.value = 'hunting'
        showMessage('warning', '刷怪进行中，无法开始打坐。')
        return
      }
      showMessage('error', error?.message || '开始打坐失败')
    } finally {
      isMeditationSubmitting.value = false
    }
  }

  const stopMeditation = async ({ silent = false } = {}) => {
    if (isMeditationSubmitting.value) return

    try {
      isMeditationSubmitting.value = true
      const result = await stopMeditationApi()
      applyServerResult(result)
      applyMeditationActionResult(result, { silentMessage: silent })
    } catch (error) {
      if (!silent) {
        showMessage('error', error?.message || '停止打坐失败')
      }
    } finally {
      isMeditationSubmitting.value = false
    }
  }

  const handleBreakthrough = async () => {
    if (!canBreakthrough.value) return
    try {
      isBreakthroughSubmitting.value = true
      const result = await breakthroughApi()
      applyServerResult(result)
      showMessage('success', `突破成功，当前境界：${playerStore.realm}`)
      await loadHuntingMaps()
    } catch (error) {
      if (error?.payload?.error === 'breakthrough unavailable') {
        showMessage('warning', '当前修为尚未达到突破要求。')
        return
      }
      showMessage('error', error?.message || '突破失败')
    } finally {
      isBreakthroughSubmitting.value = false
    }
  }

  const huntingMapOptions = computed(() => {
    return huntingMaps.value.map(map => ({
      label: `${map.name}（Lv.${map.minLevel}+）`,
      value: map.id,
      disabled: playerStore.level < map.minLevel
    }))
  })

  const selectedHuntingMap = computed(() => {
    return huntingMaps.value.find(map => map.id === selectedHuntingMapId.value) || null
  })

  const huntingEstimatedSpiritCost = computed(() => {
    if (!selectedHuntingMap.value) return 0
    return Math.max(1, Math.floor(toFiniteNumber(selectedHuntingMap.value.estimatedCost, 1)))
  })

  const huntingEstimatedGain = computed(() => {
    if (!selectedHuntingMap.value) return 0
    return Math.max(1, Math.floor(toFiniteNumber(selectedHuntingMap.value.estimatedGain, 1)))
  })

  const selectedMapRecommendedPower = computed(() => {
    return Math.max(0, Math.floor(toFiniteNumber(selectedHuntingMap.value?.recommendedPower, 0)))
  })

  const selectedMapRecommendedHealth = computed(() => {
    return Math.max(0, Math.floor(toFiniteNumber(selectedHuntingMap.value?.recommendedHealth, 0)))
  })

  const currentHuntingPower = computed(() => {
    const base = playerStore.baseAttributes || {}
    const attack = toFiniteNumber(base.attack, 0)
    const defense = toFiniteNumber(base.defense, 0)
    const health = toFiniteNumber(base.health, 0)
    const speed = toFiniteNumber(base.speed, 0)
    const level = toFiniteNumber(playerStore.level, 0)
    const power = attack * 2 + defense * 1.5 + health * 0.2 + speed + level * 10
    return Math.max(0, Math.floor(power))
  })

  const currentHuntingHealth = computed(() => {
    return Math.max(0, Math.floor(toFiniteNumber(playerStore.baseAttributes?.health, 0)))
  })

  const isHuntingPowerRecommendedMet = computed(() => {
    return selectedMapRecommendedPower.value <= 0 || currentHuntingPower.value >= selectedMapRecommendedPower.value
  })

  const isHuntingHealthRecommendedMet = computed(() => {
    return selectedMapRecommendedHealth.value <= 0 || currentHuntingHealth.value >= selectedMapRecommendedHealth.value
  })

  const isHuntingEntryRecommended = computed(() => {
    return isHuntingPowerRecommendedMet.value && isHuntingHealthRecommendedMet.value
  })

  const huntingEstimatedPerHour = computed(() => {
    const cost = Number(huntingEstimatedSpiritCost.value)
    const gain = Number(huntingEstimatedGain.value)
    const spiritRate = Number(meditationCurrentRate.value || 0)
    if (!Number.isFinite(cost) || cost <= 0) return 0
    if (!Number.isFinite(gain) || gain <= 0) return 0
    if (Number.isFinite(spiritRate) && spiritRate > 0) {
      const actionsPerSecond = Math.min(1, spiritRate / cost)
      return Math.max(0, Math.floor(actionsPerSecond * gain * 3600))
    }
    return Math.max(0, Math.floor(toFiniteNumber(selectedHuntingMap.value?.estimatedPerHour, 0)))
  })

  const isHuntingRunning = computed(() => {
    return Boolean(huntingRunStatus.value?.isActive)
  })

  const huntingStateLabel = computed(() => {
    return formatHuntingState(huntingRunStatus.value?.state)
  })

  const huntingCurrentHpDisplay = computed(() => {
    return toFiniteNumber(huntingRunStatus.value?.currentHp, 0).toFixed(1)
  })

  const huntingMaxHpDisplay = computed(() => {
    return toFiniteNumber(huntingRunStatus.value?.maxHp, 0).toFixed(1)
  })

  const huntingHpPercent = computed(() => {
    const maxHp = toFiniteNumber(huntingRunStatus.value?.maxHp, 0)
    if (maxHp <= 0) return 0
    const currentHp = toFiniteNumber(huntingRunStatus.value?.currentHp, 0)
    const ratio = (currentHp / maxHp) * 100
    return Math.max(0, Math.min(100, Math.floor(ratio)))
  })

  const huntingKillCount = computed(() => {
    return Math.max(0, Math.floor(toFiniteNumber(huntingRunStatus.value?.killCount, 0)))
  })

  const huntingTotalSpiritCost = computed(() => {
    return Math.max(0, Math.floor(toFiniteNumber(huntingRunStatus.value?.totalSpiritCost, 0)))
  })

  const huntingTotalCultivationGain = computed(() => {
    return Math.max(0, Math.floor(toFiniteNumber(huntingRunStatus.value?.totalCultivationGain, 0)))
  })

  const huntingProgressPercent = computed(() => {
    if (!isHuntingRunning.value) return 0
    const basePercent = Math.max(0, Math.min(100, toFiniteNumber(huntingRunStatus.value?.progressPercent, 0)))
    const remainingMs = Math.max(0, Math.floor(toFiniteNumber(huntingRunStatus.value?.progressRemainingMs, 0)))
    if (remainingMs <= 0 || huntingStatusReceivedAt.value <= 0) {
      return basePercent
    }
    const elapsedMs = Math.max(0, huntingProgressNow.value - huntingStatusReceivedAt.value)
    const currentState = String(huntingRunStatus.value?.state || '')
    if (currentState === 'reviving') {
      const remainRatio = Math.max(0.001, 1 - basePercent / 100)
      const totalMs = Math.max(1, Math.round(remainingMs / remainRatio))
      const percent = basePercent + (elapsedMs * 100) / totalMs
      return Math.max(0, Math.min(100, percent))
    }
    const percent = basePercent + (elapsedMs * 100) / 1000
    return Math.max(0, Math.min(100, percent))
  })

  const huntingProgressPercentDisplay = computed(() => {
    return huntingProgressPercent.value.toFixed(2)
  })

  const huntingProgressRemainingMs = computed(() => {
    if (!isHuntingRunning.value) return 0
    const baseRemaining = Math.max(0, Math.floor(toFiniteNumber(huntingRunStatus.value?.progressRemainingMs, 0)))
    if (baseRemaining <= 0 || huntingStatusReceivedAt.value <= 0) {
      return baseRemaining
    }
    const elapsedMs = Math.max(0, huntingProgressNow.value - huntingStatusReceivedAt.value)
    return Math.max(0, baseRemaining - elapsedMs)
  })

  const huntingProgressDisplayText = computed(() => {
    const state = String(huntingRunStatus.value?.state || '')
    const progressName = huntingRunStatus.value?.progressLabel || (state === 'reviving' ? '复活倒计时' : '击杀进度')
    const remainingSeconds = Math.max(0, Math.ceil(huntingProgressRemainingMs.value / 1000))
    if (state === 'reviving') {
      return `${progressName}：${remainingSeconds}秒后复活`
    }
    return `${progressName}：预计${remainingSeconds}秒内结算当前战斗`
  })

  const canStartHunting = computed(() => {
    if (!selectedHuntingMap.value) return false
    if (playerStore.level < selectedHuntingMap.value.minLevel) return false
    if (isHuntingSubmitting.value || isMeditationSubmitting.value || isBreakthroughSubmitting.value) return false
    if (isHuntingRunning.value) return false
    return true
  })

  const pickDefaultHuntingMap = () => {
    if (!huntingMaps.value.length) {
      selectedHuntingMapId.value = ''
      return
    }
    const unlocked = huntingMaps.value
      .filter(map => playerStore.level >= map.minLevel)
      .sort((a, b) => b.minLevel - a.minLevel)
    if (unlocked.length > 0) {
      selectedHuntingMapId.value = unlocked[0].id
      return
    }
    selectedHuntingMapId.value = huntingMaps.value[0].id
  }

  const loadHuntingMaps = async () => {
    try {
      isLoadingHuntingMaps.value = true
      const result = await listHuntingMapsApi()
      const maps = Array.isArray(result?.maps) ? result.maps : []
      huntingMaps.value = maps
      const currentExists = maps.some(map => map.id === selectedHuntingMapId.value)
      if (!currentExists) {
        pickDefaultHuntingMap()
      }
    } catch (error) {
      showMessage('error', error?.message || '加载刷怪地图失败')
      huntingMaps.value = []
      selectedHuntingMapId.value = ''
    } finally {
      isLoadingHuntingMaps.value = false
    }
  }

  const applyHuntingRunResult = result => {
    if (result?.run) {
      huntingRunStatus.value = normalizeHuntingRun(result.run)
      huntingStatusReceivedAt.value = Date.now()
    }

    if (result?.message) {
      const messageType =
        result.state === 'defeat' || result.state === 'exhausted' || result.state === 'offline_timeout'
          ? 'error'
          : result.state === 'reviving' || result.state === 'stopped'
            ? 'warning'
            : 'success'
      showMessage(messageType, result.message)
    }
  }

  const clearHuntingProgressTimer = () => {
    if (!huntingProgressTimer.value) return
    clearInterval(huntingProgressTimer.value)
    huntingProgressTimer.value = null
  }

  const startHuntingProgressTicker = () => {
    if (huntingProgressTimer.value) return
    huntingProgressTimer.value = setInterval(() => {
      huntingProgressNow.value = Date.now()
    }, huntingProgressRefreshIntervalMs)
  }

  const startHunting = async () => {
    const currentMap = selectedHuntingMap.value
    if (!currentMap) {
      showMessage('warning', '请先选择刷怪地图')
      return
    }
    if (playerStore.level < currentMap.minLevel) {
      showMessage('error', `境界不足，需达到${currentMap.minLevel}级`)
      return
    }
    if (isHuntingSubmitting.value || isMeditationSubmitting.value || isBreakthroughSubmitting.value) return

    try {
      isHuntingSubmitting.value = true
      if (isMeditating.value) {
        await stopMeditation({ silent: true })
        showMessage('warning', '开始刷怪前已自动停止打坐。')
      }

      huntingLastSeenLogSeq.value = 0
      huntingLogSeqInitialized.value = true
      const result = await startHuntingRunApi(currentMap.id)
      applyServerResult(result)
      applyHuntingRunResult(result)
      if (result?.run?.mapId) {
        selectedHuntingMapId.value = result.run.mapId
      }
    } catch (error) {
      if (error?.payload?.error === 'hunting map locked') {
        showMessage('error', `地图未解锁，需达到${error.payload.requiredLevel || 0}级`)
        return
      }
      if (error?.payload?.error === 'invalid hunting map') {
        showMessage('error', '地图配置不存在，正在重新拉取...')
        await loadHuntingMaps()
        return
      }
      showMessage('error', error?.message || '进入地图失败')
    } finally {
      isHuntingSubmitting.value = false
    }
  }

  const stopHunting = async ({ silent = false } = {}) => {
    if (!huntingRunStatus.value?.isActive) {
      huntingRunStatus.value = {
        ...huntingRunStatus.value,
        isActive: false,
        state: 'stopped'
      }
      if (!silent) {
        showMessage('warning', '已停止刷怪。')
      }
      return
    }

    try {
      isHuntingSubmitting.value = true
      const result = await stopHuntingRunApi()
      applyServerResult(result)
      applyHuntingRunResult(result)
    } catch (error) {
      showMessage('error', error?.message || '退出地图失败')
    } finally {
      isHuntingSubmitting.value = false
    }
  }

  watch(
    () => gameRealtimeStore.meditationRun,
    run => {
      if (!run || typeof run !== 'object') return
      applyMeditationStatus(run)
      syncMeditationStatusLog(meditationRunStatus.value)
    },
    { immediate: true }
  )

  watch(
    () => gameRealtimeStore.huntingRun,
    run => {
      if (!run || typeof run !== 'object') return
      isLoadingHuntingStatus.value = false
      huntingRunStatus.value = normalizeHuntingRun(run)
      huntingStatusReceivedAt.value = Date.now()
      syncHuntingStatusLog(huntingRunStatus.value)
      if (huntingRunStatus.value.isActive && huntingRunStatus.value.mapId) {
        selectedHuntingMapId.value = huntingRunStatus.value.mapId
      } else if (!selectedHuntingMap.value) {
        pickDefaultHuntingMap()
      }
    },
    { immediate: true }
  )

  onMounted(async () => {
    isLoadingHuntingStatus.value = !gameRealtimeStore.huntingRun
    if (gameRealtimeStore.meditationRun) {
      applyMeditationStatus(gameRealtimeStore.meditationRun)
    }
    if (gameRealtimeStore.huntingRun) {
      huntingRunStatus.value = normalizeHuntingRun(gameRealtimeStore.huntingRun)
      huntingStatusReceivedAt.value = Date.now()
    }
    await loadHuntingMaps()
    startHuntingProgressTicker()
  })

  onUnmounted(() => {
    clearHuntingProgressTimer()
  })
</script>

<style scoped>
  :deep(.n-space) {
    width: 100%;
  }

  .hunting-progress-block {
    margin-top: 4px;
  }

  .hunting-progress-row {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .hunting-progress-row :deep(.n-progress) {
    flex: 1;
  }

  .hunting-progress-percent {
    min-width: 56px;
    text-align: right;
  }
</style>
