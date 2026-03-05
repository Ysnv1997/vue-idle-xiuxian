<template>
  <div class="ranking-container">
    <n-card title="排行榜">
      <template #header-extra>
        <n-space>
          <n-select v-model:value="rankingType" :options="rankingOptions" style="width: 160px" :disabled="loading" />
          <n-select v-model:value="rankingScope" :options="scopeOptions" style="width: 140px" :disabled="loading" />
          <n-input-number v-model:value="limit" :min="10" :max="100" :step="10" style="width: 100px" :disabled="loading" />
          <n-button type="primary" :loading="loading" @click="refreshAll">刷新</n-button>
        </n-space>
      </template>
      <n-space vertical>
        <n-card size="small" title="我的关注">
          <n-spin :show="loadingFollows">
            <n-empty v-if="follows.length === 0" description="暂无关注，去全服榜点击关注即可加入好友榜。" />
            <div v-else class="follow-list">
              <div v-for="follow in follows" :key="follow.userId" class="follow-item">
                <span>{{ follow.name }} · {{ follow.realm }}</span>
                <n-button
                  size="tiny"
                  tertiary
                  :loading="followSubmittingUserId === follow.userId"
                  @click="unfollowById(follow.userId)"
                >
                  取消关注
                </n-button>
              </div>
            </div>
          </n-spin>
        </n-card>

        <n-card size="small" title="我的排名" v-if="selfEntry">
          <n-descriptions :column="4" bordered size="small">
            <n-descriptions-item label="排名">#{{ selfEntry.rank }}</n-descriptions-item>
            <n-descriptions-item label="道号">{{ selfEntry.name }}</n-descriptions-item>
            <n-descriptions-item label="境界">{{ selfEntry.realm }}</n-descriptions-item>
            <n-descriptions-item :label="valueLabel">{{ formatValue(selfEntry.value) }}</n-descriptions-item>
          </n-descriptions>
        </n-card>
        <n-empty v-else description="暂无个人排行数据" />

        <n-spin :show="loading">
          <n-table striped size="small">
            <thead>
              <tr>
                <th style="width: 80px">排名</th>
                <th>道号</th>
                <th style="width: 120px">境界</th>
                <th style="width: 180px">{{ valueLabel }}</th>
                <th style="width: 120px">操作</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="entry in entries" :key="entry.userId">
                <td>#{{ entry.rank }}</td>
                <td>{{ entry.name }}</td>
                <td>{{ entry.realm }}</td>
                <td>{{ formatValue(entry.value) }}</td>
                <td>
                  <n-button
                    v-if="canToggleFollow(entry.userId)"
                    size="tiny"
                    :type="isFollowed(entry.userId) ? 'default' : 'primary'"
                    :loading="followSubmittingUserId === entry.userId"
                    @click="toggleFollow(entry.userId)"
                  >
                    {{ isFollowed(entry.userId) ? '取关' : '关注' }}
                  </n-button>
                  <span v-else>-</span>
                </td>
              </tr>
              <tr v-if="entries.length === 0">
                <td colspan="5">
                  <n-empty description="暂无排行数据" />
                </td>
              </tr>
            </tbody>
          </n-table>
        </n-spin>
      </n-space>
    </n-card>
  </div>
</template>

<script setup>
  import { computed, onMounted, ref, watch } from 'vue'
  import { useMessage } from 'naive-ui'
  import {
    fetchRankingFollows,
    fetchRankings,
    followRankingUser,
    unfollowRankingUser
  } from '../api/modules/ranking'
  import { useSessionStore } from '../stores/session'

  const message = useMessage()
  const sessionStore = useSessionStore()

  const loading = ref(false)
  const loadingFollows = ref(false)
  const followSubmittingUserId = ref('')

  const rankingType = ref('realm')
  const rankingScope = ref('global')
  const limit = ref(50)

  const entries = ref([])
  const selfEntry = ref(null)
  const follows = ref([])

  const rankingOptions = [
    { label: '境界榜', value: 'realm' },
    { label: '修为榜', value: 'cultivation' },
    { label: '战力榜', value: 'power' },
    { label: '秘境榜', value: 'dungeon' },
    { label: '财富榜', value: 'wealth' }
  ]

  const scopeOptions = [
    { label: '全服榜', value: 'global' },
    { label: '好友榜', value: 'friends' }
  ]

  const currentUserId = computed(() => String(selfEntry.value?.userId || sessionStore.user?.id || ''))
  const followedUserIdSet = computed(() => new Set(follows.value.map(item => String(item?.userId || ''))))

  const valueLabel = computed(() => {
    switch (rankingType.value) {
      case 'cultivation':
        return '修为'
      case 'power':
        return '战力'
      case 'dungeon':
        return '最高层'
      case 'wealth':
        return '灵石'
      default:
        return '境界值'
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
      if (error?.payload?.error === 'invalid ranking type') {
        message.error('排行榜类型无效')
      } else if (error?.payload?.error === 'invalid ranking scope') {
        message.error('排行榜范围无效')
      } else {
        message.error(error?.message || '加载排行榜失败')
      }
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
      message.error(error?.message || '加载关注列表失败')
    } finally {
      loadingFollows.value = false
    }
  }

  const refreshAll = async () => {
    await Promise.all([loadRankings(), loadFollows()])
  }

  const isFollowed = userId => {
    return followedUserIdSet.value.has(String(userId || ''))
  }

  const canToggleFollow = userId => {
    const normalized = String(userId || '')
    if (!normalized) return false
    return normalized !== currentUserId.value
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
        message.success('关注成功')
      }
      await loadFollows()
      if (rankingScope.value === 'friends') {
        await loadRankings()
      }
    } catch (error) {
      message.error(error?.message || '关注操作失败')
    } finally {
      followSubmittingUserId.value = ''
    }
  }

  const unfollowById = async userId => {
    const normalized = String(userId || '')
    if (!normalized) return

    followSubmittingUserId.value = normalized
    try {
      await unfollowRankingUser(normalized)
      message.success('已取消关注')
      await loadFollows()
      if (rankingScope.value === 'friends') {
        await loadRankings()
      }
    } catch (error) {
      message.error(error?.message || '取消关注失败')
    } finally {
      followSubmittingUserId.value = ''
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
  .ranking-container {
    margin: 0 auto;
  }

  .follow-list {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .follow-item {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    padding: 6px 0;
    border-bottom: 1px solid rgba(127, 127, 127, 0.15);
  }

  .follow-item:last-child {
    border-bottom: none;
  }
</style>
