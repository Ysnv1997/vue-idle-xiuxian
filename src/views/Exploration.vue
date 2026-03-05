<template>
  <section class="page-view exploration-view">
    <header class="page-head">
      <p class="page-eyebrow">外出历练</p>
      <h2>探索</h2>
      <p class="page-desc">探索各处秘境，寻找机缘造化。小心谨慎，危险与机遇并存。</p>
    </header>

    <n-card :bordered="false" class="page-card">
      <n-space vertical>
        <n-alert type="info" show-icon>
          <template #icon>
            <n-icon>
              <compass-outline />
            </n-icon>
          </template>
          每个地点都有灵力消耗与境界要求，建议按当前灵力储备安排自动探索。
        </n-alert>
        <n-grid :cols="2" :x-gap="12">
          <n-grid-item v-for="location in availableLocations" :key="location.id">
            <n-card :title="location.name" size="small">
              <n-space vertical>
                <n-text depth="3">{{ location.description }}</n-text>
                <n-space justify="space-between">
                  <n-text>消耗灵力：{{ location.spiritCost }}</n-text>
                  <n-text>最低境界：{{ getRealmName(location.minLevel).name }}</n-text>
                </n-space>
                <n-space>
                  <n-button
                    type="primary"
                    @click="exploreLocation(location)"
                    :disabled="playerStore.spirit < location.spiritCost || isAutoExploring || isSubmitting"
                    :loading="isSubmitting"
                  >
                    探索
                  </n-button>
                  <n-button
                    :type="exploringLocations[location.id] ? 'warning' : 'success'"
                    @click="
                      exploringLocations[location.id] ? stopAutoExploration(location) : startAutoExploration(location)
                    "
                    :disabled="
                      playerStore.spirit < location.spiritCost ||
                      (isAutoExploring && !exploringLocations[location.id]) ||
                      isSubmitting
                    "
                  >
                    {{ exploringLocations[location.id] ? '停止' : '自动' }}
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
  import { computed, onUnmounted, ref } from 'vue'
  import { usePlayerStore } from '../stores/player'
  import { CompassOutline } from '@vicons/ionicons5'
  import { getRealmName } from '../plugins/realm'
  import { locations } from '../plugins/locations'
  import LogPanel from '../components/LogPanel.vue'
  import { startExploration } from '../api/modules/game'

  const logRef = ref(null)
  const playerStore = usePlayerStore()
  // 探索相关数值
  const explorationInterval = 3000 // 探索间隔（毫秒）
  const exploringLocations = ref({}) // 记录每个地点的探索状态
  const explorationTimers = ref({}) // 记录每个地点的定时器
  const isAutoExploring = ref(false) // 是否有地点正在自动探索
  const autoExploringLocationId = ref(null) // 正在自动探索的地点ID
  const isSubmitting = ref(false)

  // 探索指定地点
  const exploreLocation = async location => {
    if (playerStore.spirit < location.spiritCost) {
      showMessage('error', '灵力不足！')
      return
    }
    if (isSubmitting.value) return
    try {
      isSubmitting.value = true
      const result = await startExploration(location.id)
      if (result?.snapshot) {
        playerStore.applyServerSnapshot(result.snapshot)
      }
      const messages = Array.isArray(result?.messages) ? result.messages : []
      if (messages.length === 0) {
        showMessage('success', '探索完成！')
      } else {
        messages.forEach(message => {
          const type = message.includes('损失') ? 'error' : message.includes('触发') ? 'info' : 'success'
          showMessage(type, message)
        })
      }
    } catch (error) {
      if (error?.payload?.error === 'insufficient spirit') {
        showMessage('error', '灵力不足！')
        stopAllAutoExploration()
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
      showMessage('error', error?.message || '探索失败！')
    } finally {
      isSubmitting.value = false
    }
  }

  // 获取可用地点列表
  const availableLocations = computed(() => {
    return locations.filter(loc => playerStore.level >= loc.minLevel)
  })

  // 显示消息并处理重复
  const showMessage = (type, content) => {
    return logRef.value?.addLog(type, content)
  }

  const stopAllAutoExploration = () => {
    Object.keys(explorationTimers.value).forEach(locationId => {
      clearInterval(explorationTimers.value[locationId])
      delete explorationTimers.value[locationId]
      exploringLocations.value[locationId] = false
    })
    isAutoExploring.value = false
    autoExploringLocationId.value = null
  }

  // 开始自动探索
  const startAutoExploration = location => {
    if (exploringLocations.value[location.id] || isAutoExploring.value) return
    isAutoExploring.value = true
    autoExploringLocationId.value = location.id
    exploringLocations.value[location.id] = true
    explorationTimers.value[location.id] = setInterval(() => {
      if (playerStore.spirit >= location.spiritCost) {
        exploreLocation(location)
      } else {
        stopAutoExploration(location)
        showMessage('warning', '灵力不足，自动探索已停止！')
      }
    }, explorationInterval)
  }

  // 停止自动探索
  const stopAutoExploration = location => {
    if (explorationTimers.value[location.id]) {
      clearInterval(explorationTimers.value[location.id])
      delete explorationTimers.value[location.id]
    }
    exploringLocations.value[location.id] = false
    isAutoExploring.value = false
    autoExploringLocationId.value = null
  }

  onUnmounted(() => {
    stopAllAutoExploration()
  })

  const clearLogPanel = () => {
    logRef.value?.clearLogs()
  }
</script>

<style scoped>
  :deep(.n-space) {
    width: 100%;
  }
</style>
