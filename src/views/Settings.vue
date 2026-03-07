<template>
  <div class="page-view settings-page">
    <!-- 顶部标题 -->
    <header class="page-head">
      <div class="head-main">
        <p class="page-eyebrow">洞府修整 · 凡尘管理</p>
        <h2 class="page-title">设置与管理</h2>
      </div>
      <div class="version-tag">版本 {{ version }}</div>
    </header>

    <div class="settings-content">
      <!-- 个人身份卡片 -->
      <section class="identity-section">
        <div class="identity-card">
          <div class="id-avatar">
            <img v-if="sessionStore.user?.avatar" :src="sessionStore.user.avatar" alt="Avatar" />
            <div v-else class="avatar-placeholder">{{ (sessionStore.user?.username || '道').slice(0, 1) }}</div>
          </div>
          <div class="id-info">
            <div class="id-name">{{ sessionStore.user?.username || '未知道友' }}</div>
            <div class="id-sub">LinuxDo ID: {{ sessionStore.user?.linuxDoUserId || '未绑定' }}</div>
          </div>
          <n-button type="error" ghost round @click="confirmLogout" :loading="loggingOut">
            退出登录
          </n-button>
        </div>
      </section>

      <!-- 通用设置 -->
      <section class="general-settings">
        <div class="section-title">通用偏好</div>
        <div class="settings-grid">
          <div class="setting-item" @click="playerStore.toggle">
            <div class="s-info">
              <div class="s-label">视觉风格</div>
              <div class="s-desc">{{ playerStore.isDarkMode ? '幽冥暗色' : '清逸浅色' }}</div>
            </div>
            <n-icon size="24">
              <Moon v-if="playerStore.isDarkMode" />
              <Sunny v-else />
            </n-icon>
          </div>

          <div class="setting-item" @click="qq = true">
            <div class="s-info">
              <div class="s-label">同道交流</div>
              <div class="s-desc">加入玩家交流群</div>
            </div>
            <n-icon size="24"><PeopleOutline /></n-icon>
          </div>
        </div>
      </section>

      <!-- 天道管理后台 (Admin Console) -->
      <section class="admin-section" v-if="showAdminConsole">
        <div class="section-title">天道管理 ({{ formatAdminRole(sessionStore.user?.adminRole) }})</div>
        
        <n-tabs type="line" animated class="admin-tabs">
          <!-- 仙职管理 -->
          <n-tab-pane name="admins" tab="仙职管理" v-if="canManageAdmins">
            <div class="admin-form-box">
              <n-space vertical gap="16">
                <n-grid :cols="24" :x-gap="12" :y-gap="12">
                  <n-gi :span="8"><n-input v-model:value="newAdminLinuxDoUserId" placeholder="用户 ID" /></n-gi>
                  <n-gi :span="6"><n-select v-model:value="newAdminRole" :options="adminRoleOptions" /></n-gi>
                  <n-gi :span="10"><n-input v-model:value="newAdminNote" placeholder="备注" /></n-gi>
                </n-grid>
                <n-space justify="end">
                  <n-button :loading="adminLoading" @click="loadAdminUsers">刷新</n-button>
                  <n-button type="primary" :loading="adminSubmitting" @click="submitAdminUser">授予仙职</n-button>
                </n-space>
              </n-space>

              <n-scrollbar class="admin-table-scroll">
                <n-table striped size="small" class="admin-table">
                  <thead>
                    <tr>
                      <th>用户 ID</th>
                      <th>职位</th>
                      <th>时间</th>
                      <th>操作</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-for="item in adminUsers" :key="item.id">
                      <td class="font-mono">{{ item.linuxDoUserId }}</td>
                      <td><n-tag size="small" :type="item.role === 'super_admin' ? 'error' : 'info'">{{ formatAdminRole(item.role) }}</n-tag></td>
                      <td class="text-sub">{{ formatTime(item.updatedAt) }}</td>
                      <td>
                        <n-button size="tiny" quaternary type="error" :disabled="isCurrentOperator(item.linuxDoUserId)" @click="removeAdminUserAction(item.linuxDoUserId)">撤职</n-button>
                      </td>
                    </tr>
                  </tbody>
                </n-table>
              </n-scrollbar>
            </div>
          </n-tab-pane>

          <!-- 律法管理 (Runtime Config) -->
          <n-tab-pane name="configs" tab="律法配置" v-if="canManageRuntimeConfigs">
            <div class="admin-form-box">
              <n-space vertical gap="16">
                <div class="config-quick-form">
                  <n-input v-model:value="runtimeConfigForm.key" placeholder="配置键 (key)" />
                  <n-input v-model:value="runtimeConfigForm.value" placeholder="值 (value)" />
                  <n-select v-model:value="runtimeConfigForm.valueType" :options="runtimeConfigValueTypeOptions" style="width: 120px" />
                  <n-button type="primary" @click="submitRuntimeConfig" :loading="runtimeConfigSubmitting">保存</n-button>
                </div>
                <div class="config-search-row">
                  <n-input v-model:value="runtimeConfigKeyword" placeholder="检索键或说明..." size="small">
                    <template #prefix><n-icon><SearchOutline /></n-icon></template>
                  </n-input>
                  <n-button size="small" @click="loadRuntimeConfigs">同步</n-button>
                </div>
              </n-space>

              <n-scrollbar class="admin-table-scroll">
                <n-table striped size="small" class="admin-table">
                  <thead><tr><th>键</th><th>值</th><th>描述</th><th>操作</th></tr></thead>
                  <tbody>
                    <tr v-for="config in filteredRuntimeConfigs" :key="config.key">
                      <td class="font-mono text-primary">{{ config.key }}</td>
                      <td>{{ config.value }}</td>
                      <td class="text-sub">{{ config.description || '-' }}</td>
                      <td><n-button size="tiny" quaternary @click="editRuntimeConfig(config)">编辑</n-button></td>
                    </tr>
                  </tbody>
                </n-table>
              </n-scrollbar>
            </div>
          </n-tab-pane>

          <!-- 聊天管治 (Moderation) -->
          <n-tab-pane name="moderation" tab="聊天管治" v-if="canModerateChat">
            <div class="admin-form-box">
              <div class="mute-form">
                <n-input v-model:value="chatMuteTargetLinuxDoUserId" placeholder="目标 ID" />
                <n-select v-model:value="chatMuteDurationMinutes" :options="chatMuteDurationOptions" style="width: 140px" />
                <n-button type="warning" @click="submitChatMute">执行禁言</n-button>
              </div>
              
              <div class="moderation-lists">
                <div class="m-list-head">活跃禁言 ({{ chatActiveMutes.length }})</div>
                <n-scrollbar style="max-height: 300px">
                  <div class="mute-cards">
                    <div v-for="item in chatActiveMutes" :key="item.id" class="mute-card">
                      <div class="m-info">
                        <div class="m-user">{{ item.targetName }} ({{ item.targetLinuxDoUserId }})</div>
                        <div class="m-time">到期：{{ formatTime(item.mutedUntil) }}</div>
                      </div>
                      <n-button size="tiny" secondary type="success" @click="submitChatUnmute(item.targetLinuxDoUserId)">赦免</n-button>
                    </div>
                    <n-empty v-if="chatActiveMutes.length === 0" description="四海清平，无人被禁" />
                  </div>
                </n-scrollbar>
              </div>
            </div>
          </n-tab-pane>
        </n-tabs>
      </section>
    </div>

    <!-- 弹窗 -->
    <n-modal preset="dialog" title="同道交流群" v-model:show="qq" class="custom-modal">
      <div class="group-info">
        <p>目前尚无官方群组，请在 <strong>LinuxDO 社区</strong> 帖子下回帖互动。</p>
        <p class="text-sub">关注开发者动态，获取最新秘籍。</p>
      </div>
    </n-modal>
  </div>
