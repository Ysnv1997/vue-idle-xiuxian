<template>
  <div class="settings-container">
    <n-card title="游戏设置">
      <template #header-extra>游戏版本{{ version }}</template>
      <n-space vertical>

      </n-space>
    </n-card>
    <n-modal preset="dialog" title="玩家交流群" v-model:show="qq">
      <n-card :bordered="false" size="huge" role="dialog" aria-modal="true">
        <n-space vertical>
          <n-input value="920930589" readonly type="text" />
        </n-space>
      </n-card>
    </n-modal>
  </div>
</template>

<script setup>
  import { ref } from 'vue'

  const version = __APP_VERSION__
  const qq = ref(false)
</script>

<style scoped></style>
