<template>
  <div class="log-panel-container">
    <div class="log-header" v-if="title">
      <span class="log-title">{{ title }}</span>
      <n-button quaternary size="tiny" @click="clearLogs" type="error">清空</n-button>
    </div>
    
    <div class="log-body">
      <n-scrollbar ref="scrollRef" trigger="none" style="max-height: 240px">
        <div class="log-list" v-if="logs.length">
          <div v-for="(log, index) in logs" :key="index" class="log-entry" :class="[`type-${log.type}`]">
            <span class="log-time">[{{ log.time.split(' ')[1] || log.time }}]</span>
            <span class="log-content">{{ log.content }}</span>
          </div>
        </div>
        <n-empty v-else description="暂无传书" size="small" style="padding: 20px 0" />
      </n-scrollbar>
    </div>
  </div>
</template>

<script setup>
  import { ref, onMounted, onUnmounted, watch } from 'vue'

  const props = defineProps({
    title: {
      type: String,
      default: '系统日志'
    }
  })

  const logs = ref([])
  const scrollRef = ref(null)
  const logWorker = ref(null)

  onMounted(() => {
    logWorker.value = new Worker(new URL('../workers/log.js', import.meta.url), { type: 'module' })
    logWorker.value.onmessage = e => {
      if (e.data.type === 'LOGS_UPDATED') {
        logs.value = e.data.logs
        scrollToBottom()
      }
    }
  })

  onUnmounted(() => {
    if (logWorker.value) logWorker.value.terminate()
  })

  const addLog = (type, content) => {
    if (logWorker.value) {
      logWorker.value.postMessage({ type: 'ADD_LOG', data: { type, content } })
    }
  }

  const clearLogs = () => {
    if (logWorker.value) logWorker.value.postMessage({ type: 'CLEAR_LOGS' })
  }

  const scrollToBottom = () => {
    setTimeout(() => {
      if (scrollRef.value) {
        scrollRef.value.scrollTo({ top: 99999, behavior: 'smooth' })
      }
    }, 60)
  }

  watch(() => logs.value.length, scrollToBottom)

  defineExpose({ addLog, clearLogs })
</script>

<style scoped>
.log-panel-container {
  background: var(--panel-bg);
  border: 1px solid var(--panel-border);
  border-radius: 16px;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.log-header {
  padding: 8px 16px;
  background: rgba(0,0,0,0.03);
  border-bottom: 1px solid var(--panel-border);
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.log-title { font-size: 12px; font-weight: bold; color: var(--ink-sub); opacity: 0.8; }

.log-body { padding: 12px; }

.log-list { display: flex; flex-direction: column; gap: 6px; }

.log-entry {
  font-size: 13px;
  line-height: 1.5;
  display: flex;
  gap: 8px;
  padding: 4px 8px;
  border-radius: 6px;
  transition: background 0.2s;
}

.log-entry:hover { background: rgba(0,0,0,0.02); }

.log-time {
  font-family: ui-monospace, monospace;
  font-size: 11px;
  color: var(--ink-sub);
  opacity: 0.5;
  white-space: nowrap;
}

.log-content { flex: 1; word-break: break-all; }

/* 日志类型颜色 */
.type-info .log-content { color: var(--ink-main); }
.type-success .log-content { color: #18a058; font-weight: 500; }
.type-warning .log-content { color: #f0a020; }
.type-error .log-content { color: #d03050; }

@media (max-width: 768px) {
  .log-entry { font-size: 12px; padding: 2px 4px; }
}
</style>
