<template>
  <div class="page-view achievements-page">
    <!-- 顶部标题与总体进度 -->
    <header class="page-head">
      <div class="head-main">
        <p class="page-eyebrow">仙道漫漫 · 步步留痕</p>
        <h2 class="page-title">成就道果</h2>
      </div>
      <div class="overall-stats">
        <div class="stat-item">
          <span class="label">道果完成度</span>
          <div class="value-row">
            <strong>{{ completedCount }} / {{ totalCount }}</strong>
            <n-progress
              type="circle"
              :percentage="totalProgressPercent"
              :stroke-width="10"
              :width="48"
              color="var(--accent-primary)"
            />
          </div>
        </div>
      </div>
    </header>

    <!-- 分类 Tabs -->
    <n-tabs v-model:value="activeCategory" type="segment" animated class="achievement-tabs">
      <n-tab-pane
        v-for="category in achievementCategories"
        :key="category.key"
        :name="category.key"
        :tab="category.name"
      >
        <div class="achievement-grid">
          <div 
            v-for="achievement in category.achievements" 
            :key="achievement.id"
            class="achievement-card"
            :class="{ 'is-completed': isAchievementCompleted(achievement.id) }"
            @click="showAchievementDetails(achievement)"
          >
            <!-- 左侧：勋章图标 -->
            <div class="achievement-badge">
              <div class="badge-icon">
                <n-icon size="32">
                  <TrophyOutline v-if="isAchievementCompleted(achievement.id)" />
                  <Trophy v-else />
                </n-icon>
              </div>
              <div class="badge-shine" v-if="isAchievementCompleted(achievement.id)"></div>
            </div>

            <!-- 中间：详细内容 -->
            <div class="achievement-info">
              <div class="title-row">
                <h3 class="name">{{ achievement.name }}</h3>
                <n-tag 
                  v-if="isAchievementCompleted(achievement.id)" 
                  size="tiny" 
                  type="success" 
                  round 
                  bordered
                >
                  圆满
                </n-tag>
              </div>
              <p class="desc">{{ achievement.description }}</p>
              
              <!-- 奖励预览 -->
              <div class="reward-preview" v-if="achievement.reward">
                <span class="r-label">加成：</span>
                <span class="r-val">{{ getRewardSummary(achievement.reward) }}</span>
              </div>

              <!-- 进度条 -->
              <div class="progress-area">
                <n-progress
                  type="line"
                  :percentage="getProgress(achievement)"
                  :show-indicator="false"
                  :height="6"
                  round
                  :color="isAchievementCompleted(achievement.id) ? '#18a058' : '#2080f0'"
                  rail-color="rgba(0,0,0,0.05)"
                />
                <span class="percent-text">{{ getProgress(achievement) }}%</span>
              </div>
            </div>
          </div>
        </div>
        <n-empty v-if="!category.achievements?.length" description="此道途暂无记载" style="padding: 100px 0" />
      </n-tab-pane>
    </n-tabs>

    <!-- 底部属性总加成面板 -->
    <footer class="achievement-footer" v-if="totalBonusSummary.length > 0">
      <div class="bonus-box">
        <div class="box-title">道果总持加成</div>
        <div class="bonus-grid">
          <div v-for="bonus in totalBonusSummary" :key="bonus.label" class="bonus-chip">
            <span class="b-label">{{ bonus.label }}</span>
            <span class="b-val">+{{ bonus.value }}</span>
          </div>
        </div>
      </div>
    </footer>
  </div>
</template>

