<template>
  <n-config-provider :theme="playerStore.isDarkMode ? darkTheme : null">
    <n-message-provider>
      <n-dialog-provider>
        <n-spin :show="isLoading" description="正在加载游戏数据...">
          <template v-if="showGameShell">
            <n-layout class="app-shell">
              <n-layout-header bordered class="app-header">
                <div class="header-wrap">
                  <div class="brand-area">
                    <p class="brand-eyebrow">修真总览</p>
                    <h1 class="brand-title">修仙大世界</h1>
                  </div>

                  <div class="header-right">
                    <div class="resource-ribbon">
                      <div class="stat-chip">
                        <span>境界</span>
                        <strong>{{ currentRealmName }}</strong>
                      </div>
                      <div class="stat-chip">
                        <span>灵石</span>
                        <strong>{{ formatNumber(playerStore.spiritStones) }}</strong>
                      </div>
                      <div class="stat-chip">
                        <span>灵力</span>
                        <strong>{{ playerStore.spirit.toFixed(2) }}</strong>
                      </div>
                    </div>

                    <n-button quaternary circle class="theme-switch" @click="playerStore.toggle">
                      <template #icon>
                        <n-icon>
                          <Sunny v-if="playerStore.isDarkMode" />
                          <Moon v-else />
                        </n-icon>
                      </template>
                    </n-button>
                  </div>
                </div>
              </n-layout-header>

              <n-layout-content class="app-content">
                <div class="workspace">
                  <aside class="side-column panel-enter-left">
                    <n-card :bordered="false" class="cultivator-card">
                      <div class="cultivator-head">
                        <div class="name-seal">{{ playerInitial }}</div>
                        <div>
                          <h2>{{ playerStore.name }}</h2>
                          <p>{{ currentRealmName }}</p>
                        </div>
                      </div>

                      <div class="progress-meta">
                        <span>当前修为</span>
                        <strong>{{ formatNumber(playerStore.cultivation) }} / {{ formatNumber(playerStore.maxCultivation) }}</strong>
                      </div>
                      <n-progress
                        type="line"
                        :percentage="cultivationPercent"
                        indicator-placement="inside"
                        :show-indicator="true"
                        processing
                        color="var(--accent-primary)"
                        rail-color="var(--accent-muted)"
                      />

                      <div class="quick-grid">
                        <div class="quick-item">
                          <span>强化石</span>
                          <strong>{{ formatNumber(playerStore.reinforceStones) }}</strong>
                        </div>
                        <div class="quick-item">
                          <span>洗练石</span>
                          <strong>{{ formatNumber(playerStore.refinementStones) }}</strong>
                        </div>
                        <div class="quick-item">
                          <span>攻击</span>
                          <strong>{{ (playerStore.baseAttributes.attack || 0).toFixed(0) }}</strong>
                        </div>
                        <div class="quick-item">
                          <span>生命</span>
                          <strong>{{ (playerStore.baseAttributes.health || 0).toFixed(0) }}</strong>
                        </div>
                      </div>

                      <n-collapse arrow-placement="right" class="detail-collapse">
                        <n-collapse-item title="战斗参数" name="combat">
                          <n-descriptions :column="2" bordered size="small">
                            <n-descriptions-item label="暴击率">
                              {{ (playerStore.combatAttributes.critRate * 100).toFixed(1) }}%
                            </n-descriptions-item>
                            <n-descriptions-item label="闪避率">
                              {{ (playerStore.combatAttributes.dodgeRate * 100).toFixed(1) }}%
                            </n-descriptions-item>
                            <n-descriptions-item label="连击率">
                              {{ (playerStore.combatAttributes.comboRate * 100).toFixed(1) }}%
                            </n-descriptions-item>
                            <n-descriptions-item label="吸血率">
                              {{ (playerStore.combatAttributes.vampireRate * 100).toFixed(1) }}%
                            </n-descriptions-item>
                          </n-descriptions>
                        </n-collapse-item>
                      </n-collapse>
                    </n-card>

                    <n-card :bordered="false" class="nav-card" title="山门导航">
                      <n-scrollbar :x-scrollable="isCompact" trigger="none" class="nav-scroll">
                        <n-menu
                          :mode="isCompact ? 'horizontal' : 'vertical'"
                          :options="menuOptions"
                          :value="getCurrentMenuKey()"
                          @update:value="handleMenuClick"
                        />
                      </n-scrollbar>
                    </n-card>
                  </aside>

                  <main class="stage-column panel-enter-right">
                    <div class="stage-shell">
                      <router-view />
                    </div>
                  </main>
                </div>
              </n-layout-content>
            </n-layout>
            <global-chat-dock v-if="showGlobalChatDock" />
          </template>
          <main v-else class="auth-shell">
            <router-view />
          </main>
        </n-spin>
      </n-dialog-provider>
    </n-message-provider>
  </n-config-provider>
