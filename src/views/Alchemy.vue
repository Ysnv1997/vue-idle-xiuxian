<template>
  <section class="page-view alchemy-view">
    <header class="page-head">
      <p class="page-eyebrow">丹鼎炉火</p>
      <h2>丹药炼制</h2>
      <p class="page-desc">选择已掌握丹方，消耗灵草炼制丹药。</p>
    </header>

    <n-card :bordered="false" class="page-card">
      <n-space vertical>
        <template v-if="unlockedRecipes.length > 0">
          <n-divider>丹方选择</n-divider>
          <n-grid :cols="2" :x-gap="12">
            <n-grid-item v-for="recipe in unlockedRecipes" :key="recipe.id">
              <n-card :title="recipe.name" size="small">
                <n-space vertical>
                  <n-text depth="3">{{ recipe.description }}</n-text>
                  <n-space>
                    <n-tag type="info">{{ pillGrades[recipe.grade].name }}</n-tag>
                    <n-tag type="warning">{{ pillTypes[recipe.type].name }}</n-tag>
                  </n-space>
                  <n-button
                    @click="selectRecipe(recipe)"
                    block
                    :type="selectedRecipe?.id === recipe.id ? 'primary' : 'default'"
                  >
                    {{ selectedRecipe?.id === recipe.id ? '已选择' : '选择' }}
                  </n-button>
                </n-space>
              </n-card>
            </n-grid-item>
          </n-grid>
        </template>
        <n-space vertical v-else>
          <n-empty description="暂未掌握任何丹方" />
        </n-space>
        <template v-if="selectedRecipe">
          <n-divider>材料需求</n-divider>
          <n-list>
            <n-list-item v-for="material in selectedRecipe.materials" :key="material.herb">
              <n-space justify="space-between">
                <n-space>
                  <span>{{ getHerbName(material.herb) }}</span>
                  <n-tag size="small">需要数量: {{ material.count }}</n-tag>
                </n-space>
                <n-tag
                  :type="getMaterialStatus(material) === `${material.count}/${material.count}` ? 'success' : 'warning'"
                >
                  拥有: {{ getMaterialStatus(material) }}
                </n-tag>
              </n-space>
            </n-list-item>
          </n-list>
        </template>
        <template v-if="selectedRecipe">
          <n-divider>效果预览</n-divider>
          <n-descriptions bordered :column="2">
            <n-descriptions-item label="丹药介绍">
              {{ selectedRecipe.description }}
            </n-descriptions-item>
            <n-descriptions-item label="效果数值">+{{ (currentEffect.value * 100).toFixed(1) }}%</n-descriptions-item>
            <n-descriptions-item label="持续时间">{{ Math.floor(currentEffect.duration / 60) }}分钟</n-descriptions-item>
            <n-descriptions-item label="成功率">{{ (currentEffect.successRate * 100).toFixed(1) }}%</n-descriptions-item>
          </n-descriptions>
        </template>
        <n-button
          class="craft-button"
          type="primary"
          block
          v-if="selectedRecipe"
          :disabled="!selectedRecipe || !checkMaterials(selectedRecipe) || isSubmitting"
          :loading="isSubmitting"
          @click="craftPill"
        >
          {{ !checkMaterials(selectedRecipe) ? '材料不足' : '开始炼制' }}
        </n-button>
      </n-space>
      <log-panel v-if="selectedRecipe" ref="logRef" title="炼丹日志" />
    </n-card>
  </section>
</template>