<script setup>
  import { computed, onMounted, ref } from 'vue'
  import { useMessage } from 'naive-ui'
  import { TrophyOutline, Trophy, MedalOutline, SparklesOutline } from '@vicons/ionicons5'
  import { fetchAchievements, syncAchievements } from '../api/modules/achievements'
  import { usePlayerStore } from '../stores/player'

  const message = useMessage()
  const playerStore = usePlayerStore()

  const achievementCategories = ref([])
  const activeCategory = ref('')

  const categoryNames = {
    equipment: '神兵',
    dungeon_explore: '禁地',
    dungeon_combat: '试炼',
    cultivation: '修持',
    breakthrough: '晋升',
    exploration: '云游',
    collection: '百宝',
    resources: '财富',
    alchemy: '丹道'
  }

  const achievementStatusMap = computed(() => {
    const map = {}
    achievementCategories.value.forEach(category => {
      ;(category.achievements || []).forEach(achievement => {
        map[achievement.id] = achievement
      })
    })
    return map
  })

  const completedCount = computed(() => {
    let count = 0
    achievementCategories.value.forEach(c => {
      count += (c.achievements || []).filter(a => a.completed).length
    })
    return count
  })

  const totalCount = computed(() => {
    let count = 0
    achievementCategories.value.forEach(c => {
      count += (c.achievements || []).length
    })
    return count || 1
  })

  const totalProgressPercent = computed(() => Math.floor((completedCount.value / totalCount.value) * 100))

  const totalBonusSummary = computed(() => {
    // 汇总所有已完成成就的奖励
    const summary = { spirit: 0, spiritRate: 0, herbRate: 0, alchemyRate: 0, luck: 0 }
    achievementCategories.value.forEach(c => {
      (c.achievements || []).forEach(a => {
        if (a.completed && a.reward) {
          if (a.reward.spirit) summary.spirit += a.reward.spirit
          if (a.reward.spiritRate) summary.spiritRate += (a.reward.spiritRate - 1)
          if (a.reward.herbRate) summary.herbRate += (a.reward.herbRate - 1)
          if (a.reward.alchemyRate) summary.alchemyRate += (a.reward.alchemyRate - 1)
          if (a.reward.luck) summary.luck += (a.reward.luck - 1)
        }
      })
    })

    const result = []
    if (summary.spirit > 0) result.push({ label: '灵力基数', value: summary.spirit })
    if (summary.spiritRate > 0) result.push({ label: '灵力获取', value: (summary.spiritRate * 100).toFixed(0) + '%' })
    if (summary.herbRate > 0) result.push({ label: '采药效率', value: (summary.herbRate * 100).toFixed(0) + '%' })
    if (summary.alchemyRate > 0) result.push({ label: '炼丹成功', value: (summary.alchemyRate * 100).toFixed(0) + '%' })
    if (summary.luck > 0) result.push({ label: '福缘提升', value: (summary.luck * 100).toFixed(0) + '%' })
    return result
  })

  const getRewardSummary = reward => {
    if (!reward) return '无'
    if (reward.spiritRate) return `灵力获取 +${((reward.spiritRate - 1) * 100).toFixed(0)}%`
    if (reward.spirit) return `灵力 +${reward.spirit}`
    if (reward.herbRate) return `灵草效率 +${((reward.herbRate - 1) * 100).toFixed(0)}%`
    return '修为裨益'
  }

  const getCategoryName = key => categoryNames[key] || '其它'

  const applyServerAchievementPayload = payload => {
    const categories = Array.isArray(payload?.categories) ? payload.categories : []
    achievementCategories.value = categories.map(c => ({
      key: c.key,
      name: getCategoryName(c.key),
      achievements: Array.isArray(c.achievements) ? c.achievements : []
    }))
    if (!activeCategory.value && achievementCategories.value.length > 0) {
      activeCategory.value = achievementCategories.value[0].key
    }
  }

  async function loadServerAchievements(syncFirst = false) {
    const response = syncFirst ? await syncAchievements() : await fetchAchievements()
    if (response?.snapshot) playerStore.applyServerSnapshot(response.snapshot)
    applyServerAchievementPayload(response?.achievements || response)
    return response
  }

  onMounted(async () => {
    try {
      const syncResult = await loadServerAchievements(true)
      ;(syncResult?.newlyCompleted || []).forEach(a => {
        message.success(`【道果圆满】\n恭喜达成成就：${a.name}！`, { duration: 4000 })
      })
    } catch (e) {
      await loadServerAchievements(false).catch(() => message.error('加载成就失败'))
    }
  })

  const isAchievementCompleted = id => Boolean(achievementStatusMap.value[id]?.completed)

  const showAchievementDetails = a => {
    let rewardText = '【道果奖励】'
    if (a.reward) {
      if (a.reward.spirit) rewardText += `\n灵力：+${a.reward.spirit}`
      if (a.reward.spiritRate) rewardText += `\n灵力获取：+${((a.reward.spiritRate - 1) * 100).toFixed(0)}%`
      if (a.reward.herbRate) rewardText += `\n灵草获取：+${((a.reward.herbRate - 1) * 100).toFixed(0)}%`
      if (a.reward.luck) rewardText += `\n福缘：+${((a.reward.luck - 1) * 100).toFixed(0)}%`
    }
    message.info(`${a.name}\n\n${a.description}\n\n${rewardText}`, { duration: 5000 })
  }

  const getProgress = a => {
    const p = Number(achievementStatusMap.value[a.id]?.progress ?? 0)
    return Math.min(100, Math.max(0, Math.round(p)))
  }
