<template>
  <section class="page-view cultivation-view">
    <header class="page-head">
      <p class="page-eyebrow">静室打坐</p>
      <h2>修炼</h2>
      <p class="page-desc">通过打坐修炼提升修为，积累足够后尝试突破境界。</p>
    </header>

    <n-card :bordered="false" class="page-card">
      <n-space vertical>
        <n-alert type="info" show-icon>
          <template #icon>
            <n-icon>
              <book-outline />
            </n-icon>
          </template>
          修炼受灵力影响，建议在灵力充足时使用一键突破。
        </n-alert>
        <n-space vertical>
          <n-button
            type="primary"
            size="large"
            block
            @click="cultivate"
            :disabled="playerStore.spirit < cultivationCost || isSubmitting"
            :loading="isSubmitting"
          >
            打坐修炼 (消耗 {{ cultivationCost }} 灵力)
          </n-button>
          <n-button :type="isAutoCultivating ? 'warning' : 'success'" size="large" block @click="toggleAutoCultivation">
            {{ isAutoCultivating ? '停止自动修炼' : '开始自动修炼' }}
          </n-button>
          <n-button
            type="info"
            size="large"
            block
            @click="cultivateUntilBreakthrough"
            :disabled="playerStore.spirit < calculateBreakthroughCost() || isSubmitting"
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
        <log-panel ref="logRef" title="修炼日志" />
      </n-space>
    </n-card>
  </section>
</template>

<script setup>
  import { usePlayerStore } from '../stores/player'
  import { computed, onUnmounted, ref } from 'vue'
  import { NIcon } from 'naive-ui'
  import { BookOutline } from '@vicons/ionicons5'
  import LogPanel from '../components/LogPanel.vue'
  import {
    cultivateOnce as cultivateOnceApi,
    cultivateUntilBreakthrough as cultivateUntilBreakthroughApi
  } from '../api/modules/game'

  const playerStore = usePlayerStore()
  const logRef = ref(null)

  // 修炼相关数值
  const baseGainRate = 1 // 基础灵力获取率
  const baseCultivationCost = 10 // 基础修炼消耗的灵力
  const baseCultivationGain = 1 // 基础修炼获得的修为
  const autoCultivateInterval = 1000 // 自动修炼基础间隔（毫秒）
  const autoCultivateMinWait = 1000 // 无灵力时最短等待（毫秒）
  const autoCultivateMaxWait = 60000 // 无灵力时最长等待（毫秒）

  // 计算当前境界的修炼消耗
  const getCurrentCultivationCost = () => {
    return Math.floor(baseCultivationCost * Math.pow(1.5, playerStore.level - 1))
  }

  // 计算当前境界的修炼获得
  const getCurrentCultivationGain = () => {
    return Math.floor(baseCultivationGain * Math.pow(1.2, playerStore.level - 1))
  }

  // 计算当前修炼消耗（作为计算属性）
  const cultivationCost = computed(() => {
    return getCurrentCultivationCost()
  })

  // 计算当前修炼获得（作为计算属性）
  const cultivationGain = computed(() => {
    return getCurrentCultivationGain()
  })

  // 计算突破所需的总灵力
  const calculateBreakthroughCost = () => {
    const remainingCultivation = Math.max(0, playerStore.maxCultivation - playerStore.cultivation)
    const gain = cultivationGain?.value || 1
    if (gain <= 0) return 0
    const cultivationTimes = Math.ceil(remainingCultivation / gain)
    return Math.max(0, cultivationTimes * getCurrentCultivationCost())
  }

  // 自动修炼状态
  const isAutoCultivating = ref(false)
  const cultivationTimer = ref(null)
  const isSubmitting = ref(false)
  const autoPausedForSpirit = ref(false)

  // 显示消息并处理重复
  const showMessage = (type, content) => {
    return logRef.value?.addLog(type, content)
  }

  const applyServerResult = result => {
    if (result?.snapshot) {
      playerStore.applyServerSnapshot(result.snapshot)
    }
  }

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

  // 一键修炼（直到突破）
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

  // 手动修炼
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

  // 切换自动修炼
  const toggleAutoCultivation = () => {
    try {
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

  // 组件卸载时清理定时器
  onUnmounted(() => {
    try {
      clearAutoTimer()
      isAutoCultivating.value = false
      autoPausedForSpirit.value = false
    } catch (error) {
      console.error('清理定时器出错：', error)
    }
  })
</script>

<style scoped>
  :deep(.n-space) {
    width: 100%;
  }

  .n-button {
    margin-bottom: 12px;
  }
</style>