<script setup>
  import { ref, computed } from 'vue'
  import { usePlayerStore } from '../stores/player'
  import { pillRecipes, pillGrades, pillTypes, calculatePillEffect } from '../plugins/pills'
  import { herbs } from '../plugins/herbs'
  import LogPanel from '../components/LogPanel.vue'
  import { craftAlchemyPill } from '../api/modules/game'

  const playerStore = usePlayerStore()
  const logRef = ref(null)
  const isSubmitting = ref(false)

  // 当前选择的丹方
  const selectedRecipe = ref(null)

  // 已解锁的丹方列表
  const unlockedRecipes = computed(() => {
    return pillRecipes.filter(recipe => playerStore.pillRecipes.includes(recipe.id))
  })

  // 选择丹方
  const selectRecipe = recipe => {
    selectedRecipe.value = recipe
  }

  // 检查材料是否充足
  const checkMaterials = recipe => {
    if (!recipe) return false
    return recipe.materials.every(material => {
      const count = playerStore.herbs.filter(h => h.id === material.herb).length
      return count >= material.count
    })
  }

  // 获取材料状态文本
  const getMaterialStatus = material => {
    const count = playerStore.herbs.filter(h => h.id === material.herb).length
    return `${count}/${material.count}`
  }

  // 获取灵草名称
  const getHerbName = herbId => {
    const herb = herbs.find(h => h.id === herbId)
    return herb ? herb.name : herbId
  }

  // 计算当前效果
  const currentEffect = computed(() => {
    if (!selectedRecipe.value) return null
    return calculatePillEffect(selectedRecipe.value, playerStore.level)
  })

  // 炼制丹药
  const playCraftAnimation = success => {
    const btn = document.querySelector('.craft-button')
    if (!btn) return
    if (success) {
      btn.classList.add('success-animation')
      setTimeout(() => {
        btn.classList.remove('success-animation')
      }, 1000)
      return
    }
    btn.classList.add('fail-animation')
    setTimeout(() => {
      btn.classList.remove('fail-animation')
    }, 1000)
  }

  const craftPill = async () => {
    if (!selectedRecipe.value) return
    try {
      isSubmitting.value = true
      const result = await craftAlchemyPill(selectedRecipe.value.id)
      if (result?.snapshot) {
        playerStore.applyServerSnapshot(result.snapshot)
      }
      if (result?.success) {
        logRef.value?.addLog('success', result.message || '炼制成功！')
        playCraftAnimation(true)
      } else {
        logRef.value?.addLog('error', `炼制失败：${result?.message || '炼制失败'}`)
        playCraftAnimation(false)
      }
    } catch (error) {
      const code = error?.payload?.error
      if (code === 'recipe locked') {
        logRef.value?.addLog('error', '炼制失败：未掌握该丹方')
      } else if (code === 'insufficient materials') {
        logRef.value?.addLog('error', '炼制失败：材料不足')
      } else {
        logRef.value?.addLog('error', `炼制失败：${error?.message || '请求失败'}`)
      }
      playCraftAnimation(false)
    } finally {
      isSubmitting.value = false
    }
  }
</script>

<style scoped>
  :deep(.n-space) {
    width: 100%;
  }

  :deep(.n-descriptions-table-content) {
    word-break: break-word;
  }

  .n-button {
    margin-bottom: 12px;
  }

  .n-collapse {
    margin-top: 12px;
  }

  .craft-button {
    position: relative;
    overflow: hidden;
  }

  @keyframes success-ripple {
    0% {
      transform: scale(0);
      opacity: 1;
    }
    100% {
      transform: scale(4);
      opacity: 0;
    }
  }

  @keyframes fail-shake {
    0%,
    100% {
      transform: translateX(0);
    }
    25% {
      transform: translateX(-10px);
    }
    75% {
      transform: translateX(10px);
    }
  }

  .success-animation::after {
    content: '';
    position: absolute;
    top: 50%;
    left: 50%;
    width: 20px;
    height: 20px;
    background: rgba(0, 255, 0, 0.3);
    border-radius: 50%;
    transform: translate(-50%, -50%);
    animation: success-ripple 1s ease-out;
  }

  .fail-animation {
    animation: fail-shake 0.5s ease-in-out;
  }

  @media (max-width: 768px) {
    :deep(.n-grid) {
      grid-template-columns: minmax(0, 1fr) !important;
    }

    :deep(.n-list-item) {
      padding-left: 0;
      padding-right: 0;
    }

    :deep(.n-descriptions) {
      --n-td-padding: 8px;
    }

    .craft-button {
      position: sticky;
      bottom: 8px;
      z-index: 3;
    }
  }
</style>