</script>

<style scoped>
.achievements-page {
  display: flex;
  flex-direction: column;
  height: 100%;
  max-width: 1000px;
  margin: 0 auto;
}

.page-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 24px;
}

.overall-stats .stat-item {
  background: var(--panel-bg);
  border: 1px solid var(--panel-border);
  padding: 12px 24px;
  border-radius: 20px;
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.stat-item .label { font-size: 11px; color: var(--ink-sub); text-transform: uppercase; }
.stat-item .value-row { display: flex; align-items: center; gap: 16px; }
.stat-item strong { font-size: 20px; color: var(--accent-primary); }

.achievement-tabs { margin-bottom: 24px; }

/* 成就网格 */
.achievement-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(400px, 1fr));
  gap: 16px;
}

.achievement-card {
  background: var(--panel-bg);
  border: 1px solid var(--panel-border);
  border-radius: 24px;
  padding: 24px;
  display: flex;
  gap: 20px;
  cursor: pointer;
  transition: all 0.3s ease;
  position: relative;
  overflow: hidden;
}

.achievement-card:hover { transform: translateY(-4px); border-color: var(--accent-primary); box-shadow: 0 12px 32px rgba(0,0,0,0.05); }

.achievement-badge {
  width: 64px; height: 64px;
  background: rgba(0,0,0,0.03);
  border-radius: 16px;
  display: grid; place-items: center;
  flex-shrink: 0;
  position: relative;
  transition: all 0.5s;
}

.badge-icon { color: var(--ink-sub); opacity: 0.3; }

.is-completed .achievement-badge { background: var(--accent-primary); box-shadow: 0 0 20px var(--accent-muted); }
.is-completed .badge-icon { color: white; opacity: 1; transform: scale(1.1); }

.badge-shine {
  position: absolute;
  inset: 0;
  background: linear-gradient(135deg, transparent 0%, rgba(255,255,255,0.4) 50%, transparent 100%);
  animation: shine 3s infinite;
}
@keyframes shine { 0% { transform: translateX(-100%); } 100% { transform: translateX(100%); } }

.achievement-info { flex: 1; display: flex; flex-direction: column; gap: 8px; }

.title-row { display: flex; justify-content: space-between; align-items: center; }
.name { font-size: 18px; font-weight: bold; margin: 0; font-family: var(--font-display); }

.desc { font-size: 13px; color: var(--ink-sub); line-height: 1.5; height: 40px; overflow: hidden; }

.reward-preview { font-size: 11px; background: var(--accent-muted); padding: 4px 10px; border-radius: 6px; align-self: flex-start; }
.r-label { color: var(--accent-primary); font-weight: bold; }

.progress-area { margin-top: 8px; display: flex; align-items: center; gap: 12px; }
.percent-text { font-size: 11px; color: var(--ink-sub); font-variant-numeric: tabular-nums; }

/* 底部加成 */
.achievement-footer { margin-top: 40px; margin-bottom: 100px; }
.bonus-box {
  background: var(--panel-bg);
  border: 1px solid var(--panel-border);
  border-radius: 24px;
  padding: 24px;
}
.box-title { font-size: 14px; font-weight: bold; margin-bottom: 16px; text-align: center; opacity: 0.6; }
.bonus-grid { display: flex; flex-wrap: wrap; justify-content: center; gap: 12px; }
.bonus-chip {
  padding: 8px 16px;
  background: rgba(0,0,0,0.03);
  border: 1px solid var(--panel-border);
  border-radius: 99px;
  display: flex;
  gap: 8px;
  font-size: 13px;
}
.b-label { color: var(--ink-sub); }
.b-val { font-weight: bold; color: var(--accent-primary); }

@media (max-width: 768px) {
  .achievement-grid { grid-template-columns: 1fr; }
  .achievement-card { padding: 16px; gap: 16px; }
  .achievement-badge { width: 48px; height: 48px; }
  .name { font-size: 16px; }
  .desc { font-size: 12px; height: auto; }
}
</style>
