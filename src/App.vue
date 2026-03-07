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
                    <p class="brand-presence">活跃修士：{{ formatNumber(activePlayers) }} 人</p>
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
                        <strong>{{ displaySpirit }}</strong>
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
                <div class="workspace" :class="{ 'is-compact': isCompact }">
                  <aside v-if="!isCompact" class="side-column panel-enter-left">
                    <n-card :bordered="false" class="cultivator-card" @click="showProfileDrawer = true">
                      <div class="cultivator-head">
                        <div class="name-avatar-wrap">
                          <img v-if="linuxDoAvatar" :src="linuxDoAvatar" alt="Avatar" class="linuxdo-avatar" />
                          <div v-else class="name-seal">{{ playerInitial }}</div>
                        </div>
                        <div class="head-info">
                           <h2>{{ playerStore.name }}</h2>
                           <p>{{ currentRealmName }}</p>
                        </div>
                      </div>

                      <div class="progress-meta">
                        <span>修为进度</span>
                        <strong>{{ cultivationPercent }}%</strong>
                      </div>
                      <n-progress
                        type="line"
                        :percentage="cultivationPercent"
                        :show-indicator="false"
                        processing
                        color="var(--accent-primary)"
                        rail-color="var(--accent-muted)"
                      />
                    </n-card>

                    <n-card :bordered="false" class="nav-card">
                      <n-menu
                        mode="vertical"
                        :options="menuOptions"
                        :value="getCurrentMenuKey()"
                        @update:value="handleMenuClick"
                      />
                    </n-card>
                  </aside>

                  <main class="stage-column panel-enter-right">
                    <div class="stage-shell">
                      <router-view />
                    </div>
                  </main>
                </div>
              </n-layout-content>

              <!-- Mobile Navigation Bottom Tabbar -->
              <div v-if="isCompact" class="mobile-tabbar">
                <div 
                  v-for="item in menuOptions.slice(0, 5)" 
                  :key="item.key" 
                  class="tab-item"
                  :class="{ 'active': getCurrentMenuKey() === item.key }"
                  @click="handleMenuClick(item.key)"
                >
                  <component :is="item.icon" />
                  <span>{{ item.label }}</span>
                </div>
                <div class="tab-item" @click="showProfileDrawer = true">
                  <div class="name-seal mini">{{ playerInitial }}</div>
                  <span>我</span>
                </div>
              </div>
            </n-layout>

            <player-profile-drawer 
              v-model:show="showProfileDrawer" 
              :placement="isCompact ? 'bottom' : 'right'" 
            />
            
            <transition name="world-banner">
              <div v-if="activeWorldAnnouncement" class="world-announcement-layer">
                <div class="world-announcement-track" :class="worldAnnouncementCategoryClass">
                  <span class="world-announcement-badge">天道传音</span>
                  <span class="world-announcement-text">{{ activeWorldAnnouncement.message }}</span>
                </div>
              </div>
            </transition>
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
  import { computed, h, onMounted, onUnmounted, ref, watch, watchEffect } from 'vue'
  import { useRouter, useRoute } from 'vue-router'
  import { NIcon, darkTheme } from 'naive-ui'
  import {
    BarChartOutlined,
    AppstoreOutlined,
    BookOutlined,
    CompassOutlined,
    ExperimentOutlined,
    GiftOutlined,
    HomeOutlined,
    MedicineBoxOutlined,
    SettingOutlined,
    TrophyOutlined,
    WalletOutlined
  } from '@ant-design/icons-vue'
  import { Moon, Sunny, Flash } from '@vicons/ionicons5'

  import GlobalChatDock from './components/GlobalChatDock.vue'
  import PlayerProfileDrawer from './components/PlayerProfileDrawer.vue'
  import { getRealmName } from './plugins/realm'
  import { fetchActivePlayerCount } from './api/modules/player'
  import { usePlayerStore } from './stores/player'
  import { useGameRealtimeStore } from './stores/game-realtime'
  import { useSessionStore } from './stores/session'
  import { formatScaledGrowth } from './utils/growth-display'

  const router = useRouter()
  const route = useRoute()
  const playerStore = usePlayerStore()
  const gameRealtimeStore = useGameRealtimeStore()
  const sessionStore = useSessionStore()

  const menuOptions = ref([])
  const isNewPlayer = ref(false)
  const isLoading = ref(true)
  const isCompact = ref(false)
  const showProfileDrawer = ref(false)

  const activeUsersSyncIntervalMs = 300000
  const appTitle = '修仙大世界'
  let activeUsersTimer = null
  const activePlayers = ref(0)
  const activeWindowHours = ref(12)

  const currentRealmName = computed(() => getRealmName(playerStore.level).name)
  const linuxDoAvatar = computed(() => String(sessionStore.user?.avatar || '').trim())
  const linuxDoUserId = computed(() => String(sessionStore.user?.linuxDoUserId || '').trim())
  const playerInitial = computed(() => {
    const source = String(playerStore.name || '').trim()
    return source ? source.slice(0, 1) : '修'
  })
  const cultivationPercent = computed(() => {
    const max = Number(playerStore.maxCultivation || 1)
    if (max <= 0) return 0
    return Number(((Number(playerStore.cultivation || 0) / max) * 100).toFixed(2))
  })
  const displaySpirit = computed(() => formatScaledGrowth(playerStore.spirit, { maximumFractionDigits: 1 }))
  const displayCultivation = computed(() => formatScaledGrowth(playerStore.cultivation))
  const displayMaxCultivation = computed(() => formatScaledGrowth(playerStore.maxCultivation))
  const showGameShell = computed(() => sessionStore.isAuthenticated)
  const showGlobalChatDock = computed(() => showGameShell.value && !isLoading.value)
  const activeWorldAnnouncement = computed(() => gameRealtimeStore.activeWorldAnnouncement)
  const worldAnnouncementCategoryClass = computed(() => {
    const category = activeWorldAnnouncement.value?.category
    switch (category) {
      case 'breakthrough':
        return 'is-breakthrough'
      case 'enhance':
        return 'is-enhance'
      case 'loot':
        return 'is-loot'
      default:
        return ''
    }
  })
  const titleActivity = computed(() => {
    if (!sessionStore.isAuthenticated) {
      return '未登录'
    }

    const huntingRun = gameRealtimeStore.huntingRun
    if (huntingRun?.isActive) {
      const mapName = String(huntingRun.mapName || huntingRun.mapId || '').trim()
      const huntingState = String(huntingRun.state || '').trim()
      if (huntingState === 'reviving') {
        return mapName ? `正在${mapName}中复活` : '正在刷图复活'
      }
      return mapName ? `正在${mapName}中战斗` : '正在刷图战斗'
    }

    const meditationRun = gameRealtimeStore.meditationRun
    if (meditationRun?.isActive) {
      return '正在打坐'
    }

    const explorationRun = gameRealtimeStore.explorationRun
    if (explorationRun?.isActive) {
      const locationName = String(explorationRun.locationName || explorationRun.locationId || '').trim()
      return locationName ? `正在${locationName}探索` : '正在探索'
    }

    const currentPath = String(route.path || '')
    if (currentPath.startsWith('/dungeon')) {
      return '正在秘境探索'
    }
    if (currentPath.startsWith('/exploration')) {
      return '正在探索'
    }
    if (currentPath.startsWith('/alchemy')) {
      return '正在炼丹'
    }
    return '空闲中'
  })

  const formatNumber = value => Number(value || 0).toLocaleString()
  const formatInt = value => Math.floor(Number(value || 0)).toLocaleString()
  const formatPercent = value => `${(Number(value || 0) * 100).toFixed(1)}%`

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
        icon: renderIcon(BarChartOutlined)
      },
      {
        label: '拍卖',
        key: 'auction',
        icon: renderIcon(AppstoreOutlined)
      },
      {
        label: '充值',
        key: 'recharge',
        icon: renderIcon(WalletOutlined)
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

  watchEffect(() => {
    const status = String(titleActivity.value || '').trim()
    document.title = status ? `${appTitle}｜${status}` : appTitle
  })

  const loadActivePlayers = async ({ silent = true } = {}) => {
    if (!sessionStore.isAuthenticated) {
      activePlayers.value = 0
      return
    }
    try {
      const result = await fetchActivePlayerCount()
      activePlayers.value = Number(result?.activeUsers || 0)
      activeWindowHours.value = Number(result?.windowHours || 12)
    } catch (error) {
      if (!silent) {
        console.error('加载活跃人数失败:', error)
      }
    }
  }

  const startActiveUsersPolling = () => {
    if (activeUsersTimer) return
    activeUsersTimer = setInterval(() => {
      loadActivePlayers({ silent: true })
    }, activeUsersSyncIntervalMs)
  }

  const stopActiveUsersPolling = () => {
    if (!activeUsersTimer) return
    clearInterval(activeUsersTimer)
    activeUsersTimer = null
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
        await loadActivePlayers({ silent: false })
      } finally {
        isNewPlayer.value = playerStore.isNewPlayer
        getMenuOptions()
      }
    } else {
      menuOptions.value = []
      isNewPlayer.value = true
      activePlayers.value = 0
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
        gameRealtimeStore.disconnect()
        stopActiveUsersPolling()
        activePlayers.value = 0
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
      gameRealtimeStore.connect()
      startActiveUsersPolling()
      loadActivePlayers({ silent: true })
    },
    { immediate: true }
  )

  onMounted(() => {
    syncViewportMode()
    window.addEventListener('resize', syncViewportMode, { passive: true })
  })

  onUnmounted(() => {
    window.removeEventListener('resize', syncViewportMode)
    gameRealtimeStore.disconnect()
    stopActiveUsersPolling()
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

  .brand-presence {
    margin-top: 6px;
    font-size: 12px;
    color: var(--ink-sub);
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
    grid-template-columns: 280px minmax(0, 1fr);
    gap: 20px;
    align-items: start;
  }

  .workspace.is-compact {
    grid-template-columns: 1fr;
    padding-bottom: 80px;
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

  .cultivator-card {
    cursor: pointer;
    transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
    margin-bottom: 14px;
  }

  .cultivator-card:hover {
    transform: translateY(-2px);
    box-shadow: 0 18px 48px rgba(47, 107, 109, 0.15);
    border-color: var(--accent-primary);
  }

  .nav-card {
    padding: 8px 0;
  }

  .cultivator-head {
    display: flex;
    align-items: center;
    gap: 12px;
    margin-bottom: 14px;
  }

  .name-avatar-wrap {
    width: 46px;
    height: 46px;
    flex-shrink: 0;
  }

  .linuxdo-avatar {
    width: 46px;
    height: 46px;
    border-radius: 12px;
    object-fit: cover;
    border: 1px solid rgba(127, 127, 127, 0.2);
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
  }

  .name-seal.mini {
    width: 24px;
    height: 24px;
    font-size: 14px;
    border-radius: 6px;
    margin-bottom: 4px;
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
    padding: 24px;
    display: flex;
    flex-direction: column;
    gap: 20px;
  }

  .page-head h2 {
    font-size: 32px;
    letter-spacing: 0.04em;
    font-family: var(--font-display);
  }

  .page-desc {
    color: var(--ink-sub);
    font-size: 14px;
  }

  /* Mobile Bottom Tabbar */
  .mobile-tabbar {
    position: fixed;
    bottom: 0;
    left: 0;
    right: 0;
    height: 72px;
    background: color-mix(in srgb, var(--panel-bg) 95%, transparent);
    backdrop-filter: blur(20px);
    border-top: 1px solid var(--panel-border);
    display: flex;
    align-items: center;
    justify-content: space-around;
    padding: 0 10px;
    z-index: 1000;
    box-shadow: 0 -10px 30px rgba(0, 0, 0, 0.05);
  }

  .tab-item {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 4px;
    color: var(--ink-sub);
    transition: all 0.3s ease;
    padding: 8px 12px;
    border-radius: 12px;
    cursor: pointer;
  }

  .tab-item :deep(.n-icon) {
    font-size: 22px;
  }

  .tab-item span {
    font-size: 10px;
    font-weight: 500;
  }

  .tab-item.active {
    color: var(--accent-primary);
    background: var(--accent-muted);
  }

  .tab-item.active :deep(.n-icon) {
    transform: scale(1.1);
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

  /* Scrollbar */
  ::-webkit-scrollbar { width: 8px; height: 8px; }
  ::-webkit-scrollbar-track { background: rgba(0, 0, 0, 0.05); }
  ::-webkit-scrollbar-thumb { background: rgba(0, 0, 0, 0.2); border-radius: 10px; }
  html.dark ::-webkit-scrollbar-track { background: rgba(255, 255, 255, 0.05); }
  html.dark ::-webkit-scrollbar-thumb { background: rgba(255, 255, 255, 0.2); }

  .world-announcement-layer {
    position: fixed;
    top: 84px;
    left: 0;
    right: 0;
    z-index: 2200;
    pointer-events: none;
    overflow: hidden;
  }

  .world-announcement-track {
    display: inline-flex;
    align-items: center;
    gap: 14px;
    padding: 12px 22px;
    min-width: max-content;
    border: 1px solid rgba(255, 214, 102, 0.55);
    border-radius: 999px;
    background: linear-gradient(90deg, rgba(24, 26, 44, 0.95), rgba(79, 45, 18, 0.92));
    box-shadow: 0 16px 30px rgba(0, 0, 0, 0.28);
    animation: world-banner-scroll 9s linear forwards;
  }

  .world-announcement-track.is-breakthrough {
    border-color: rgba(136, 207, 255, 0.58);
    background: linear-gradient(90deg, rgba(14, 38, 68, 0.96), rgba(24, 103, 162, 0.9));
  }

  .world-announcement-badge { color: #ffe29a; font-size: 12px; letter-spacing: 0.2em; }
  .world-announcement-text { color: #fff7e1; font-size: 16px; font-weight: 700; }

  @keyframes world-banner-scroll {
    from { transform: translateX(100vw); }
    to { transform: translateX(calc(-100% - 32px)); }
  }

  @media (max-width: 1080px) {
    .app-header { padding: 10px 0; }
    .brand-title { font-size: 22px; }
    .header-wrap { padding: 10px 16px; }
    .workspace { gap: 14px; }
  }

  @media (max-width: 768px) {
    .app-content { padding: 10px; }
    .stage-shell { min-height: auto; border-radius: 12px !important; }
    .page-view { padding: 16px; }
    .page-head h2 { font-size: 24px; }
    .resource-ribbon { display: grid; grid-template-columns: repeat(3, 1fr); gap: 6px; }
    .stat-chip { min-width: 0; padding: 6px; }
    .stat-chip strong { font-size: 12px; }
    .world-announcement-layer { top: 72px; }
  }

  @media (max-width: 480px) {
    .brand-presence { font-size: 12px; }
    .resource-ribbon { grid-template-columns: 1fr; }
    .cultivator-head h2 { font-size: 20px; }
    .page-view { padding: 10px; }
  }
</style>
