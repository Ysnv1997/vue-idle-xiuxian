<template>
  <div class="page-view ranking-page">
    <!-- 顶部标题与快速切换 -->
    <header class="page-head">
      <div class="head-main">
        <p class="page-eyebrow">天机阁 · 众仙榜</p>
        <h2 class="page-title">天阶排行</h2>
      </div>
      <div class="head-actions">
        <n-button-group round>
          <n-button 
            v-for="scope in scopeOptions" 
            :key="scope.value"
            :secondary="rankingScope !== scope.value"
            :type="rankingScope === scope.value ? 'primary' : 'default'"
            @click="rankingScope = scope.value"
          >
            {{ scope.label }}
          </n-button>
        </n-button-group>
        <n-button quaternary circle @click="refreshAll" :loading="loading">
          <template #icon><n-icon><RefreshOutline /></n-icon></template>
        </n-button>
      </div>
    </header>

    <!-- 榜单分类 Tabs -->
    <n-tabs v-model:value="rankingType" type="segment" animated class="ranking-tabs">
      <n-tab-pane v-for="opt in rankingOptions" :key="opt.value" :name="opt.value" :tab="opt.label" />
    </n-tabs>

    <div class="ranking-container">
      <n-spin :show="loading">
        <div class="ranking-content">
          <!-- 前三名：仙榜巅峰 (Podium) -->
          <section class="podium-section" v-if="topThree.length > 0">
            <!-- 榜眼 (Rank 2) -->
            <div class="podium-card rank-2" v-if="topThree[1]">
              <div class="podium-crown">🥈</div>
              <div class="podium-avatar" @click="openProfile(topThree[1].userId)">
                <div class="avatar-inner">{{ topThree[1].name[0] }}</div>
              </div>
              <div class="podium-info">
                <div class="p-name">{{ topThree[1].name }}</div>
                <div class="p-realm">{{ topThree[1].realm }}</div>
                <div class="p-value">{{ formatValue(topThree[1].value) }} {{ valueLabel }}</div>
              </div>
              <n-button 
                v-if="canToggleFollow(topThree[1].userId)" 
                size="tiny" round tertiary
                :type="isFollowed(topThree[1].userId) ? 'primary' : 'default'"
                @click="toggleFollow(topThree[1].userId)"
              >
                {{ isFollowed(topThree[1].userId) ? '已关注' : '关注' }}
              </n-button>
            </div>

            <!-- 状元 (Rank 1) -->
            <div class="podium-card rank-1" v-if="topThree[0]">
              <div class="podium-crown">🥇</div>
              <div class="podium-avatar" @click="openProfile(topThree[0].userId)">
                <div class="avatar-inner">{{ topThree[0].name[0] }}</div>
              </div>
              <div class="podium-info">
                <div class="p-name">{{ topThree[0].name }}</div>
                <div class="p-realm">{{ topThree[0].realm }}</div>
                <div class="p-value">{{ formatValue(topThree[0].value) }} {{ valueLabel }}</div>
              </div>
              <n-button 
                v-if="canToggleFollow(topThree[0].userId)" 
                size="tiny" round 
                :type="isFollowed(topThree[0].userId) ? 'primary' : 'default'"
                @click="toggleFollow(topThree[0].userId)"
              >
                {{ isFollowed(topThree[0].userId) ? '已关注' : '关注' }}
              </n-button>
            </div>

            <!-- 探花 (Rank 3) -->
            <div class="podium-card rank-3" v-if="topThree[2]">
              <div class="podium-crown">🥉</div>
              <div class="podium-avatar" @click="openProfile(topThree[2].userId)">
                <div class="avatar-inner">{{ topThree[2].name[0] }}</div>
              </div>
              <div class="podium-info">
                <div class="p-name">{{ topThree[2].name }}</div>
                <div class="p-realm">{{ topThree[2].realm }}</div>
                <div class="p-value">{{ formatValue(topThree[2].value) }} {{ valueLabel }}</div>
              </div>
              <n-button 
                v-if="canToggleFollow(topThree[2].userId)" 
                size="tiny" round tertiary
                :type="isFollowed(topThree[2].userId) ? 'primary' : 'default'"
                @click="toggleFollow(topThree[2].userId)"
              >
                {{ isFollowed(topThree[2].userId) ? '已关注' : '关注' }}
              </n-button>
            </div>
          </section>

          <!-- 其它排名列表 -->
          <section class="rank-list-section">
            <div class="list-header">
              <span>位次</span>
              <span>道号 / 境界</span>
              <span class="text-right">{{ valueLabel }}</span>
              <span class="text-center">结交</span>
            </div>
            <div class="list-body">
              <div 
                v-for="entry in otherEntries" 
                :key="entry.userId" 
                class="rank-row"
                :class="{ 'is-self': entry.userId === currentUserId }"
              >
                <div class="r-index">#{{ entry.rank }}</div>
                <div class="r-main" @click="openProfile(entry.userId)">
                  <div class="r-name">{{ entry.name }}</div>
                  <div class="r-realm">{{ entry.realm }}</div>
                </div>
                <div class="r-value">{{ formatValue(entry.value) }}</div>
                <div class="r-action">
                  <n-button
                    v-if="canToggleFollow(entry.userId)"
                    size="small" circle quaternary
                    @click="toggleFollow(entry.userId)"
                  >
                    <template #icon>
                      <n-icon :color="isFollowed(entry.userId) ? '#d03050' : ''">
                        <Heart v-if="isFollowed(entry.userId)" />
                        <HeartOutline v-else />
                      </n-icon>
                    </template>
                  </n-button>
                </div>
              </div>
              <n-empty v-if="entries.length === 0" description="天机不可泄露 (暂无数据)" />
            </div>
          </section>
        </div>
      </n-spin>
    </div>

    <!-- 底部固定：我的排名 -->
    <footer class="my-rank-footer" v-if="selfEntry">
      <div class="my-rank-card" @click="openProfile(selfEntry.userId)">
        <div class="m-index">#{{ selfEntry.rank }}</div>
        <div class="m-info">
          <div class="m-name">我的排名：{{ selfEntry.name }}</div>
          <div class="m-realm">{{ selfEntry.realm }}</div>
        </div>
        <div class="m-value">
          <span class="label">{{ valueLabel }}</span>
          <strong>{{ formatValue(selfEntry.value) }}</strong>
        </div>
      </div>
    </footer>

    <!-- 关注列表抽屉 -->
    <n-drawer v-model:show="showFollows" :width="320" placement="right">
      <n-drawer-content title="同道中人" closable>
        <n-spin :show="loadingFollows">
          <div class="follow-drawer-list">
            <div v-for="follow in follows" :key="follow.userId" class="follow-card">
              <div class="f-info" @click="openProfile(follow.userId)">
                <div class="f-name">{{ follow.name }}</div>
                <div class="f-realm">{{ follow.realm }}</div>
              </div>
              <n-button size="tiny" quaternary type="error" @click="unfollowById(follow.userId)">取关</n-button>
            </div>
            <n-empty v-if="follows.length === 0" description="独行侠" />
          </div>
        </n-spin>
      </n-drawer-content>
    </n-drawer>

    <!-- 关注入口按钮 -->
    <div class="follow-fab" @click="showFollows = true">
      <n-badge :value="follows.length" :max="99">
        <n-button type="primary" circle size="large">
          <template #icon><n-icon><PeopleOutline /></n-icon></template>
        </n-button>
      </n-badge>
    </div>

    <player-profile-dialog v-model:show="showProfileDialog" :loading="profileLoading" :profile="selectedProfile" />
  </div>
