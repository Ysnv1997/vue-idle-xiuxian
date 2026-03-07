<template>
  <section class="page-view settings-view">
    <header class="page-head">
      <p class="page-eyebrow">洞府设置</p>
      <h2>游戏设置</h2>
      <p class="page-desc">查看版本信息与社区入口。</p>
    </header>

    <n-card :bordered="false" class="page-card">
      <template #header-extra>游戏版本 {{ version }}</template>
      <n-space vertical>
        <n-alert type="info" :show-icon="false">
          当前版本：<strong>{{ version }}</strong>
        </n-alert>
        <n-space>
          <n-button tertiary @click="qq = true">玩家交流群</n-button>
        </n-space>
      </n-space>
    </n-card>

    <n-card :bordered="false" class="page-card" title="账号与会话">
      <n-space vertical>
        <n-alert type="success" :show-icon="false">
          当前登录：<strong>{{ sessionStore.user?.username || '道友' }}</strong>
        </n-alert>
        <n-space>
          <n-button type="error" secondary :loading="loggingOut" @click="confirmLogout">退出登录</n-button>
        </n-space>
      </n-space>
    </n-card>

    <n-card v-if="showAdminConsole" :bordered="false" class="page-card" title="游戏管理后台">
      <n-space vertical>
        <n-alert type="warning" :show-icon="false">
          当前管理员角色：<strong>{{ formatAdminRole(sessionStore.user?.adminRole) }}</strong>
        </n-alert>

        <n-space v-if="canManageAdmins" vertical>
          <n-text depth="3">管理员管理（超管）</n-text>

          <n-space align="end">
            <n-input v-model:value="newAdminLinuxDoUserId" placeholder="LinuxDo 用户 ID" style="min-width: 240px" />
            <n-select
              v-model:value="newAdminRole"
              :options="adminRoleOptions"
              placeholder="角色"
              style="min-width: 150px"
            />
            <n-input v-model:value="newAdminNote" placeholder="备注(可选)" style="min-width: 200px" />
            <n-button type="primary" :loading="adminSubmitting" @click="submitAdminUser">添加/更新管理员</n-button>
            <n-button :loading="adminLoading" @click="loadAdminUsers">刷新</n-button>
          </n-space>

          <n-spin :show="adminLoading">
            <n-table striped size="small">
              <thead>
                <tr>
                  <th>LinuxDo 用户 ID</th>
                  <th>角色</th>
                  <th>来源</th>
                  <th>备注</th>
                  <th>更新时间</th>
                  <th style="width: 120px">操作</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="item in adminUsers" :key="item.id">
                  <td>{{ item.linuxDoUserId }}</td>
                  <td>{{ formatAdminRole(item.role) }}</td>
                  <td>{{ item.source || '-' }}</td>
                  <td>{{ item.note || '-' }}</td>
                  <td>{{ formatTime(item.updatedAt) }}</td>
                  <td>
                    <n-button
                      size="small"
                      type="error"
                      tertiary
                      :disabled="isCurrentOperator(item.linuxDoUserId)"
                      :loading="adminSubmitting"
                      @click="removeAdminUserAction(item.linuxDoUserId)"
                    >
                      移除
                    </n-button>
                  </td>
                </tr>
                <tr v-if="adminUsers.length === 0">
                  <td colspan="6">
                    <n-empty description="暂无管理员数据" />
                  </td>
                </tr>
              </tbody>
            </n-table>
          </n-spin>
        </n-space>

        <template v-if="canManageRuntimeConfigs">
          <n-divider />

          <n-space vertical>
            <n-text depth="3">运行时配置（运营/超管）</n-text>

            <n-space align="end">
              <n-input v-model:value="runtimeConfigForm.key" placeholder="配置 key（如 chat.send.min_gap_ms）" style="min-width: 280px" />
              <n-input v-model:value="runtimeConfigForm.value" placeholder="配置值" style="min-width: 180px" />
              <n-select
                v-model:value="runtimeConfigForm.valueType"
                :options="runtimeConfigValueTypeOptions"
                placeholder="值类型"
                style="min-width: 120px"
              />
              <n-input v-model:value="runtimeConfigForm.category" placeholder="分类" style="min-width: 120px" />
              <n-input v-model:value="runtimeConfigForm.description" placeholder="说明(可选)" style="min-width: 220px" />
              <n-button type="primary" :loading="runtimeConfigSubmitting" @click="submitRuntimeConfig">保存配置</n-button>
            </n-space>

            <n-space align="end">
              <n-input
                v-model:value="runtimeConfigFilterCategory"
                placeholder="按分类过滤（如 chat / gameplay）"
                style="min-width: 220px"
              />
              <n-input
                v-model:value="runtimeConfigKeyword"
                placeholder="按 key/说明 搜索"
                style="min-width: 220px"
              />
              <n-button :loading="runtimeConfigLoading" @click="loadRuntimeConfigs">刷新配置列表</n-button>
            </n-space>

            <n-spin :show="runtimeConfigLoading">
              <n-table striped size="small">
                <thead>
                  <tr>
                    <th>key</th>
                    <th>value</th>
                    <th>type</th>
                    <th>category</th>
                    <th>description</th>
                    <th>更新时间</th>
                    <th style="width: 100px">操作</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="config in filteredRuntimeConfigs" :key="config.key">
                    <td>{{ config.key }}</td>
                    <td>{{ config.value }}</td>
                    <td>{{ config.valueType }}</td>
                    <td>{{ config.category }}</td>
                    <td>{{ config.description || '-' }}</td>
                    <td>{{ formatTime(config.updatedAt) }}</td>
                    <td>
                      <n-button size="small" tertiary @click="editRuntimeConfig(config)">编辑</n-button>
                    </td>
                  </tr>
                  <tr v-if="filteredRuntimeConfigs.length === 0">
                    <td colspan="7">
                      <n-empty description="暂无运行时配置" />
                    </td>
                  </tr>
                </tbody>
              </n-table>
            </n-spin>
          </n-space>

          <n-divider />

          <n-space vertical>
            <n-text depth="3">运行时配置变更审计</n-text>

            <n-space align="end">
              <n-input v-model:value="runtimeAuditKey" placeholder="按配置 key 过滤" style="min-width: 260px" />
              <n-input v-model:value="runtimeAuditCategory" placeholder="按分类过滤" style="min-width: 180px" />
              <n-button :loading="runtimeAuditLoading" @click="loadRuntimeConfigAudits">刷新审计</n-button>
            </n-space>

            <n-spin :show="runtimeAuditLoading">
              <n-table striped size="small">
                <thead>
                  <tr>
                    <th>时间</th>
                    <th>key</th>
                    <th>动作</th>
                    <th>旧值</th>
                    <th>新值</th>
                    <th>旧类型/分类</th>
                    <th>新类型/分类</th>
                    <th>操作者</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="log in runtimeAudits" :key="log.id">
                    <td>{{ formatTime(log.createdAt) }}</td>
                    <td>{{ log.key }}</td>
                    <td>{{ log.action }}</td>
                    <td>{{ formatAuditValue(log.oldValue) }}</td>
                    <td>{{ formatAuditValue(log.newValue) }}</td>
                    <td>{{ formatAuditMeta(log.oldValueType, log.oldCategory) }}</td>
                    <td>{{ formatAuditMeta(log.newValueType, log.newCategory) }}</td>
                    <td>{{ runtimeAuditOperatorDisplay(log) }}</td>
                  </tr>
                  <tr v-if="runtimeAudits.length === 0">
                    <td colspan="8">
                      <n-empty description="暂无审计记录" />
                    </td>
                  </tr>
                </tbody>
              </n-table>
            </n-spin>
          </n-space>
        </template>

        <template v-if="canModerateChat">
          <n-divider />

          <n-space vertical>
            <n-text depth="3">聊天禁言管理（聊天管理/超管）</n-text>

            <n-space align="end">
              <n-input v-model:value="chatMuteTargetLinuxDoUserId" placeholder="目标 LinuxDo 用户 ID" style="min-width: 220px" />
              <n-select
                v-model:value="chatMuteDurationMinutes"
                :options="chatMuteDurationOptions"
                placeholder="禁言时长"
                style="min-width: 130px"
              />
              <n-input v-model:value="chatMuteReason" placeholder="禁言原因（可选）" style="min-width: 220px" />
              <n-button type="warning" :loading="chatModerationSubmitting" @click="submitChatMute">执行禁言</n-button>
              <n-button type="default" :loading="chatModerationSubmitting" @click="submitChatUnmute(chatMuteTargetLinuxDoUserId)">
                按输入解禁
              </n-button>
              <n-button :loading="chatMuteListLoading" @click="loadChatAdminMutes">刷新禁言列表</n-button>
            </n-space>

            <n-spin :show="chatMuteListLoading">
              <n-table striped size="small">
                <thead>
                  <tr>
                    <th style="width: 150px">目标用户ID</th>
                    <th style="width: 140px">目标昵称</th>
                    <th style="width: 170px">禁言到期</th>
                    <th>原因</th>
                    <th style="width: 150px">操作人</th>
                    <th style="width: 90px">操作</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="item in chatActiveMutes" :key="item.id">
                    <td>{{ item.targetLinuxDoUserId || '-' }}</td>
                    <td>{{ item.targetName || '-' }}</td>
                    <td>{{ formatTime(item.mutedUntil) }}</td>
                    <td>{{ item.reason || '-' }}</td>
                    <td>{{ item.createdByLinuxDoUserId || '-' }}</td>
                    <td>
                      <n-button size="tiny" tertiary :loading="chatModerationSubmitting" @click="submitChatUnmute(item.targetLinuxDoUserId)">
                        解禁
                      </n-button>
                    </td>
                  </tr>
                  <tr v-if="chatActiveMutes.length === 0">
                    <td colspan="6">
                      <n-empty description="暂无生效中的禁言记录" />
                    </td>
                  </tr>
                </tbody>
              </n-table>
            </n-spin>

            <n-divider />

            <n-text depth="3">聊天举报审核（聊天管理/超管）</n-text>

            <n-space align="end">
              <n-select
                v-model:value="chatReportFilterStatus"
                :options="chatReportStatusOptions"
                placeholder="审核状态"
                style="min-width: 140px"
              />
              <n-input v-model:value="chatReportReviewNote" placeholder="审核备注（可选）" style="min-width: 260px" />
              <n-button :loading="chatReportLoading" @click="loadChatReports">刷新举报列表</n-button>
            </n-space>

            <n-spin :show="chatReportLoading">
              <n-table striped size="small">
                <thead>
                  <tr>
                    <th style="width: 80px">ID</th>
                    <th style="width: 140px">举报人</th>
                    <th style="width: 140px">被举报人</th>
                    <th>消息内容</th>
                    <th style="width: 100px">举报原因</th>
                    <th style="width: 85px">状态</th>
                    <th style="width: 140px">审核人</th>
                    <th style="width: 170px">举报时间</th>
                    <th style="width: 140px">操作</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="item in chatReports" :key="item.id">
                    <td>#{{ item.id }}</td>
                    <td>{{ item.reporterName || item.reporterLinuxDoUserId || '-' }}</td>
                    <td>{{ item.messageSenderName || item.messageSenderLinuxDoUserId || '-' }}</td>
                    <td>{{ item.messageContent || '-' }}</td>
                    <td>{{ item.reason || '-' }}</td>
                    <td>{{ formatChatReportStatus(item.reviewStatus) }}</td>
                    <td>{{ item.reviewedByLinuxDoUserId || '-' }}</td>
                    <td>{{ formatTime(item.createdAt) }}</td>
                    <td>
                      <n-space>
                        <n-button
                          size="tiny"
                          tertiary
                          type="success"
                          :disabled="item.reviewStatus === 'approved'"
                          :loading="chatModerationSubmitting"
                          @click="reviewChatReport(item.id, 'approved')"
                        >
                          通过
                        </n-button>
                        <n-button
                          size="tiny"
                          tertiary
                          type="error"
                          :disabled="item.reviewStatus === 'rejected'"
                          :loading="chatModerationSubmitting"
                          @click="reviewChatReport(item.id, 'rejected')"
                        >
                          驳回
                        </n-button>
                      </n-space>
                    </td>
                  </tr>
                  <tr v-if="chatReports.length === 0">
                    <td colspan="9">
                      <n-empty description="暂无举报记录" />
                    </td>
                  </tr>
                </tbody>
              </n-table>
            </n-spin>

            <n-divider />

            <n-text depth="3">聊天违禁词配置（聊天管理/超管）</n-text>

            <n-space align="end">
              <n-input v-model:value="adminBlockedWord" placeholder="新增或更新违禁词" style="min-width: 280px" />
              <n-select
                v-model:value="adminBlockedWordEnabled"
                :options="blockedWordStatusOptions"
                placeholder="状态"
                style="min-width: 120px"
              />
              <n-button type="primary" :loading="chatBlockedWordSubmitting" @click="submitBlockedWord">保存违禁词</n-button>
              <n-button :loading="chatBlockedWordLoading" @click="loadChatBlockedWords">刷新违禁词</n-button>
            </n-space>

            <n-spin :show="chatBlockedWordLoading">
              <n-table striped size="small">
                <thead>
                  <tr>
                    <th style="width: 200px">违禁词</th>
                    <th style="width: 90px">状态</th>
                    <th style="width: 170px">更新时间</th>
                    <th style="width: 180px">操作</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="item in chatBlockedWords" :key="item.word">
                    <td>{{ item.word || '-' }}</td>
                    <td>{{ item.enabled ? '启用' : '停用' }}</td>
                    <td>{{ formatTime(item.updatedAt) }}</td>
                    <td>
                      <n-space>
                        <n-button size="tiny" tertiary :loading="chatBlockedWordSubmitting" @click="toggleBlockedWord(item)">
                          {{ item.enabled ? '停用' : '启用' }}
                        </n-button>
                        <n-button
                          size="tiny"
                          tertiary
                          type="error"
                          :loading="chatBlockedWordSubmitting"
                          @click="removeBlockedWord(item.word)"
                        >
                          删除
                        </n-button>
                      </n-space>
                    </td>
                  </tr>
                  <tr v-if="chatBlockedWords.length === 0">
                    <td colspan="4">
                      <n-empty description="暂无违禁词配置" />
                    </td>
                  </tr>
                </tbody>
              </n-table>
            </n-spin>
          </n-space>
        </template>
      </n-space>
    </n-card>

    <n-modal preset="dialog" title="玩家交流群" v-model:show="qq">
      <n-card :bordered="false" size="huge" role="dialog" aria-modal="true">
        <n-space vertical>
          <n-text depth="3">微信群</n-text>
          
          <n-text>
            想啥呢，目前不需要，帖子里面去回复评论！
          </n-text>
        </n-space>
      </n-card>
    </n-modal>
  </section>