</template>

<script setup>
  import { computed, h, onMounted, onUnmounted, ref, watch } from 'vue'
  import { useRouter, useRoute } from 'vue-router'
  import { NIcon, darkTheme } from 'naive-ui'
  import {
    AppstoreOutlined,
    BookOutlined,
    CompassOutlined,
    ExperimentOutlined,
    GiftOutlined,
    HomeOutlined,
    MedicineBoxOutlined,
    SettingOutlined,
    TrophyOutlined
  } from '@ant-design/icons-vue'
  import { Moon, Sunny, Flash } from '@vicons/ionicons5'

  import GlobalChatDock from './components/GlobalChatDock.vue'
  import { getRealmName } from './plugins/realm'
  import { usePlayerStore } from './stores/player'
  import { useSessionStore } from './stores/session'

  const router = useRouter()
  const route = useRoute()
  const playerStore = usePlayerStore()
  const sessionStore = useSessionStore()

  const menuOptions = ref([])
  const isNewPlayer = ref(false)
  const isLoading = ref(true)
  const isCompact = ref(false)

  const snapshotSyncIntervalMs = 3000
  let snapshotTimer = null

  const currentRealmName = computed(() => getRealmName(playerStore.level).name)
  const playerInitial = computed(() => {
    const source = String(playerStore.name || '').trim()
    return source ? source.slice(0, 1) : '修'
  })
  const cultivationPercent = computed(() => {
    const max = Number(playerStore.maxCultivation || 1)
    if (max <= 0) return 0
    return Number(((Number(playerStore.cultivation || 0) / max) * 100).toFixed(2))
  })
  const showGameShell = computed(() => sessionStore.isAuthenticated)
  const showGlobalChatDock = computed(() => showGameShell.value && !isLoading.value)

  const formatNumber = value => Number(value || 0).toLocaleString()

  const renderIcon = icon => {
    return () => h(NIcon, null, { default: () => h(icon) })
  }

  const getMenuOptions = () => {
    menuOptions.value = [
      ...(isNewPlayer.value
        ? [
            {
              label: '欢迎',
              key: '',
              icon: renderIcon(HomeOutlined)
            }
          ]
        : []),
      {
        label: '修炼',
        key: 'cultivation',
        icon: renderIcon(BookOutlined)
      },
      {
        label: '背包',
        key: 'inventory',
        icon: renderIcon(ExperimentOutlined)
      },
      {
        label: '抽奖',
        key: 'gacha',
        icon: renderIcon(GiftOutlined)
      },
      {
        label: '炼丹',
        key: 'alchemy',
        icon: renderIcon(MedicineBoxOutlined)
      },
      {
        label: '探索',
        key: 'exploration',
        icon: renderIcon(CompassOutlined)
      },
      {
        label: '秘境',
        key: 'dungeon',
        icon: renderIcon(Flash)
      },
      {
        label: '成就',
        key: 'achievements',
        icon: renderIcon(TrophyOutlined)
      },
      {
        label: '排行',
        key: 'ranking',
        icon: renderIcon(TrophyOutlined)
      },
      {
        label: '拍卖',
        key: 'auction',
        icon: renderIcon(AppstoreOutlined)
      },
      {
        label: '充值',
        key: 'recharge',
        icon: renderIcon(AppstoreOutlined)
      },
      {
        label: '设置',
        key: 'settings',
        icon: renderIcon(SettingOutlined)
      }
    ]
  }

  const getCurrentMenuKey = () => {
    return route.path.slice(1)
  }

  const handleMenuClick = key => {
    router.push(`/${key}`)
  }

  const startSnapshotPolling = () => {
    if (snapshotTimer) return
    snapshotTimer = setInterval(() => {
      playerStore.refreshSnapshot()
    }, snapshotSyncIntervalMs)
  }

  const stopSnapshotPolling = () => {
    if (!snapshotTimer) return
    clearInterval(snapshotTimer)
    snapshotTimer = null
  }

  const syncViewportMode = () => {
    isCompact.value = window.innerWidth < 1080
  }

  const bootstrapGame = async () => {
    try {
      await sessionStore.initializeSession()
    } catch (error) {
      console.error('初始化会话失败:', error)
    }

    if (sessionStore.isAuthenticated) {
      try {
        await playerStore.initializePlayer()
      } finally {
        isNewPlayer.value = playerStore.isNewPlayer
        getMenuOptions()
      }
    } else {
      menuOptions.value = []
      isNewPlayer.value = true
    }

    isLoading.value = false
  }

  bootstrapGame()

  watch(
    () => playerStore.isNewPlayer,
    bool => {
      if (!sessionStore.isAuthenticated) return
      isNewPlayer.value = bool
      getMenuOptions()
      if (!bool && route.path === '/') {
        router.push('/cultivation')
      }
    }
  )

  watch(
    () => sessionStore.isAuthenticated,
    authed => {
      if (!authed) {
        stopSnapshotPolling()
        menuOptions.value = []
        if (route.path !== '/' && route.path !== '/auth/callback') {
          router.replace('/')
        }
        return
      }

      getMenuOptions()
      if (route.path === '/') {
        router.replace('/cultivation')
      }
      startSnapshotPolling()
    },
    { immediate: true }
  )

  onMounted(() => {
    syncViewportMode()
    window.addEventListener('resize', syncViewportMode, { passive: true })
  })

  onUnmounted(() => {
    window.removeEventListener('resize', syncViewportMode)
    stopSnapshotPolling()
  })