</template>

<script setup>
  import { computed, onMounted, ref, watch } from 'vue'
  import { useMessage } from 'naive-ui'
  import { 
    RefreshOutline, 
    HeartOutline, 
    Heart, 
    PeopleOutline,
    ChevronUpOutline,
    ChevronDownOutline,
    HelpCircleOutline
  } from '@vicons/ionicons5'
  import {
    fetchRankingFollows,
    fetchRankings,
    followRankingUser,
    unfollowRankingUser
  } from '../api/modules/ranking'
  import { fetchPublicPlayerProfile } from '../api/modules/player'
  import PlayerProfileDialog from '../components/PlayerProfileDialog.vue'
  import { useSessionStore } from '../stores/session'

  const message = useMessage()
  const sessionStore = useSessionStore()

  const loading = ref(false)
  const loadingFollows = ref(false)
  const followSubmittingUserId = ref('')
  const showFollows = ref(false)

  const rankingType = ref('realm')
  const rankingScope = ref('global')
  const limit = ref(50)

  const entries = ref([])
  const selfEntry = ref(null)
  const follows = ref([])
  const showProfileDialog = ref(false)
  const profileLoading = ref(false)
  const selectedProfile = ref(null)

  const rankingOptions = [
    { label: '境界', value: 'realm' },
    { label: '修为', value: 'cultivation' },
    { label: '战力', value: 'power' },
    { label: '秘境', value: 'dungeon' },
    { label: '财富', value: 'wealth' }
  ]

  const scopeOptions = [
    { label: '全服', value: 'global' },
    { label: '道友', value: 'friends' }
  ]

  const currentUserId = computed(() => String(sessionStore.user?.id || ''))
  const followedUserIdSet = computed(() => new Set(follows.value.map(item => String(item?.userId || ''))))

  const topThree = computed(() => entries.value.slice(0, 3))
  const otherEntries = computed(() => entries.value.slice(3))

  const valueLabel = computed(() => {
    switch (rankingType.value) {
      case 'cultivation': return '修为'
      case 'power': return '战力'
      case 'dungeon': return '层'
      case 'wealth': return '灵石'
      default: return '道行'
    }
  })

  const formatValue = value => Number(value || 0).toLocaleString()

  const loadRankings = async () => {
    try {
      loading.value = true
      const result = await fetchRankings(rankingType.value, limit.value, rankingScope.value)
      entries.value = Array.isArray(result?.entries) ? result.entries : []
      selfEntry.value = result?.self || null
    } catch (error) {
      message.error(error?.message || '加载排行榜失败')
    } finally {
      loading.value = false
    }
  }

  const loadFollows = async () => {
    try {
      loadingFollows.value = true
      const result = await fetchRankingFollows(200)
      follows.value = Array.isArray(result?.follows) ? result.follows : []
    } catch (error) {
      console.error('加载关注列表失败')
    } finally {
      loadingFollows.value = false
    }
  }

  const refreshAll = async () => {
    await Promise.all([loadRankings(), loadFollows()])
  }

  const isFollowed = userId => followedUserIdSet.value.has(String(userId || ''))

  const canToggleFollow = userId => {
    const normalized = String(userId || '')
    return normalized && normalized !== currentUserId.value
  }

  const toggleFollow = async userId => {
    const normalized = String(userId || '')
    if (!canToggleFollow(normalized)) return

    followSubmittingUserId.value = normalized
    try {
      if (isFollowed(normalized)) {
        await unfollowRankingUser(normalized)
        message.success('已取消关注')
      } else {
        await followRankingUser(normalized)
        message.success('已结交为道友')
      }
      await loadFollows()
      if (rankingScope.value === 'friends') await loadRankings()
    } catch (error) {
      message.error('操作失败')
    } finally {
      followSubmittingUserId.value = ''
    }
  }

  const unfollowById = async userId => {
    const normalized = String(userId || '')
    try {
      await unfollowRankingUser(normalized)
      message.success('已取消关注')
      await loadFollows()
      if (rankingScope.value === 'friends') await loadRankings()
    } catch (e) { message.error('操作失败') }
  }

  const openProfile = async userId => {
    const normalized = String(userId || '').trim()
    if (!normalized) return
    showProfileDialog.value = true
    profileLoading.value = true
    selectedProfile.value = null
    try {
      selectedProfile.value = await fetchPublicPlayerProfile(normalized)
    } catch (error) {
      message.error(error?.message || '加载玩家资料失败')
      showProfileDialog.value = false
    } finally {
      profileLoading.value = false
    }
  }

  watch([rankingType, rankingScope], () => {
    loadRankings()
  })

  onMounted(() => {
    refreshAll()
  })