</template>

<script setup>
  import { computed, onMounted, ref } from 'vue'
  import { useDialog, useMessage } from 'naive-ui'
  import { useRouter } from 'vue-router'
  import { 
    Moon, Sunny, PeopleOutline, ExitOutline, ShieldCheckmarkOutline, 
    SettingsOutline, SearchOutline, ConstructOutline, WarningOutline
  } from '@vicons/ionicons5'
  import {
    deleteChatBlockedWord,
    fetchChatAdminMutes,
    fetchChatAdminReports,
    fetchChatBlockedWords,
    muteChatUser,
    reviewChatAdminReport,
    unmuteChatUser,
    upsertChatBlockedWord
  } from '../api/modules/chat'
  import {
    fetchAdminUsers,
    fetchRuntimeConfigAudits,
    fetchRuntimeConfigs,
    removeAdminUser,
    upsertAdminUser,
    upsertRuntimeConfig
  } from '../api/modules/admin'
  import { useSessionStore } from '../stores/session'
  import { usePlayerStore } from '../stores/player'

  const version = __APP_VERSION__
  const sessionStore = useSessionStore()
  const playerStore = usePlayerStore()
  const message = useMessage()
  const dialog = useDialog()
  const router = useRouter()

  // 基础状态
  const qq = ref(false)
  const loggingOut = ref(false)

  // 管理员状态
  const adminLoading = ref(false)
  const adminSubmitting = ref(false)
  const adminUsers = ref([])
  const newAdminLinuxDoUserId = ref('')
  const newAdminRole = ref('ops_admin')
  const newAdminNote = ref('')

  const runtimeConfigLoading = ref(false)
  const runtimeConfigSubmitting = ref(false)
  const runtimeConfigs = ref([])
  const runtimeConfigKeyword = ref('')
  const runtimeConfigForm = ref({ key: '', value: '', valueType: 'string', category: 'general', description: '' })

  const chatActiveMutes = ref([])
  const chatMuteTargetLinuxDoUserId = ref('')
  const chatMuteDurationMinutes = ref(60)
  const chatMuteListLoading = ref(false)
  const chatModerationSubmitting = ref(false)

  // 选项配置
  const adminRoleOptions = [
    { label: '超管 (Super)', value: 'super_admin' },
    { label: '运营 (Ops)', value: 'ops_admin' },
    { label: '房管 (Chat)', value: 'chat_admin' }
  ]
  const runtimeConfigValueTypeOptions = [
    { label: '文本 (string)', value: 'string' },
    { label: '整数 (int)', value: 'int' },
    { label: '小数 (float)', value: 'float' },
    { label: '布尔 (bool)', value: 'bool' }
  ]
  const chatMuteDurationOptions = [
    { label: '10分', value: 10 }, { label: '1小时', value: 60 }, { label: '24小时', value: 1440 }, { label: '3天', value: 4320 }
  ]

  // ---------------- 权限计算 ----------------
  const canManageAdmins = computed(() => Boolean(sessionStore.user?.canManageAdmins))
  const canManageRuntimeConfigs = computed(() => Boolean(sessionStore.user?.canManageRuntimeConfigs))
  const canModerateChat = computed(() => Boolean(sessionStore.user?.canModerateChat))
  const showAdminConsole = computed(() => canManageAdmins.value || canManageRuntimeConfigs.value || canModerateChat.value)

  const filteredRuntimeConfigs = computed(() => {
    const k = runtimeConfigKeyword.value.trim().toLowerCase()
    return k ? runtimeConfigs.value.filter(i => String(i.key).toLowerCase().includes(k) || String(i.description).toLowerCase().includes(k)) : runtimeConfigs.value
  })

  // ---------------- 逻辑方法 ----------------
  const confirmLogout = () => {
    dialog.warning({
      title: '退隐山林', content: '确认要退出当前账号，暂别修仙界吗？',
      positiveText: '确认退出', negativeText: '再留一会',
      onPositiveClick: async () => {
        loggingOut.value = true
        await sessionStore.logout()
        message.success('已平安退隐')
        router.replace('/')
      }
    })
  }

  const formatAdminRole = r => ({ super_admin: '超管', ops_admin: '运营', chat_admin: '房管' }[r] || r)
  const formatTime = v => v ? new Date(v).toLocaleString() : '-'
  const isCurrentOperator = id => String(sessionStore.user?.linuxDoUserId) === String(id)

  const loadAdminUsers = async () => {
    adminLoading.value = true
    try {
      const res = await fetchAdminUsers(500)
      adminUsers.value = res?.users || []
    } finally { adminLoading.value = false }
  }

  const loadRuntimeConfigs = async () => {
    runtimeConfigLoading.value = true
    try {
      const res = await fetchRuntimeConfigs({ limit: 1000 })
      runtimeConfigs.value = res?.configs || []
    } finally { runtimeConfigLoading.value = false }
  }

  const loadChatAdminMutes = async () => {
    chatMuteListLoading.value = true
    try {
      const res = await fetchChatAdminMutes('', 200)
      chatActiveMutes.value = res?.mutes || []
    } finally { chatMuteListLoading.value = false }
  }

  const submitAdminUser = async () => {
    if (!newAdminLinuxDoUserId.value) return
    adminSubmitting.value = true
    try {
      await upsertAdminUser(newAdminLinuxDoUserId.value, newAdminRole.value, newAdminNote.value)
      message.success('仙职已更新')
      loadAdminUsers()
    } catch (e) { message.error('操作失败') }
    finally { adminSubmitting.value = false }
  }

  const removeAdminUserAction = async id => {
    adminSubmitting.value = true
    try {
      await removeAdminUser(id)
      message.success('已撤销仙职')
      loadAdminUsers()
    } finally { adminSubmitting.value = false }
  }

  const submitRuntimeConfig = async () => {
    runtimeConfigSubmitting.value = true
    try {
      await upsertRuntimeConfig(runtimeConfigForm.value)
      message.success('律法已更新')
      loadRuntimeConfigs()
    } catch (e) { message.error('更新失败') }
    finally { runtimeConfigSubmitting.value = false }
  }

  const editRuntimeConfig = c => runtimeConfigForm.value = { ...c }

  const submitChatMute = async () => {
    chatModerationSubmitting.value = true
    try {
      await muteChatUser(chatMuteTargetLinuxDoUserId.value, chatMuteDurationMinutes.value)
      message.success('禁言令已下达')
      loadChatAdminMutes()
    } finally { chatModerationSubmitting.value = false }
  }

  const submitChatUnmute = async id => {
    try {
      await unmuteChatUser(id)
      message.success('禁言已解除')
      loadChatAdminMutes()
    } catch (e) { message.error('操作失败') }
  }

  onMounted(() => {
    if (canManageAdmins.value) loadAdminUsers()
    if (canManageRuntimeConfigs.value) loadRuntimeConfigs()
    if (canModerateChat.value) loadChatAdminMutes()
  })
