<template>
  <section class="page-view cultivation-view">
    <header class="page-head">
      <p class="page-eyebrow">静室打坐</p>
      <h2>修炼</h2>
      <p class="page-desc">通过打坐修炼提升修为，或进入地图持续刷怪加速成长。</p>
    </header>

    <n-card :bordered="false" class="page-card">
      <n-space vertical>
        <n-alert type="info" show-icon>
          <template #icon>
            <n-icon>
              <book-outline />
            </n-icon>
          </template>
          修炼受灵力影响；刷怪会持续战斗，战败后会自动复活继续战斗，灵力耗尽才会暂停。
        </n-alert>

        <n-tabs v-model:value="activeCultivationTab" type="line" animated>
          <n-tab-pane name="meditation" tab="静室打坐">
            <n-space vertical>
              <n-space vertical>
                <n-button
                  type="primary"
                  size="large"
                  block
                  @click="cultivate"
                  :disabled="playerStore.spirit < cultivationCost || isSubmitting || isHuntingSubmitting || isHuntingRunning"
                  :loading="isSubmitting"
                >
                  打坐修炼 (消耗 {{ cultivationCost }} 灵力)
                </n-button>
                <n-button
                  :type="isAutoCultivating ? 'warning' : 'success'"
                  size="large"
                  block
                  @click="toggleAutoCultivation"
                  :disabled="isHuntingSubmitting || isHuntingRunning"
                >
                  {{ isAutoCultivating ? '停止自动修炼' : '开始自动修炼' }}
                </n-button>
                <n-button
                  type="info"
                  size="large"
                  block
                  @click="cultivateUntilBreakthrough"
                  :disabled="
                    playerStore.spirit < calculateBreakthroughCost() ||
                    isSubmitting ||
                    isHuntingSubmitting ||
                    isHuntingRunning
                  "
                  :loading="isSubmitting"
                >
                  一键突破
                </n-button>
              </n-space>

              <n-divider>修炼详情</n-divider>
              <n-descriptions bordered>
                <n-descriptions-item label="灵力获取速率">{{ baseGainRate * playerStore.spiritRate }} / 秒</n-descriptions-item>
                <n-descriptions-item label="修炼效率">{{ cultivationGain }} 修为 / 次</n-descriptions-item>
                <n-descriptions-item label="突破所需修为">
                  {{ playerStore.maxCultivation }}
                </n-descriptions-item>
              </n-descriptions>
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
                    :disabled="!canStartHunting || isAutoCultivating"
                    :loading="isHuntingSubmitting"
                    @click="startHunting"
                  >
                    开始刷怪（{{ selectedHuntingMap.name }}）
                  </n-button>
                  <n-button v-else type="error" size="large" block :loading="isHuntingSubmitting" @click="stopHunting()">
                    退出地图
                  </n-button>

                  <n-space v-if="isHuntingRunning" vertical size="small" class="hunting-progress-block">
                    <n-text depth="3">
                      {{ huntingProgressDisplayText }}
                    </n-text>
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
  import { computed, onMounted, onUnmounted, ref } from 'vue'
  import { NIcon } from 'naive-ui'
  import { BookOutline } from '@vicons/ionicons5'
  import LogPanel from '../components/LogPanel.vue'
  import { usePlayerStore } from '../stores/player'
  import {
    cultivateOnce as cultivateOnceApi,
    cultivateUntilBreakthrough as cultivateUntilBreakthroughApi,
    getHuntingStatus as getHuntingStatusApi,
    listHuntingMaps as listHuntingMapsApi,
    startHuntingRun as startHuntingRunApi,
    stopHuntingRun as stopHuntingRunApi
  } from '../api/modules/game'

  const playerStore = usePlayerStore()
  const logRef = ref(null)

  const baseGainRate = 1
  const baseCultivationCost = 10
  const baseCultivationGain = 1

  const autoCultivateInterval = 1000
  const autoCultivateMinWait = 1000
  const autoCultivateMaxWait = 60000

  const huntingStatusSyncIntervalMs = 1000
  const huntingProgressRefreshIntervalMs = 200

  const getCurrentCultivationCost = () => {
    return Math.floor(baseCultivationCost * Math.pow(1.5, playerStore.level - 1))
  }

  const getCurrentCultivationGain = () => {
    return Math.floor(baseCultivationGain * Math.pow(1.2, playerStore.level - 1))
  }

  const cultivationCost = computed(() => {
    return getCurrentCultivationCost()
  })

  const cultivationGain = computed(() => {
    return getCurrentCultivationGain()
  })

  const calculateBreakthroughCost = () => {
    const remainingCultivation = Math.max(0, playerStore.maxCultivation - playerStore.cultivation)
    const gain = cultivationGain?.value || 1
    if (gain <= 0) return 0
    const cultivationTimes = Math.ceil(remainingCultivation / gain)
    return Math.max(0, cultivationTimes * getCurrentCultivationCost())
  }

  const isAutoCultivating = ref(false)
  const activeCultivationTab = ref('meditation')
  const cultivationTimer = ref(null)
  const isSubmitting = ref(false)
  const autoPausedForSpirit = ref(false)

  const isLoadingHuntingMaps = ref(false)
  const isLoadingHuntingStatus = ref(false)
  const isHuntingSubmitting = ref(false)
  const huntingMaps = ref([])
  const selectedHuntingMapId = ref('')
  const huntingStatusTimer = ref(null)
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

  const applyServerResult = result => {
    if (result?.snapshot) {
      playerStore.applyServerSnapshot(result.snapshot)
    }
  }

  const estimateHuntingValue = (baseValue, factor) => {
    const base = Number(baseValue)
    const ratio = Number(factor)
    if (!Number.isFinite(base) || base <= 0) return 1
    if (!Number.isFinite(ratio) || ratio <= 0) return Math.max(1, Math.floor(base))
    return Math.max(1, Math.floor(base * ratio))
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
    if (!selectedHuntingMap.value) return cultivationCost.value
    return estimateHuntingValue(cultivationCost.value, selectedHuntingMap.value.rewardFactor)
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

  const huntingEstimatedGain = computed(() => {
    if (!selectedHuntingMap.value) return cultivationGain.value
    const baseCost = Number(cultivationCost.value)
    const baseGain = Number(cultivationGain.value)
    const targetCost = Number(huntingEstimatedSpiritCost.value)
    if (!Number.isFinite(baseCost) || baseCost <= 0) return Math.max(1, Math.floor(baseGain) || 1)
    if (!Number.isFinite(baseGain) || baseGain <= 0) return 1
    if (!Number.isFinite(targetCost) || targetCost <= 0) return 1
    const aligned = targetCost * (baseGain / baseCost)
    return Math.max(1, Math.floor(aligned * 2.0))
  })

  const huntingEstimatedPerHour = computed(() => {
    const cost = Number(huntingEstimatedSpiritCost.value)
    const gain = Number(huntingEstimatedGain.value)
    const spiritRate = Number(playerStore.spiritRate || 0)
    if (!Number.isFinite(cost) || cost <= 0) return 0
    if (!Number.isFinite(gain) || gain <= 0) return 0
    if (!Number.isFinite(spiritRate) || spiritRate <= 0) return 0
    const actionsPerSecond = Math.min(1, spiritRate / cost)
    return Math.max(0, Math.floor(actionsPerSecond * gain * 3600))
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
    if (isSubmitting.value || isHuntingSubmitting.value) return false
    if (isHuntingRunning.value) return false
    return true
  })

  const showServerResultMessages = (result, fallbackSuccessMessage = '修炼成功！') => {
    if (result?.doubleGainTimes > 0) {
      showMessage('success', `福缘不错，获得${result.doubleGainTimes}次双倍修为！`)
    }
    if (result?.breakthrough) {
      showMessage('success', `突破成功！恭喜进入${playerStore.realm}！`)
      return
    }
    showMessage('success', fallbackSuccessMessage)
  }

  const showServerError = error => {
    if (error?.payload?.error === 'insufficient spirit') {
      showMessage(
        'error',
        `灵力不足！突破需要${(error.payload.requiredSpirit || 0).toFixed(0)}灵力，当前灵力：${(error.payload.currentSpirit || 0).toFixed(1)}`
      )
      return
    }
    showMessage('error', error?.message || '修炼失败！')
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
          : result.state === 'reviving'
            ? 'warning'
          : result.state === 'stopped'
            ? 'warning'
            : 'success'
      showMessage(messageType, result.message)
    }
  }

  const loadHuntingStatus = async ({ silent = false } = {}) => {
    try {
      isLoadingHuntingStatus.value = true
      const result = await getHuntingStatusApi()
      huntingRunStatus.value = normalizeHuntingRun(result)
      huntingStatusReceivedAt.value = Date.now()
      syncHuntingStatusLog(huntingRunStatus.value)

      if (huntingRunStatus.value.isActive && huntingRunStatus.value.mapId) {
        selectedHuntingMapId.value = huntingRunStatus.value.mapId
      } else if (!selectedHuntingMap.value) {
        pickDefaultHuntingMap()
      }
    } catch (error) {
      if (!silent) {
        showMessage('error', error?.message || '加载刷怪状态失败')
      }
    } finally {
      isLoadingHuntingStatus.value = false
    }
  }

  const clearHuntingStatusTimer = () => {
    if (!huntingStatusTimer.value) return
    clearInterval(huntingStatusTimer.value)
    huntingStatusTimer.value = null
  }

  const clearHuntingProgressTimer = () => {
    if (!huntingProgressTimer.value) return
    clearInterval(huntingProgressTimer.value)
    huntingProgressTimer.value = null
  }

  const startHuntingStatusPolling = () => {
    if (huntingStatusTimer.value) return
    huntingStatusTimer.value = setInterval(() => {
      loadHuntingStatus({ silent: true })
    }, huntingStatusSyncIntervalMs)
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
    if (isHuntingSubmitting.value || isSubmitting.value) return

    if (isAutoCultivating.value) {
      isAutoCultivating.value = false
      clearAutoTimer()
      autoPausedForSpirit.value = false
      showMessage('warning', '已自动停止打坐，切换到刷怪模式。')
    }

    try {
      isHuntingSubmitting.value = true
      huntingLastSeenLogSeq.value = 0
      huntingLogSeqInitialized.value = true
      const result = await startHuntingRunApi(currentMap.id)
      applyServerResult(result)
      applyHuntingRunResult(result)

      if (result?.run?.mapId) {
        selectedHuntingMapId.value = result.run.mapId
      }

      await loadHuntingStatus({ silent: true })
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
      await loadHuntingStatus({ silent: true })
    } catch (error) {
      showMessage('error', error?.message || '退出地图失败')
    } finally {
      isHuntingSubmitting.value = false
    }
  }

  const clearAutoTimer = () => {
    if (!cultivationTimer.value) return
    clearTimeout(cultivationTimer.value)
    cultivationTimer.value = null
  }

  const estimateAutoWaitMs = (requiredSpirit, currentSpirit) => {
    const required = Number(requiredSpirit)
    const current = Number(currentSpirit)
    const regenRate = Number(playerStore.spiritRate || 0)

    if (!Number.isFinite(required) || !Number.isFinite(current) || required <= current) {
      return autoCultivateMinWait
    }

    if (!Number.isFinite(regenRate) || regenRate <= 0) {
      return 3000
    }

    const missing = required - current
    const seconds = missing / regenRate
    const waitMs = Math.ceil(seconds * 1000 + 200)
    return Math.min(autoCultivateMaxWait, Math.max(autoCultivateMinWait, waitMs))
  }

  const resolveInsufficientSpiritWaitMs = error => {
    const retryAfterMs = Number(error?.payload?.retryAfterMs)
    if (Number.isFinite(retryAfterMs) && retryAfterMs > 0) {
      return Math.min(autoCultivateMaxWait, Math.max(autoCultivateMinWait, Math.ceil(retryAfterMs)))
    }

    const retryAfterSeconds = Number(error?.payload?.retryAfterSeconds)
    if (Number.isFinite(retryAfterSeconds) && retryAfterSeconds > 0) {
      return Math.min(autoCultivateMaxWait, Math.max(autoCultivateMinWait, Math.ceil(retryAfterSeconds * 1000)))
    }

    const required = Number(error?.payload?.requiredSpirit)
    const current = Number(error?.payload?.currentSpirit)
    if (Number.isFinite(required) && Number.isFinite(current)) {
      return estimateAutoWaitMs(required, current)
    }

    return estimateAutoWaitMs(cultivationCost.value, playerStore.spirit)
  }

  const scheduleAutoCultivation = delayMs => {
    if (!isAutoCultivating.value) return
    clearAutoTimer()
    cultivationTimer.value = setTimeout(() => {
      cultivationTimer.value = null
      runAutoCultivationTick()
    }, Math.max(200, Number(delayMs) || autoCultivateInterval))
  }

  const runAutoCultivationTick = async () => {
    if (!isAutoCultivating.value) return
    if (isHuntingRunning.value) {
      isAutoCultivating.value = false
      clearAutoTimer()
      autoPausedForSpirit.value = false
      showMessage('warning', '刷怪进行中，自动打坐已停止。')
      return
    }
    if (isSubmitting.value) {
      scheduleAutoCultivation(300)
      return
    }

    if (playerStore.spirit < cultivationCost.value) {
      if (!autoPausedForSpirit.value) {
        autoPausedForSpirit.value = true
        const waitMs = estimateAutoWaitMs(cultivationCost.value, playerStore.spirit)
        showMessage('warning', `灵力不足，自动修炼已进入等待（约${Math.ceil(waitMs / 1000)}秒后重试）`)
      }
      scheduleAutoCultivation(estimateAutoWaitMs(cultivationCost.value, playerStore.spirit))
      return
    }

    autoPausedForSpirit.value = false

    try {
      isSubmitting.value = true
      const result = await cultivateOnceApi()
      applyServerResult(result)
      showServerResultMessages(result, '修炼成功！')
      scheduleAutoCultivation(autoCultivateInterval)
    } catch (error) {
      if (error?.payload?.error === 'insufficient spirit') {
        if (!autoPausedForSpirit.value) {
          autoPausedForSpirit.value = true
          const waitMs = resolveInsufficientSpiritWaitMs(error)
          showMessage('warning', `灵力不足，自动修炼已进入等待（约${Math.ceil(waitMs / 1000)}秒后重试）`)
        }
        scheduleAutoCultivation(resolveInsufficientSpiritWaitMs(error))
        return
      }

      showServerError(error)
      scheduleAutoCultivation(1500)
    } finally {
      isSubmitting.value = false
    }
  }

  const cultivateUntilBreakthrough = async () => {
    if (isSubmitting.value) return
    try {
      isSubmitting.value = true
      const result = await cultivateUntilBreakthroughApi()
      applyServerResult(result)
      showServerResultMessages(result, '修炼成功！')
    } catch (error) {
      showServerError(error)
    } finally {
      isSubmitting.value = false
    }
  }

  const cultivate = async () => {
    if (isSubmitting.value) return
    try {
      isSubmitting.value = true
      const result = await cultivateOnceApi()
      applyServerResult(result)
      showServerResultMessages(result, '修炼成功！')
    } catch (error) {
      showServerError(error)
    } finally {
      isSubmitting.value = false
    }
  }

  const toggleAutoCultivation = () => {
    try {
      if (isHuntingRunning.value) {
        showMessage('warning', '刷怪进行中，请先退出地图。')
        return
      }
      isAutoCultivating.value = !isAutoCultivating.value
      if (isAutoCultivating.value) {
        autoPausedForSpirit.value = false
        scheduleAutoCultivation(0)
      } else {
        clearAutoTimer()
        autoPausedForSpirit.value = false
      }
    } catch (error) {
      console.error('切换自动修炼出错：', error)
      logRef.value?.addLog('error', '切换失败！')
      isAutoCultivating.value = false
      clearAutoTimer()
      autoPausedForSpirit.value = false
    }
  }

  onUnmounted(() => {
    try {
      clearAutoTimer()
      clearHuntingStatusTimer()
      clearHuntingProgressTimer()
      isAutoCultivating.value = false
      autoPausedForSpirit.value = false
    } catch (error) {
      console.error('清理定时器出错：', error)
    }
  })

  onMounted(async () => {
    await loadHuntingMaps()
    await loadHuntingStatus({ silent: true })
    startHuntingStatusPolling()
    startHuntingProgressTicker()
  })
</script>

<style scoped>
  :deep(.n-space) {
    width: 100%;
  }

  .n-button {
    margin-bottom: 12px;
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