</script>

<style scoped>
.ranking-page {
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
  margin-bottom: 20px;
}

.ranking-tabs { margin-bottom: 24px; }

/* Podium 样式 */
.podium-section {
  display: flex;
  justify-content: center;
  align-items: flex-end;
  gap: 20px;
  margin-bottom: 40px;
  padding: 20px 0;
}

.podium-card {
  background: var(--panel-bg);
  border: 1px solid var(--panel-border);
  border-radius: 24px;
  padding: 24px 16px;
  display: flex;
  flex-direction: column;
  align-items: center;
  text-align: center;
  position: relative;
  transition: transform 0.3s ease;
  width: 180px;
}

.podium-card:hover { transform: translateY(-8px); }

.rank-1 { height: 280px; width: 220px; border-color: #f0a020; box-shadow: 0 12px 40px rgba(240, 160, 32, 0.15); z-index: 2; }
.rank-2 { height: 240px; border-color: #94a3b8; }
.rank-3 { height: 220px; border-color: #c6853e; }

.podium-crown { font-size: 32px; margin-bottom: 8px; }
.podium-avatar {
  width: 80px; height: 80px;
  background: var(--accent-muted);
  border-radius: 50%;
  display: grid;
  place-items: center;
  margin-bottom: 16px;
  cursor: pointer;
  border: 2px solid transparent;
}
.rank-1 .podium-avatar { width: 100px; height: 100px; border-color: #f0a020; }

.avatar-inner { font-family: var(--font-display); font-size: 32px; color: var(--accent-primary); }

.p-name { font-weight: bold; font-size: 18px; margin-bottom: 4px; }
.p-realm { font-size: 12px; color: var(--ink-sub); margin-bottom: 12px; }
.p-value { font-size: 14px; font-weight: bold; color: var(--accent-primary); margin-bottom: 16px; }

/* 列表样式 */
.rank-list-section {
  background: var(--panel-bg);
  border: 1px solid var(--panel-border);
  border-radius: 24px;
  overflow: hidden;
  margin-bottom: 100px;
}

.list-header {
  display: grid;
  grid-template-columns: 80px 1fr 150px 80px;
  padding: 16px 24px;
  background: rgba(0,0,0,0.02);
  font-size: 12px;
  color: var(--ink-sub);
  font-weight: bold;
}

.list-body { display: flex; flex-direction: column; }

.rank-row {
  display: grid;
  grid-template-columns: 80px 1fr 150px 80px;
  padding: 16px 24px;
  align-items: center;
  border-bottom: 1px solid var(--panel-border);
  transition: background 0.2s;
}

.rank-row:hover { background: rgba(0,0,0,0.01); }
.rank-row.is-self { background: var(--accent-muted); }

.r-index { font-family: var(--font-display); font-size: 18px; color: var(--ink-sub); }
.r-main { cursor: pointer; }
.r-name { font-weight: bold; font-size: 15px; }
.r-realm { font-size: 12px; color: var(--ink-sub); }
.r-value { font-weight: bold; text-align: right; font-variant-numeric: tabular-nums; }
.r-action { display: flex; justify-content: center; }

/* 底部我的排名 */
.my-rank-footer {
  position: fixed;
  bottom: 80px;
  left: 50%;
  transform: translateX(-50%);
  width: calc(100% - 40px);
  max-width: 800px;
  z-index: 100;
}

.my-rank-card {
  background: color-mix(in srgb, var(--panel-bg) 95%, transparent);
  backdrop-filter: blur(12px);
  border: 2px solid var(--accent-primary);
  border-radius: 20px;
  padding: 12px 24px;
  display: flex;
  align-items: center;
  gap: 20px;
  box-shadow: 0 10px 30px rgba(0,0,0,0.1);
  cursor: pointer;
}

.m-index { font-family: var(--font-display); font-size: 24px; color: var(--accent-primary); }
.m-info { flex: 1; }
.m-name { font-weight: bold; }
.m-realm { font-size: 12px; color: var(--ink-sub); }
.m-value { text-align: right; }
.m-value .label { font-size: 11px; color: var(--ink-sub); display: block; }
.m-value strong { font-size: 18px; color: var(--accent-primary); }

/* FAB */
.follow-fab {
  position: fixed;
  right: 24px;
  bottom: 160px;
  z-index: 90;
}

.follow-card {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px;
  border-bottom: 1px solid var(--panel-border);
}
.f-info { cursor: pointer; }
.f-name { font-weight: bold; }
.f-realm { font-size: 11px; color: var(--ink-sub); }

.text-right { text-align: right; }
.text-center { text-align: center; }

@media (max-width: 768px) {
  .podium-section { gap: 10px; }
  .podium-card { width: 110px; padding: 16px 8px; }
  .rank-1 { height: 220px; width: 130px; }
  .rank-2 { height: 190px; }
  .rank-3 { height: 170px; }
  .podium-crown { font-size: 20px; }
  .podium-avatar { width: 50px; height: 50px; }
  .rank-1 .podium-avatar { width: 60px; height: 60px; }
  .avatar-inner { font-size: 20px; }
  .p-name { font-size: 14px; }
  .p-realm, .p-value { font-size: 10px; }

  .list-header, .rank-row {
    grid-template-columns: 50px 1fr 100px 50px;
    padding: 12px 16px;
  }
  .r-index { font-size: 14px; }
  .r-name { font-size: 13px; }
  .r-value { font-size: 12px; }
  
  .my-rank-footer { bottom: 90px; }
  .my-rank-card { gap: 10px; padding: 10px 16px; }
  .m-index { font-size: 18px; }
  .m-value strong { font-size: 14px; }
}
</style>