</script>

<style scoped>
.settings-page {
  display: flex;
  flex-direction: column;
  height: 100%;
  max-width: 900px;
  margin: 0 auto;
}

.page-head {
  display: flex;
  justify-content: space-between;
  align-items: flex-end;
  margin-bottom: 32px;
}

.version-tag { font-size: 12px; color: var(--ink-sub); opacity: 0.6; }

.settings-content { display: flex; flex-direction: column; gap: 40px; }

.section-title { font-size: 14px; font-weight: bold; color: var(--accent-primary); margin-bottom: 16px; opacity: 0.8; }

/* 身份卡片 */
.identity-card {
  background: var(--panel-bg);
  border: 1px solid var(--panel-border);
  border-radius: 24px;
  padding: 24px;
  display: flex;
  align-items: center;
  gap: 20px;
  box-shadow: 0 8px 32px rgba(0,0,0,0.05);
}

.id-avatar { width: 64px; height: 64px; border-radius: 16px; overflow: hidden; border: 2px solid var(--panel-border); }
.id-avatar img { width: 100%; height: 100%; object-fit: cover; }
.avatar-placeholder { width: 100%; height: 100%; background: var(--accent-primary); color: white; display: grid; place-items: center; font-family: var(--font-display); font-size: 32px; }

.id-info { flex: 1; }
.id-name { font-size: 20px; font-weight: bold; }
.id-sub { font-size: 12px; color: var(--ink-sub); margin-top: 4px; }

