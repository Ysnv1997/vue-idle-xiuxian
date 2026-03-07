<template>
  <div class="page-view home-page">
    <!-- 沉浸式登录背景效果 -->
    <div class="zen-background">
      <div class="mist-layer"></div>
      <div class="mountain-silhouette"></div>
    </div>

    <div class="home-content">
      <div class="brand-hero">
        <p class="brand-eyebrow">—— 放置类修仙角色扮演游戏 ——</p>
        <h1 class="brand-title">修仙大世界</h1>
        <p class="brand-motto">一念成仙，一念入魔；万载修持，只为长生。</p>
      </div>

      <!-- 登录/进入区域 -->
      <div class="action-card">
        <template v-if="!sessionStore.isAuthenticated">
          <div class="login-intro">
            <h3>开启仙途</h3>
            <p>即刻加入数千名修士的行列，参悟天道，证得圆满。</p>
          </div>
          <n-button 
            type="primary" 
            size="large" 
            round 
            class="cta-btn"
            @click="sessionStore.redirectToLinuxDoLogin"
          >
            <template #icon>
              <img :src="linuxDoIcon" alt="LinuxDO" class="btn-icon" />
            </template>
            使用 LinuxDO 账号登录
          </n-button>
          <div class="login-hint">本游戏深度集成 LinuxDO 社区身份</div>
        </template>

        <template v-else>
          <div class="login-status">
            <div class="user-welcome">欢迎归来，<strong>{{ sessionStore.user?.username }}</strong> 道友</div>
            <n-button type="primary" size="large" round class="cta-btn" @click="$router.push('/cultivation')">
              进入仙界
            </n-button>
            <n-button quaternary size="small" @click="logout">暂别红尘 (退出)</n-button>
          </div>
        </template>
      </div>

      <!-- 游戏特色简述 -->
      <div class="features-grid">
        <div class="feature-item">
          <n-icon size="32" color="var(--accent-primary)"><BookOutline /></n-icon>
          <h4>功法修持</h4>
          <p>打坐纳气，突破瓶颈，步步登天。</p>
        </div>
        <div class="feature-item">
          <n-icon size="32" color="var(--accent-primary)"><FlameOutline /></n-icon>
          <h4>神兵炼制</h4>
          <p>搜集灵草，开炉炼丹，锻造旷世绝锋。</p>
        </div>
        <div class="feature-item">
          <n-icon size="32" color="var(--accent-primary)"><StorefrontOutline /></n-icon>
          <h4>自由贸易</h4>
          <p>坊市交流，互通有无，积累惊天财富。</p>
        </div>
      </div>
    </div>

    <!-- 底部版权与链接 -->
    <footer class="home-footer">
      <div class="footer-links">
        <a href="https://linux.do" target="_blank">LinuxDO 社区</a>
        <n-divider vertical />
        <span>开源修仙项目</span>
      </div>
      <p class="copyright">© 2024-2026 XiuXian Game · 基于天道律法运行</p>
    </footer>
  </div>
</template>

<script setup>
  import { computed } from 'vue'
  import { useSessionStore } from '../stores/session'
  import { useMessage } from 'naive-ui'
  import { useRouter } from 'vue-router'
  import { BookOutline, FlameOutline, StorefrontOutline } from '@vicons/ionicons5'
  import linuxDoIcon from '../assets/icons/linuxdo-icon.png'

  const router = useRouter()
  const sessionStore = useSessionStore()
  const message = useMessage()

  const logout = async () => {
    await sessionStore.logout()
    message.success('已退隐江湖')
  }
</script>

<style scoped>
.home-page {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  min-height: calc(100vh - 100px);
  position: relative;
  overflow: hidden;
  padding: 40px 20px;
}

.zen-background {
  position: absolute;
  inset: 0;
  z-index: -1;
  pointer-events: none;
}

.mist-layer {
  position: absolute;
  inset: 0;
  background: radial-gradient(circle at 50% 50%, transparent 0%, var(--bg-a) 100%);
}

.home-content {
  max-width: 800px;
  width: 100%;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 60px;
  z-index: 1;
}

.brand-hero { text-align: center; }
.brand-eyebrow { font-size: 14px; letter-spacing: 4px; color: var(--ink-sub); margin-bottom: 16px; text-transform: uppercase; }
.brand-title { font-family: var(--font-display); font-size: 64px; margin: 0; color: var(--ink-main); text-shadow: 0 4px 20px rgba(0,0,0,0.1); }
.brand-motto { font-size: 18px; color: var(--ink-sub); margin-top: 16px; font-style: italic; opacity: 0.8; }

.action-card {
  background: color-mix(in srgb, var(--panel-bg) 80%, transparent);
  backdrop-filter: blur(24px);
  border: 1px solid var(--panel-border);
  border-radius: 32px;
  padding: 40px;
  width: 100%;
  max-width: 480px;
  text-align: center;
  box-shadow: 0 20px 60px rgba(0,0,0,0.1);
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.login-intro h3 { font-family: var(--font-display); font-size: 24px; margin-bottom: 8px; }
.login-intro p { font-size: 14px; color: var(--ink-sub); line-height: 1.6; }

.cta-btn { height: 56px !important; font-size: 18px !important; font-weight: bold !important; width: 100%; }
.btn-icon { width: 24px; height: 24px; border-radius: 50%; }

.login-hint { font-size: 11px; opacity: 0.5; }

.user-welcome { font-size: 16px; margin-bottom: 16px; }

.features-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 32px;
  width: 100%;
}

.feature-item {
  text-align: center;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
}
.feature-item h4 { font-size: 16px; font-weight: bold; margin: 0; }
.feature-item p { font-size: 13px; color: var(--ink-sub); line-height: 1.5; }

.home-footer {
  margin-top: 80px;
  text-align: center;
  opacity: 0.6;
}
.footer-links { margin-bottom: 8px; font-size: 13px; }
.footer-links a { color: var(--ink-main); text-decoration: none; }
.footer-links a:hover { color: var(--accent-primary); }
.copyright { font-size: 11px; }

@media (max-width: 768px) {
  .brand-title { font-size: 40px; }
  .features-grid { grid-template-columns: 1fr; gap: 40px; }
  .action-card { padding: 30px 20px; }
}
</style>
