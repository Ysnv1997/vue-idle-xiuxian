<template>
  <n-layout>
    <n-layout-header bordered>
      <n-page-header>
        <template #title>成就系统</template>
      </n-page-header>
    </n-layout-header>
    <n-layout-content>
      <n-card :bordered="false">
        <n-tabs type="line">
          <n-tab-pane
            v-for="category in achievementCategories"
            :key="category.key"
            :name="category.key"
            :tab="category.name"
          >
            <n-space vertical>
              <n-grid :cols="2" :x-gap="12" :y-gap="8">
                <n-grid-item v-for="achievement in category.achievements" :key="achievement.id">
                  <n-card
                    :class="{ completed: isAchievementCompleted(achievement.id) }"
                    size="small"
                    hoverable
                    @click="showAchievementDetails(achievement)"
                  >
                    <template #header>
                      <n-space justify="space-between" align="center">
                        <span>{{ achievement.name }}</span>
                        <n-tag :type="isAchievementCompleted(achievement.id) ? 'success' : 'default'">
                          {{ isAchievementCompleted(achievement.id) ? '已完成' : '未完成' }}
                        </n-tag>
                      </n-space>
                    </template>
                    <p>{{ achievement.description }}</p>
                    <n-progress
                      type="line"
                      :percentage="getProgress(achievement)"
                      :color="isAchievementCompleted(achievement.id) ? '#18a058' : '#2080f0'"
                      :height="8"
                      :border-radius="4"
                      :show-indicator="true"
                    />
                  </n-card>
                </n-grid-item>
              </n-grid>
            </n-space>
          </n-tab-pane>
        </n-tabs>
      </n-card>
    </n-layout-content>
  </n-layout>
</template>

<script setup>
  import { computed, onMounted, ref } from 'vue'
  import { useMessage } from 'naive-ui'

  import { fetchAchievements, syncAchievements } from '../api/modules/achievements'
  import { usePlayerStore } from '../stores/player'

  const categoryNames = {
    equipment: '装备成就',
    dungeon_explore: '秘境探索',
    dungeon_combat: '秘境战斗',
    cultivation: '修炼成就',
    breakthrough: '突破成就',
    exploration: '探索成就',
    collection: '收集成就',
    resources: '资源成就',
    alchemy: '炼丹成就'
  }

  const playerStore = usePlayerStore()
  const message = useMessage()

  const achievementCategories = ref([])

  const achievementStatusMap = computed(() => {
    const map = {}
    achievementCategories.value.forEach(category => {
      ;(category.achievements || []).forEach(achievement => {
        map[achievement.id] = achievement
      })
    })
    return map
  })

  function getCategoryName(category) {
    return categoryNames[category] || '其他成就'
  }

  function applyServerAchievementPayload(payload) {
    const categories = Array.isArray(payload?.categories) ? payload.categories : []
    if (categories.length === 0) {
      achievementCategories.value = []
      return
    }

    achievementCategories.value = categories.map(category => ({
      key: category.key,
      name: category.name || getCategoryName(category.key),
      achievements: Array.isArray(category.achievements) ? category.achievements : []
    }))
  }

  async function loadServerAchievements(syncFirst = false) {
    const response = syncFirst ? await syncAchievements() : await fetchAchievements()

    if (response?.snapshot) {
      playerStore.applyServerSnapshot(response.snapshot)
    }

    if (response?.achievements) {
      applyServerAchievementPayload(response.achievements)
    } else {
      applyServerAchievementPayload(response)
    }

    return response
  }

  // 检查成就完成情况
  onMounted(async () => {
    try {
      const syncResult = await loadServerAchievements(true)
      ;(syncResult?.newlyCompleted || []).forEach(achievement => {
        message.success(`恭喜解锁新成就：${achievement.name}！\n\n${achievement.description}`, { duration: 3000 })
      })
    } catch (error) {
      console.error('同步成就失败:', error)
      try {
        await loadServerAchievements(false)
      } catch (listError) {
        console.error('加载成就列表失败:', listError)
        message.error('加载成就失败，请稍后重试')
      }
    }
  })

  // 检查成就是否完成
  const isAchievementCompleted = achievementId => {
    return Boolean(achievementStatusMap.value[achievementId]?.completed)
  }

  // 显示成就详情
  const showAchievementDetails = achievement => {
    let rewardText = '奖励：'
    if (achievement.reward) {
      if (achievement.reward.spirit) rewardText += `\n${achievement.reward.spirit} 灵力`
      if (achievement.reward.spiritRate)
        rewardText += `\n${(achievement.reward.spiritRate * 100 - 100).toFixed(0)}% 灵力获取提升`
      if (achievement.reward.herbRate)
        rewardText += `\n${(achievement.reward.herbRate * 100 - 100).toFixed(0)}% 灵草获取提升`
      if (achievement.reward.alchemyRate)
        rewardText += `\n${(achievement.reward.alchemyRate * 100 - 100).toFixed(0)}% 炼丹成功率提升`
      if (achievement.reward.luck) rewardText += `\n${(achievement.reward.luck * 100 - 100).toFixed(0)}% 幸运提升`
    }
    message.info(`${achievement.name}\n\n${achievement.description}\n\n${rewardText}`, { duration: 5000 })
  }

  // 获取成就进度
  const getProgress = achievement => {
    try {
      const progress = Number(achievementStatusMap.value[achievement.id]?.progress ?? 0)
      return Number.isFinite(progress) ? Math.min(100, Math.max(0, Math.round(progress))) : 0
    } catch (error) {
      console.error('成就进度报错:', error)
      return 0
    }
  }
</script>

<style scoped>
  .completed {
    background-color: rgba(24, 160, 88, 0.1);
  }
</style>