</template>

<script setup>
  import { computed, onMounted, ref } from 'vue'
  import { useDialog, useMessage } from 'naive-ui'
  import { useRouter } from 'vue-router'
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

  const version = __APP_VERSION__
  const qq = ref(false)
  const loggingOut = ref(false)
  const adminLoading = ref(false)
  const adminSubmitting = ref(false)
  const adminUsers = ref([])
  const newAdminLinuxDoUserId = ref('')
  const newAdminRole = ref('ops_admin')
  const newAdminNote = ref('')
  const runtimeConfigLoading = ref(false)
  const runtimeConfigSubmitting = ref(false)
  const runtimeConfigs = ref([])
  const chatBlockedWordLoading = ref(false)
  const chatBlockedWordSubmitting = ref(false)
  const chatBlockedWords = ref([])
  const chatMuteListLoading = ref(false)
  const chatReportLoading = ref(false)
  const chatModerationSubmitting = ref(false)
  const chatActiveMutes = ref([])
  const chatReports = ref([])
  const chatMuteTargetLinuxDoUserId = ref('')
  const chatMuteDurationMinutes = ref(60)
  const chatMuteReason = ref('')
  const chatReportFilterStatus = ref('pending')
  const chatReportReviewNote = ref('')
  const adminBlockedWord = ref('')
  const adminBlockedWordEnabled = ref(true)
  const runtimeAuditLoading = ref(false)
  const runtimeAudits = ref([])
  const runtimeAuditKey = ref('')
  const runtimeAuditCategory = ref('')
  const runtimeConfigFilterCategory = ref('')
  const runtimeConfigKeyword = ref('')
  const runtimeConfigForm = ref({
    key: '',
    value: '',
    valueType: 'string',
    category: 'general',
    description: ''
  })
  const runtimeConfigValueTypeOptions = [
    { label: 'string', value: 'string' },
    { label: 'int', value: 'int' },
    { label: 'float', value: 'float' },
    { label: 'bool', value: 'bool' }
  ]
  const blockedWordStatusOptions = [
    { label: '启用', value: true },
    { label: '停用', value: false }
  ]
  const chatReportStatusOptions = [
    { label: '待审核', value: 'pending' },
    { label: '已通过', value: 'approved' },
    { label: '已驳回', value: 'rejected' },
    { label: '全部', value: 'all' }
  ]
  const chatMuteDurationOptions = [
    { label: '10分钟', value: 10 },
    { label: '30分钟', value: 30 },
    { label: '1小时', value: 60 },
    { label: '6小时', value: 360 },
    { label: '24小时', value: 1440 },
    { label: '72小时', value: 4320 }
  ]
  const adminRoleOptions = [
    { label: '超管', value: 'super_admin' },
    { label: '运营', value: 'ops_admin' },
    { label: '聊天管理', value: 'chat_admin' }
  ]
  const sessionStore = useSessionStore()
  const message = useMessage()
  const dialog = useDialog()
  const router = useRouter()
  const canManageAdmins = computed(() => Boolean(sessionStore.user?.canManageAdmins))
  const canManageRuntimeConfigs = computed(() => Boolean(sessionStore.user?.canManageRuntimeConfigs))
  const canModerateChat = computed(() => Boolean(sessionStore.user?.canModerateChat))
  const showAdminConsole = computed(() => canManageAdmins.value || canManageRuntimeConfigs.value || canModerateChat.value)
  const filteredRuntimeConfigs = computed(() => {
    const keyword = runtimeConfigKeyword.value.trim().toLowerCase()
    if (!keyword) {
      return runtimeConfigs.value
    }
    return runtimeConfigs.value.filter(item => {
      const key = String(item?.key || '').toLowerCase()
      const desc = String(item?.description || '').toLowerCase()
      return key.includes(keyword) || desc.includes(keyword)
    })
  })

  const executeLogout = async () => {
    if (loggingOut.value) {
      return
    }

    loggingOut.value = true
    try {
      const remoteLoggedOut = await sessionStore.logout()
      if (remoteLoggedOut) {
        message.success('已退出登录')
      } else {
        message.warning('本地已退出登录，服务端退出请求失败')
      }
      await router.replace('/')
    } catch (error) {
      message.error(error?.message || '退出登录失败')
    } finally {
      loggingOut.value = false
    }
  }

  const confirmLogout = () => {
    dialog.warning({
      title: '退出登录',
      content: '确认退出当前账号吗？',
      positiveText: '退出登录',
      negativeText: '取消',
      onPositiveClick: executeLogout
    })
  }

  const isCurrentOperator = linuxDoUserId => {
    const current = String(sessionStore.user?.linuxDoUserId || '').trim()
    return current !== '' && current === String(linuxDoUserId || '').trim()
  }

  const formatTime = value => {
    if (!value) return '-'
    const date = new Date(value)
    if (Number.isNaN(date.getTime())) return '-'
    return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')} ${String(
      date.getHours()
    ).padStart(2, '0')}:${String(date.getMinutes()).padStart(2, '0')}:${String(date.getSeconds()).padStart(2, '0')}`
  }

  const loadAdminUsers = async () => {
    if (!canManageAdmins.value) {
      adminUsers.value = []
      return
    }
    adminLoading.value = true
    try {
      const result = await fetchAdminUsers(500)
      adminUsers.value = Array.isArray(result?.users) ? result.users : []
    } catch (error) {
      message.error(error?.message || '加载管理员列表失败')
    } finally {
      adminLoading.value = false
    }
  }

  const loadRuntimeConfigs = async () => {
    if (!canManageRuntimeConfigs.value) {
      runtimeConfigs.value = []
      return
    }
    runtimeConfigLoading.value = true
    try {
      const result = await fetchRuntimeConfigs({
        category: runtimeConfigFilterCategory.value.trim(),
        keyword: runtimeConfigKeyword.value.trim(),
        limit: 1000
      })
      runtimeConfigs.value = Array.isArray(result?.configs) ? result.configs : []
    } catch (error) {
      message.error(error?.message || '加载运行时配置失败')
    } finally {
      runtimeConfigLoading.value = false
    }
  }

  const loadRuntimeConfigAudits = async () => {
    if (!canManageRuntimeConfigs.value) {
      runtimeAudits.value = []
      return
    }
    runtimeAuditLoading.value = true
    try {
      const result = await fetchRuntimeConfigAudits({
        key: runtimeAuditKey.value.trim(),
        category: runtimeAuditCategory.value.trim(),
        limit: 200
      })
      runtimeAudits.value = Array.isArray(result?.logs) ? result.logs : []
    } catch (error) {
      message.error(error?.message || '加载运行时配置审计失败')
    } finally {
      runtimeAuditLoading.value = false
    }
  }

  const loadChatBlockedWords = async () => {
    if (!canModerateChat.value) {
      chatBlockedWords.value = []
      return
    }
    chatBlockedWordLoading.value = true
    try {
      const result = await fetchChatBlockedWords(true, 300)
      chatBlockedWords.value = Array.isArray(result?.words) ? result.words : []
    } catch (error) {
      message.error(error?.message || '加载违禁词失败')
    } finally {
      chatBlockedWordLoading.value = false
    }
  }

  const loadChatAdminMutes = async () => {
    if (!canModerateChat.value) {
      chatActiveMutes.value = []
      return
    }
    chatMuteListLoading.value = true
    try {
      const result = await fetchChatAdminMutes('', 200)
      chatActiveMutes.value = Array.isArray(result?.mutes) ? result.mutes : []
    } catch (error) {
      message.error(error?.message || '加载禁言列表失败')
    } finally {
      chatMuteListLoading.value = false
    }
  }

  const loadChatReports = async () => {
    if (!canModerateChat.value) {
      chatReports.value = []
      return
    }
    chatReportLoading.value = true
    try {
      const result = await fetchChatAdminReports(chatReportFilterStatus.value, 200)
      chatReports.value = Array.isArray(result?.reports) ? result.reports : []
    } catch (error) {
      message.error(error?.message || '加载举报列表失败')
    } finally {
      chatReportLoading.value = false
    }
  }

  const submitChatMute = async () => {
    if (!canModerateChat.value) {
      message.error('当前角色无聊天管理权限')
      return
    }
    const targetLinuxDoUserId = chatMuteTargetLinuxDoUserId.value.trim()
    if (!targetLinuxDoUserId) {
      message.warning('请输入目标 LinuxDo 用户 ID')
      return
    }
    const durationMinutes = Math.max(1, Math.floor(Number(chatMuteDurationMinutes.value || 0)))
    if (durationMinutes <= 0) {
      message.warning('禁言时长必须大于 0')
      return
    }

    chatModerationSubmitting.value = true
    try {
      const result = await muteChatUser(targetLinuxDoUserId, durationMinutes, chatMuteReason.value.trim())
      message.success(result?.message || '禁言成功')
      await Promise.all([loadChatAdminMutes(), loadChatReports()])
    } catch (error) {
      message.error(error?.message || '禁言失败')
    } finally {
      chatModerationSubmitting.value = false
    }
  }

  const submitChatUnmute = async targetLinuxDoUserIdRaw => {
    if (!canModerateChat.value) {
      message.error('当前角色无聊天管理权限')
      return
    }
    const targetLinuxDoUserId = String(targetLinuxDoUserIdRaw || '').trim()
    if (!targetLinuxDoUserId) {
      message.warning('请输入目标 LinuxDo 用户 ID')
      return
    }

    chatModerationSubmitting.value = true
    try {
      const result = await unmuteChatUser(targetLinuxDoUserId)
      if (result?.updated) {
        message.success(result?.message || '解除禁言成功')
      } else {
        message.warning('目标当前未被禁言')
      }
      await Promise.all([loadChatAdminMutes(), loadChatReports()])
    } catch (error) {
      message.error(error?.message || '解除禁言失败')
    } finally {
      chatModerationSubmitting.value = false
    }
  }

  const reviewChatReport = async (reportId, status) => {
    if (!canModerateChat.value) {
      message.error('当前角色无聊天管理权限')
      return
    }
    chatModerationSubmitting.value = true
    try {
      await reviewChatAdminReport(reportId, status, chatReportReviewNote.value.trim())
      message.success('举报审核已更新')
      await loadChatReports()
    } catch (error) {
      message.error(error?.message || '举报审核失败')
    } finally {
      chatModerationSubmitting.value = false
    }
  }

  const submitAdminUser = async () => {
    if (!canManageAdmins.value) {
      message.error('当前角色无管理员管理权限')
      return
    }
    const linuxDoUserId = newAdminLinuxDoUserId.value.trim()
    if (!linuxDoUserId) {
      message.warning('请先输入 LinuxDo 用户 ID')
      return
    }
    adminSubmitting.value = true
    try {
      await upsertAdminUser(linuxDoUserId, newAdminRole.value, newAdminNote.value.trim())
      message.success('管理员已更新')
      newAdminLinuxDoUserId.value = ''
      newAdminRole.value = 'ops_admin'
      newAdminNote.value = ''
      await loadAdminUsers()
    } catch (error) {
      message.error(error?.message || '更新管理员失败')
    } finally {
      adminSubmitting.value = false
    }
  }

  const removeAdminUserAction = async linuxDoUserId => {
    if (!canManageAdmins.value) {
      message.error('当前角色无管理员管理权限')
      return
    }
    const target = String(linuxDoUserId || '').trim()
    if (!target) return
    adminSubmitting.value = true
    try {
      await removeAdminUser(target)
      message.success('管理员已移除')
      await loadAdminUsers()
    } catch (error) {
      message.error(error?.message || '移除管理员失败')
    } finally {
      adminSubmitting.value = false
    }
  }

  const editRuntimeConfig = config => {
    runtimeConfigForm.value = {
      key: String(config?.key || ''),
      value: String(config?.value ?? ''),
      valueType: String(config?.valueType || 'string'),
      category: String(config?.category || 'general'),
      description: String(config?.description || '')
    }
  }

  const submitRuntimeConfig = async () => {
    if (!canManageRuntimeConfigs.value) {
      message.error('当前角色无运行时配置权限')
      return
    }
    const payload = {
      key: runtimeConfigForm.value.key.trim(),
      value: String(runtimeConfigForm.value.value ?? '').trim(),
      valueType: runtimeConfigForm.value.valueType,
      category: runtimeConfigForm.value.category.trim(),
      description: runtimeConfigForm.value.description.trim()
    }
    if (!payload.key) {
      message.warning('请先输入配置 key')
      return
    }
    if (payload.value === '') {
      message.warning('请先输入配置 value')
      return
    }
    runtimeConfigSubmitting.value = true
    try {
      await upsertRuntimeConfig(payload)
      message.success('运行时配置已更新')
      await Promise.all([loadRuntimeConfigs(), loadRuntimeConfigAudits()])
    } catch (error) {
      message.error(error?.message || '更新运行时配置失败')
    } finally {
      runtimeConfigSubmitting.value = false
    }
  }

  const submitBlockedWord = async () => {
    if (!canModerateChat.value) {
      message.error('当前角色无聊天管理权限')
      return
    }
    const word = adminBlockedWord.value.trim()
    if (!word) {
      message.warning('请输入违禁词')
      return
    }
    chatBlockedWordSubmitting.value = true
    try {
      await upsertChatBlockedWord(word, Boolean(adminBlockedWordEnabled.value))
      message.success('违禁词更新成功')
      adminBlockedWord.value = ''
      await loadChatBlockedWords()
    } catch (error) {
      message.error(error?.message || '违禁词更新失败')
    } finally {
      chatBlockedWordSubmitting.value = false
    }
  }

  const toggleBlockedWord = async item => {
    if (!canModerateChat.value) {
      message.error('当前角色无聊天管理权限')
      return
    }
    const word = String(item?.word || '').trim()
    if (!word) return
    chatBlockedWordSubmitting.value = true
    try {
      await upsertChatBlockedWord(word, !Boolean(item?.enabled))
      message.success('违禁词状态已更新')
      await loadChatBlockedWords()
    } catch (error) {
      message.error(error?.message || '违禁词状态更新失败')
    } finally {
      chatBlockedWordSubmitting.value = false
    }
  }

  const removeBlockedWord = async wordRaw => {
    if (!canModerateChat.value) {
      message.error('当前角色无聊天管理权限')
      return
    }
    const word = String(wordRaw || '').trim()
    if (!word) return
    chatBlockedWordSubmitting.value = true
    try {
      await deleteChatBlockedWord(word)
      message.success('违禁词已删除')
      await loadChatBlockedWords()
    } catch (error) {
      message.error(error?.message || '违禁词删除失败')
    } finally {
      chatBlockedWordSubmitting.value = false
    }
  }

  const runtimeAuditOperatorDisplay = log => {
    const linuxDoUserId = String(log?.operatorLinuxDoUserId || '').trim()
    const username = String(log?.operatorUsername || '').trim()
    if (linuxDoUserId && username) {
      return `${username} (${linuxDoUserId})`
    }
    if (linuxDoUserId) {
      return linuxDoUserId
    }
    if (username) {
      return username
    }
    return '-'
  }

  const formatAuditValue = value => {
    const text = String(value ?? '').trim()
    if (!text) {
      return '-'
    }
    if (text.length > 80) {
      return `${text.slice(0, 77)}...`
    }
    return text
  }

  const formatAuditMeta = (valueType, category) => {
    const typeText = String(valueType || '').trim()
    const categoryText = String(category || '').trim()
    if (!typeText && !categoryText) {
      return '-'
    }
    if (typeText && categoryText) {
      return `${typeText} / ${categoryText}`
    }
    return typeText || categoryText
  }

  const formatAdminRole = role => {
    const value = String(role || '').trim()
    if (value === 'super_admin') return '超管'
    if (value === 'ops_admin') return '运营'
    if (value === 'chat_admin') return '聊天管理'
    return value || '-'
  }

  const formatChatReportStatus = status => {
    const value = String(status || '').trim().toLowerCase()
    if (value === 'approved') return '已通过'
    if (value === 'rejected') return '已驳回'
    return '待审核'
  }

  onMounted(async () => {
    const tasks = []
    if (canManageAdmins.value) {
      tasks.push(loadAdminUsers())
    }
    if (canManageRuntimeConfigs.value) {
      tasks.push(loadRuntimeConfigs(), loadRuntimeConfigAudits())
    }
    if (canModerateChat.value) {
      tasks.push(loadChatBlockedWords(), loadChatAdminMutes(), loadChatReports())
    }
    if (tasks.length > 0) {
      await Promise.all(tasks)
    }
  })
</script>