</script>

<style>
  * {
    margin: 0;
    padding: 0;
    box-sizing: border-box;
  }

  :root {
    --bg-a: #f5efe3;
    --bg-b: #d8e4ef;
    --bg-c: #f4d8a6;
    --ink-main: #2d2a26;
    --ink-sub: #6b5f50;
    --panel-bg: rgba(255, 252, 245, 0.72);
    --panel-border: rgba(164, 140, 107, 0.24);
    --accent-primary: #2f6b6d;
    --accent-muted: rgba(47, 107, 109, 0.18);
    --font-display: 'STKaiti', 'KaiTi', 'Noto Serif SC', serif;
    --font-body: 'PingFang SC', 'Hiragino Sans GB', 'Microsoft YaHei', sans-serif;
  }

  html,
  body,
  #app {
    min-height: 100%;
  }

  body {
    font-family: var(--font-body);
    color: var(--ink-main);
    background:
      radial-gradient(circle at 18% 15%, rgba(244, 216, 166, 0.55), transparent 42%),
      radial-gradient(circle at 86% 8%, rgba(160, 198, 214, 0.4), transparent 38%),
      linear-gradient(145deg, var(--bg-a), var(--bg-b) 45%, #e9eef4 100%);
    background-attachment: fixed;
  }

  body::before {
    content: '';
    position: fixed;
    inset: 0;
    pointer-events: none;
    background-image:
      linear-gradient(rgba(255, 255, 255, 0.11) 1px, transparent 1px),
      linear-gradient(90deg, rgba(255, 255, 255, 0.11) 1px, transparent 1px);
    background-size: 36px 36px;
    opacity: 0.3;
    z-index: -1;
  }

  html.dark {
    --bg-a: #101822;
    --bg-b: #1a2533;
    --bg-c: #2a3a4f;
    --ink-main: #e8edf3;
    --ink-sub: #9ab0c6;
    --panel-bg: rgba(15, 23, 34, 0.72);
    --panel-border: rgba(132, 162, 184, 0.22);
    --accent-primary: #6ab2b5;
    --accent-muted: rgba(106, 178, 181, 0.22);
  }

  .n-config-provider,
  .app-shell {
    min-height: 100vh;
  }

  .auth-shell {
    min-height: 100vh;
    max-width: 960px;
    margin: 0 auto;
    padding: 24px 16px;
  }

  .app-header {
    backdrop-filter: blur(12px);
    background: color-mix(in srgb, var(--panel-bg) 84%, transparent);
    border-bottom-color: var(--panel-border) !important;
  }

  .header-wrap {
    max-width: 1440px;
    margin: 0 auto;
    padding: 14px 20px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 16px;
  }

  .brand-area {
    min-width: 220px;
  }

  .brand-eyebrow {
    font-size: 12px;
    letter-spacing: 0.3em;
    text-transform: uppercase;
    color: var(--ink-sub);
    margin-bottom: 4px;
  }

  .brand-title {
    font-family: var(--font-display);
    font-size: 28px;
    letter-spacing: 0.06em;
    color: var(--ink-main);
    line-height: 1.1;
  }

  .header-right {
    display: flex;
    align-items: center;
    gap: 14px;
    flex: 1;
    justify-content: flex-end;
  }

  .resource-ribbon {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    justify-content: flex-end;
    gap: 10px;
  }

  .stat-chip {
    min-width: 118px;
    padding: 8px 12px;
    border-radius: 14px;
    background: color-mix(in srgb, var(--panel-bg) 90%, transparent);
    border: 1px solid var(--panel-border);
    display: flex;
    flex-direction: column;
    gap: 3px;
  }

  .stat-chip span {
    font-size: 12px;
    color: var(--ink-sub);
  }

  .stat-chip strong {
    font-size: 14px;
    color: var(--ink-main);
  }

  .theme-switch {
    border: 1px solid var(--panel-border);
    background: color-mix(in srgb, var(--panel-bg) 88%, transparent);
  }

  .app-content {
    padding: 18px;
  }

  .workspace {
    max-width: 1440px;
    margin: 0 auto;
    display: grid;
    grid-template-columns: 340px minmax(0, 1fr);
    gap: 18px;
    align-items: start;
  }

  .side-column,
  .stage-column {
    min-width: 0;
  }

  .cultivator-card,
  .nav-card,
  .stage-shell {
    border-radius: 18px !important;
    border: 1px solid var(--panel-border);
    background: var(--panel-bg) !important;
    backdrop-filter: blur(10px);
    box-shadow: 0 14px 40px rgba(40, 31, 22, 0.08);
  }

  .cultivator-card,
  .nav-card {
    margin-bottom: 14px;
  }

  .cultivator-head {
    display: flex;
    align-items: center;
    gap: 12px;
    margin-bottom: 14px;
  }

  .name-seal {
    width: 46px;
    height: 46px;
    border-radius: 12px;
    display: grid;
    place-items: center;
    font-family: var(--font-display);
    font-size: 24px;
    color: #fff;
    background: linear-gradient(145deg, #c6853e, #9b5d26);
    box-shadow: inset 0 1px 2px rgba(255, 255, 255, 0.3);
  }

  .cultivator-head h2 {
    font-family: var(--font-display);
    font-size: 24px;
    line-height: 1.1;
    color: var(--ink-main);
  }

  .cultivator-head p {
    margin-top: 4px;
    font-size: 13px;
    color: var(--ink-sub);
  }

  .progress-meta {
    margin-bottom: 8px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 10px;
    font-size: 13px;
    color: var(--ink-sub);
  }

  .progress-meta strong {
    color: var(--ink-main);
    font-size: 14px;
  }

  .quick-grid {
    margin-top: 14px;
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 10px;
  }

  .quick-item {
    border-radius: 12px;
    border: 1px dashed var(--panel-border);
    padding: 8px 10px;
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .quick-item span {
    font-size: 12px;
    color: var(--ink-sub);
  }

  .quick-item strong {
    font-size: 14px;
    color: var(--ink-main);
  }

  .detail-collapse {
    margin-top: 14px;
  }

  .nav-card :deep(.n-card-header__main) {
    font-family: var(--font-display);
    letter-spacing: 0.08em;
  }

  .nav-scroll {
    max-width: 100%;
  }

  .stage-shell {
    padding: 2px;
    min-height: calc(100vh - 170px);
    animation: rise-in 0.68s ease 0.12s both;
  }

  .panel-enter-left {
    animation: rise-in 0.56s ease 0.02s both;
  }

  .panel-enter-right {
    animation: rise-in 0.7s ease 0.08s both;
  }

  .page-view {
    padding: 18px;
    display: flex;
    flex-direction: column;
    gap: 14px;
  }

  .page-head {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .page-head h2 {
    font-size: 26px;
    line-height: 1.1;
    letter-spacing: 0.04em;
    font-family: var(--font-display);
  }

  .page-eyebrow {
    font-size: 12px;
    letter-spacing: 0.28em;
    color: var(--ink-sub);
  }

  .page-desc {
    color: var(--ink-sub);
  }

  .page-card {
    border-radius: 16px !important;
  }

  @keyframes rise-in {
    from {
      opacity: 0;
      transform: translateY(16px);
    }
    to {
      opacity: 1;
      transform: translateY(0);
    }
  }

  ::-webkit-scrollbar {
    width: 11px;
    height: 11px;
  }

  ::-webkit-scrollbar-track {
    background-color: rgba(0, 0, 0, 0.06);
  }

  ::-webkit-scrollbar-thumb {
    background-color: rgba(0, 0, 0, 0.26);
    border-radius: 8px;
    border: 2px solid transparent;
    background-clip: padding-box;
  }

  html.dark ::-webkit-scrollbar-track {
    background-color: rgba(255, 255, 255, 0.06);
  }

  html.dark ::-webkit-scrollbar-thumb {
    background-color: rgba(255, 255, 255, 0.28);
  }

  @media (max-width: 1280px) {
    .workspace {
      grid-template-columns: 320px minmax(0, 1fr);
    }

    .brand-title {
      font-size: 24px;
    }
  }

  @media (max-width: 1080px) {
    .app-content {
      padding: 14px;
    }

    .workspace {
      grid-template-columns: 1fr;
      gap: 14px;
    }

    .header-wrap {
      flex-direction: column;
      align-items: flex-start;
      gap: 12px;
    }

    .header-right {
      width: 100%;
      justify-content: space-between;
    }

    .resource-ribbon {
      justify-content: flex-start;
    }

    .stage-shell {
      min-height: 0;
    }
  }

  @media (max-width: 720px) {
    .app-content {
      padding: 10px;
    }

    .header-wrap {
      padding: 10px 12px;
    }

    .brand-title {
      font-size: 22px;
    }

    .stat-chip {
      min-width: 102px;
    }

    .quick-grid {
      grid-template-columns: 1fr;
    }
  }

  @media (max-width: 768px) {
    .page-view {
      padding: 12px;
      gap: 12px;
    }
  }
</style>