/* 设置网格 */
.settings-grid { display: grid; grid-template-columns: repeat(2, 1fr); gap: 16px; }

.setting-item {
  background: var(--panel-bg);
  border: 1px solid var(--panel-border);
  border-radius: 20px;
  padding: 20px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  cursor: pointer;
  transition: all 0.2s;
}
.setting-item:hover { border-color: var(--accent-primary); transform: translateY(-2px); }

.s-label { font-weight: bold; font-size: 15px; }
.s-desc { font-size: 12px; color: var(--ink-sub); margin-top: 4px; }

/* 管理后台 */
.admin-section {
  background: var(--panel-bg);
  border: 1px solid var(--panel-border);
  border-radius: 24px;
  padding: 24px;
  margin-bottom: 100px;
}

.admin-form-box { display: flex; flex-direction: column; gap: 20px; padding: 10px 0; }

.config-quick-form, .mute-form { display: flex; gap: 12px; }
.config-search-row { display: flex; justify-content: space-between; gap: 20px; }

.admin-table-scroll { max-height: 400px; margin-top: 10px; border: 1px solid var(--panel-border); border-radius: 12px; }
.admin-table { margin: 0; }

.mute-cards { display: grid; grid-template-columns: repeat(auto-fill, minmax(240px, 1fr)); gap: 12px; }
.mute-card {
  padding: 12px 16px;
  background: rgba(0,0,0,0.02);
  border: 1px solid var(--panel-border);
  border-radius: 12px;
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.m-user { font-size: 13px; font-weight: bold; }
.m-time { font-size: 11px; opacity: 0.5; margin-top: 2px; }

.font-mono { font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace; }
.text-primary { color: var(--accent-primary); }
.text-sub { font-size: 11px; opacity: 0.5; }

@media (max-width: 768px) {
  .identity-card { flex-direction: column; text-align: center; }
  .settings-grid { grid-template-columns: 1fr; }
  .config-quick-form, .mute-form { flex-direction: column; }
  .admin-section { margin-bottom: 120px; }
}
</style>
