<template>
  <section class="page-view dungeon-view">
    <header class="page-head">
      <p class="page-eyebrow">秘境试炼</p>
      <h2>秘境探索</h2>
      <p class="page-desc">选择难度后进入战斗流程，并在层间挑选增益。</p>
    </header>

    <n-card :bordered="false" class="page-card">
      <n-space vertical>
        <n-space class="control-row" align="center" justify="space-between">
          <n-select
            v-model:value="playerStore.dungeonDifficulty"
            @update:value="handleUpdateValue"
            placeholder="请选择难度"
            :options="dungeonOptions"
            style="width: 120px"
            :disabled="dungeonState.inCombat || dungeonState.showingOptions"
          />
          <n-button
            type="primary"
            @click="startDungeon"
            :disabled="dungeonState.inCombat || dungeonState.showingOptions"
          >
            开始探索
          </n-button>
        </n-space>
        <!-- 层数显示 -->
        <n-statistic label="当前层数" :value="dungeonState.floor" />
        <!-- 选项界面 -->
        <n-card v-if="dungeonState.showingOptions" title="选择增益">
          <template #header-extra>
            <n-space>
              <n-button type="primary" @click="handleRefreshOptions" :disabled="refreshNumber === 0">
                刷新增益({{ refreshNumber }})
              </n-button>
            </n-space>
          </template>
          <div class="option-cards">
            <div
              v-for="option in dungeonState.currentOptions"
              :key="option.id"
              class="option-card"
              :style="{ borderColor: getOptionColor(option.type).color }"
              @click="selectOption(option)"
            >
              <div class="option-name">{{ option.name }}</div>
              <div class="option-description">{{ option.description }}</div>
              <div class="option-quality" :style="{ color: getOptionColor(option.type).color }">
                {{ getOptionColor(option.type).name }}
              </div>
            </div>
          </div>
        </n-card>
        <!-- 战斗界面 -->
        <template v-if="dungeonState.combatManager">
          <n-card :bordered="false">
            <n-divider>
              {{ dungeonState.combatManager.round }} / {{ dungeonState.combatManager.maxRounds }}回合
            </n-divider>
            <!-- 添加战斗场景 -->
            <div class="combat-scene">
              <div class="character player" :class="{ attack: playerAttacking, hurt: playerHurt }">
                <div v-if="playerAttacking" class="attack-effect player-effect"></div>
                <n-button class="character-name" type="info" dashed @click="infoCliclk('player')">
                  {{ dungeonState.combatManager.player.name }}
                </n-button>
                <div class="character-avatar player-avatar">
                  {{ dungeonState.combatManager.player.name[0] }}
                </div>
                <div class="health-bar">
                  <div
                    class="health-fill"
                    :style="{
                      width: `${
                        (dungeonState.combatManager.player.currentHealth /
                          dungeonState.combatManager.player.stats.maxHealth) *
                        100
                      }%`
                    }"
                  ></div>
                </div>
              </div>
              <div class="character enemy" :class="{ attack: enemyAttacking, hurt: enemyHurt }">
                <div v-if="enemyAttacking" class="attack-effect enemy-effect"></div>
                <n-button class="character-name" type="error" dashed @click="infoCliclk('enemy')">
                  {{ dungeonState.combatManager.enemy.name }}
                </n-button>
                <div class="character-avatar enemy-avatar">
                  {{ dungeonState.combatManager.enemy.name[0] }}
                </div>
                <div class="health-bar">
                  <div
                    class="health-fill"
                    :style="{
                      width: `${
                        (dungeonState.combatManager.enemy.currentHealth /
                          dungeonState.combatManager.enemy.stats.maxHealth) *
                        100
                      }%`
                    }"
                  ></div>
                </div>
              </div>
            </div>
            <n-modal
              v-model:show="infoShow"
              preset="dialog"
              :title="`${
                infoType == 'player' ? dungeonState.combatManager.player.name : dungeonState.combatManager.enemy.name
              }的属性`"
            >
              <n-card :bordered="false">
                <!-- 玩家属性 -->
                <template v-if="infoType == 'player'">
                  <n-divider>基础属性</n-divider>
                  <n-descriptions bordered :column="2">
                    <n-descriptions-item label="生命值">
                      {{ dungeonState.combatManager.player.currentHealth.toFixed(1) }} /
                      {{ dungeonState.combatManager.player.stats.maxHealth.toFixed(1) }}
                    </n-descriptions-item>
                    <n-descriptions-item label="攻击力">
                      {{ dungeonState.combatManager.player.stats.damage.toFixed(1) }}
                    </n-descriptions-item>
                    <n-descriptions-item label="防御力">
                      {{ dungeonState.combatManager.player.stats.defense.toFixed(1) }}
                    </n-descriptions-item>
                    <n-descriptions-item label="速度">
                      {{ dungeonState.combatManager.player.stats.speed.toFixed(1) }}
                    </n-descriptions-item>
                  </n-descriptions>
                  <n-divider>战斗属性</n-divider>
                  <n-descriptions bordered :column="3">
                    <n-descriptions-item label="暴击率">
                      {{ (dungeonState.combatManager.player.stats.critRate * 100).toFixed(1) }}%
                    </n-descriptions-item>
                    <n-descriptions-item label="连击率">
                      {{ (dungeonState.combatManager.player.stats.comboRate * 100).toFixed(1) }}%
                    </n-descriptions-item>
                    <n-descriptions-item label="反击率">
                      {{ (dungeonState.combatManager.player.stats.counterRate * 100).toFixed(1) }}%
                    </n-descriptions-item>
                    <n-descriptions-item label="眩晕率">
                      {{ (dungeonState.combatManager.player.stats.stunRate * 100).toFixed(1) }}%
                    </n-descriptions-item>
                    <n-descriptions-item label="闪避率">
                      {{ (dungeonState.combatManager.player.stats.dodgeRate * 100).toFixed(1) }}%
                    </n-descriptions-item>
                    <n-descriptions-item label="吸血率">
                      {{ (dungeonState.combatManager.player.stats.vampireRate * 100).toFixed(1) }}%
                    </n-descriptions-item>
                  </n-descriptions>
                  <n-divider>战斗抗性</n-divider>
                  <n-descriptions bordered :column="3">
                    <n-descriptions-item label="抗暴击">
                      {{ (dungeonState.combatManager.player.stats.critResist * 100).toFixed(1) }}%
                    </n-descriptions-item>
                    <n-descriptions-item label="抗连击">
                      {{ (dungeonState.combatManager.player.stats.comboResist * 100).toFixed(1) }}%
                    </n-descriptions-item>
                    <n-descriptions-item label="抗反击">
                      {{ (dungeonState.combatManager.player.stats.counterResist * 100).toFixed(1) }}%
                    </n-descriptions-item>
                    <n-descriptions-item label="抗眩晕">
                      {{ (dungeonState.combatManager.player.stats.stunResist * 100).toFixed(1) }}%
                    </n-descriptions-item>
                    <n-descriptions-item label="抗闪避">
                      {{ (dungeonState.combatManager.player.stats.dodgeResist * 100).toFixed(1) }}%
                    </n-descriptions-item>
                    <n-descriptions-item label="抗吸血">
                      {{ (dungeonState.combatManager.player.stats.vampireResist * 100).toFixed(1) }}%
                    </n-descriptions-item>
                  </n-descriptions>
                  <n-divider>特殊属性</n-divider>
                  <n-descriptions bordered :column="4">
                    <n-descriptions-item label="强化治疗">
                      {{ (dungeonState.combatManager.player.stats.healBoost * 100).toFixed(1) }}%
                    </n-descriptions-item>
                    <n-descriptions-item label="强化爆伤">
                      {{ (dungeonState.combatManager.player.stats.critDamageBoost * 100).toFixed(1) }}%
                    </n-descriptions-item>
                    <n-descriptions-item label="弱化爆伤">
                      {{ (dungeonState.combatManager.player.stats.critDamageReduce * 100).toFixed(1) }}%
                    </n-descriptions-item>
                    <n-descriptions-item label="最终增伤">
                      {{ (dungeonState.combatManager.player.stats.finalDamageBoost * 100).toFixed(1) }}%
                    </n-descriptions-item>
                    <n-descriptions-item label="最终减伤">
                      {{ (dungeonState.combatManager.player.stats.finalDamageReduce * 100).toFixed(1) }}%
                    </n-descriptions-item>
                    <n-descriptions-item label="战斗属性提升">
                      {{ (dungeonState.combatManager.player.stats.combatBoost * 100).toFixed(1) }}%
                    </n-descriptions-item>
                    <n-descriptions-item label="战斗抗性提升">
                      {{ (dungeonState.combatManager.player.stats.resistanceBoost * 100).toFixed(1) }}%
                    </n-descriptions-item>
                  </n-descriptions>
                </template>
                <!-- 敌人属性 -->
                <template v-else>
                  <n-divider>基础属性</n-divider>
                  <n-descriptions bordered :column="2">
                    <n-descriptions-item label="生命值">
                      {{ dungeonState.combatManager.enemy.currentHealth.toFixed(1) }} /
                      {{ dungeonState.combatManager.enemy.stats.maxHealth.toFixed(1) }}
                    </n-descriptions-item>
                    <n-descriptions-item label="攻击力">
                      {{ dungeonState.combatManager.enemy.stats.damage.toFixed(1) }}
                    </n-descriptions-item>
                    <n-descriptions-item label="防御力">
                      {{ dungeonState.combatManager.enemy.stats.defense.toFixed(1) }}
                    </n-descriptions-item>
                    <n-descriptions-item label="速度">
                      {{ dungeonState.combatManager.enemy.stats.speed.toFixed(1) }}
                    </n-descriptions-item>
                  </n-descriptions>
                  <n-divider>战斗属性</n-divider>
                  <n-descriptions bordered :column="3">
                    <n-descriptions-item label="暴击率">
                      {{ (dungeonState.combatManager.enemy.stats.critRate * 100).toFixed(1) }}%
                    </n-descriptions-item>
                    <n-descriptions-item label="连击率">
                      {{ (dungeonState.combatManager.enemy.stats.comboRate * 100).toFixed(1) }}%
                    </n-descriptions-item>
                    <n-descriptions-item label="反击率">
                      {{ (dungeonState.combatManager.enemy.stats.counterRate * 100).toFixed(1) }}%
                    </n-descriptions-item>
                    <n-descriptions-item label="眩晕率">
                      {{ (dungeonState.combatManager.enemy.stats.stunRate * 100).toFixed(1) }}%
                    </n-descriptions-item>
                    <n-descriptions-item label="闪避率">
                      {{ (dungeonState.combatManager.enemy.stats.dodgeRate * 100).toFixed(1) }}%
                    </n-descriptions-item>
                    <n-descriptions-item label="吸血率">
                      {{ (dungeonState.combatManager.enemy.stats.vampireRate * 100).toFixed(1) }}%
                    </n-descriptions-item>
                  </n-descriptions>
                  <n-divider>战斗抗性</n-divider>
                  <n-descriptions bordered :column="3">
                    <n-descriptions-item label="抗暴击">
                      {{ (dungeonState.combatManager.enemy.stats.critResist * 100).toFixed(1) }}%
                    </n-descriptions-item>
                    <n-descriptions-item label="抗连击">
                      {{ (dungeonState.combatManager.enemy.stats.comboResist * 100).toFixed(1) }}%
                    </n-descriptions-item>
                    <n-descriptions-item label="抗反击">
                      {{ (dungeonState.combatManager.enemy.stats.counterResist * 100).toFixed(1) }}%
                    </n-descriptions-item>
                    <n-descriptions-item label="抗眩晕">
                      {{ (dungeonState.combatManager.enemy.stats.stunResist * 100).toFixed(1) }}%
                    </n-descriptions-item>
                    <n-descriptions-item label="抗闪避">
                      {{ (dungeonState.combatManager.enemy.stats.dodgeResist * 100).toFixed(1) }}%
                    </n-descriptions-item>
                    <n-descriptions-item label="抗吸血">
                      {{ (dungeonState.combatManager.enemy.stats.vampireResist * 100).toFixed(1) }}%
                    </n-descriptions-item>
                  </n-descriptions>
                  <n-divider>特殊属性</n-divider>
                  <n-descriptions bordered :column="3">
                    <n-descriptions-item label="强化治疗">
                      {{ (dungeonState.combatManager.enemy.stats.healBoost * 100).toFixed(1) }}%
                    </n-descriptions-item>
                    <n-descriptions-item label="强化爆伤">
                      {{ (dungeonState.combatManager.enemy.stats.critDamageBoost * 100).toFixed(1) }}%
                    </n-descriptions-item>
                    <n-descriptions-item label="弱化爆伤">
                      {{ (dungeonState.combatManager.enemy.stats.critDamageReduce * 100).toFixed(1) }}%
                    </n-descriptions-item>
                    <n-descriptions-item label="最终增伤">
                      {{ (dungeonState.combatManager.enemy.stats.finalDamageBoost * 100).toFixed(1) }}%
                    </n-descriptions-item>
                    <n-descriptions-item label="最终减伤">
                      {{ (dungeonState.combatManager.enemy.stats.finalDamageReduce * 100).toFixed(1) }}%
                    </n-descriptions-item>
                    <n-descriptions-item label="战斗属性提升">
                      {{ (dungeonState.combatManager.enemy.stats.combatBoost * 100).toFixed(1) }}%
                    </n-descriptions-item>
                    <n-descriptions-item label="战斗抗性提升">
                      {{ (dungeonState.combatManager.enemy.stats.resistanceBoost * 100).toFixed(1) }}%
                    </n-descriptions-item>
                  </n-descriptions>
                </template>
              </n-card>
            </n-modal>
            <!-- 战斗日志 -->
            <log-panel ref="logRef" :messages="combatLog" style="margin-top: 16px" />
          </n-card>
        </template>
      </n-space>
    </n-card>
  </section>
</template>

<script setup>
import { computed, ref } from 'vue'
import { useMessage } from 'naive-ui'
import { usePlayerStore } from '../stores/player'
import LogPanel from '../components/LogPanel.vue'
import { dungeonStart, dungeonNextTurn } from '../api/modules/game'

const playerStore = usePlayerStore()
const message = useMessage()
const logRef = ref(null)

const playerAttacking = ref(false)
const playerHurt = ref(false)
const enemyAttacking = ref(false)
const enemyHurt = ref(false)
const infoShow = ref(false)
const infoType = ref('')
const isSubmitting = ref(false)
const serverDungeonRunning = ref(false)
const refreshNumber = ref(3)

const floorData = computed(() => {
  switch (playerStore.dungeonDifficulty) {
    case 1:
      return playerStore.dungeonHighestFloor
    case 2:
      return playerStore.dungeonHighestFloor_2
    case 5:
      return playerStore.dungeonHighestFloor_5
    case 10:
      return playerStore.dungeonHighestFloor_10
    case 100:
      return playerStore.dungeonHighestFloor_100
    default:
      return playerStore.dungeonHighestFloor
  }
})

const dungeonState = ref({
  floor: floorData.value,
  inCombat: false,
  showingOptions: false,
  currentOptions: [],
  combatManager: null
})

const combatLog = ref([])
const sleep = ms => new Promise(resolve => setTimeout(resolve, ms))
const battleLogIntervalMs = 480
const battleActionHoldMs = 220

const pushDungeonLog = content => {
  if (!content) return
  const type = content.includes('失败') || content.includes('损失') ? 'error' : content.includes('获得') ? 'success' : 'info'
  logRef.value?.addLog(type, content)
}

const parseDamageFromLog = content => {
  const matched = String(content || '').match(/造成([0-9]+(?:\.[0-9]+)?)点伤害/)
  return matched ? Number(matched[1]) : 0
}

const parseVampireHealFromLog = content => {
  const matched = String(content || '').match(/吸血恢复([0-9]+(?:\.[0-9]+)?)点生命值/)
  return matched ? Number(matched[1]) : 0
}

const clampHealth = (value, maxHealth) => {
  return Math.max(0, Math.min(Number(maxHealth || 0), Number(value || 0)))
}

const clearCombatFlags = () => {
  playerAttacking.value = false
  playerHurt.value = false
  enemyAttacking.value = false
  enemyHurt.value = false
}

const applyCombatPlaybackFromLog = content => {
  const text = String(content || '')
  const isAttackLog = text.includes('率先发起攻击') || text.includes('进行攻击')
  if (!isAttackLog || !dungeonState.value.combatManager) {
    return false
  }

  const isPlayerAttack = text.includes('修士')
  const wasDodged = text.includes('被闪避')
  const damage = wasDodged ? 0 : parseDamageFromLog(text)
  const vampireHeal = parseVampireHealFromLog(text)
  const manager = dungeonState.value.combatManager

  clearCombatFlags()
  if (isPlayerAttack) {
    playerAttacking.value = true
    if (!wasDodged) {
      enemyHurt.value = true
      manager.enemy.currentHealth = clampHealth(manager.enemy.currentHealth - damage, manager.enemy.stats.maxHealth)
    }
    if (vampireHeal > 0) {
      manager.player.currentHealth = clampHealth(manager.player.currentHealth + vampireHeal, manager.player.stats.maxHealth)
    }
  } else {
    enemyAttacking.value = true
    if (!wasDodged) {
      playerHurt.value = true
      manager.player.currentHealth = clampHealth(
        manager.player.currentHealth - damage,
        manager.player.stats.maxHealth
      )
    }
    if (vampireHeal > 0) {
      manager.enemy.currentHealth = clampHealth(manager.enemy.currentHealth + vampireHeal, manager.enemy.stats.maxHealth)
    }
  }
  return true
}

const playServerCombatLogs = async logs => {
  const items = Array.isArray(logs) ? logs : []
  for (const logContent of items) {
    const hasAction = applyCombatPlaybackFromLog(logContent)
    pushDungeonLog(logContent)
    if (hasAction) {
      await sleep(battleActionHoldMs)
      clearCombatFlags()
      await sleep(Math.max(0, battleLogIntervalMs - battleActionHoldMs))
    } else {
      await sleep(battleLogIntervalMs)
    }
  }
}

const buildServerDisplayStats = source => ({
  maxHealth: Number(source.maxHealth ?? 1),
  damage: Number(source.damage ?? 0),
  defense: Number(source.defense ?? 0),
  speed: Number(source.speed ?? 0),
  critRate: Number(source.critRate ?? 0),
  comboRate: Number(source.comboRate ?? 0),
  counterRate: Number(source.counterRate ?? 0),
  stunRate: Number(source.stunRate ?? 0),
  dodgeRate: Number(source.dodgeRate ?? 0),
  vampireRate: Number(source.vampireRate ?? 0),
  critResist: Number(source.critResist ?? 0),
  comboResist: Number(source.comboResist ?? 0),
  counterResist: Number(source.counterResist ?? 0),
  stunResist: Number(source.stunResist ?? 0),
  dodgeResist: Number(source.dodgeResist ?? 0),
  vampireResist: Number(source.vampireResist ?? 0),
  healBoost: Number(source.healBoost ?? 0),
  critDamageBoost: Number(source.critDamageBoost ?? 0),
  critDamageReduce: Number(source.critDamageReduce ?? 0),
  finalDamageBoost: Number(source.finalDamageBoost ?? 0),
  finalDamageReduce: Number(source.finalDamageReduce ?? 0),
  combatBoost: Number(source.combatBoost ?? 0),
  resistanceBoost: Number(source.resistanceBoost ?? 0)
})

const createServerCombatManager = floor => {
  const safeFloor = Math.max(1, Number(floor) || 1)
  const safeDifficulty = Math.max(1, Number(playerStore.dungeonDifficulty) || 1)
  const playerStats = buildServerDisplayStats({
    maxHealth: playerStore.baseAttributes.health,
    damage: playerStore.baseAttributes.attack,
    defense: playerStore.baseAttributes.defense,
    speed: playerStore.baseAttributes.speed,
    critRate: playerStore.combatAttributes.critRate,
    comboRate: playerStore.combatAttributes.comboRate,
    counterRate: playerStore.combatAttributes.counterRate,
    stunRate: playerStore.combatAttributes.stunRate,
    dodgeRate: playerStore.combatAttributes.dodgeRate,
    vampireRate: playerStore.combatAttributes.vampireRate,
    critResist: playerStore.combatResistance.critResist,
    comboResist: playerStore.combatResistance.comboResist,
    counterResist: playerStore.combatResistance.counterResist,
    stunResist: playerStore.combatResistance.stunResist,
    dodgeResist: playerStore.combatResistance.dodgeResist,
    vampireResist: playerStore.combatResistance.vampireResist,
    healBoost: playerStore.specialAttributes.healBoost,
    critDamageBoost: playerStore.specialAttributes.critDamageBoost,
    critDamageReduce: playerStore.specialAttributes.critDamageReduce,
    finalDamageBoost: playerStore.specialAttributes.finalDamageBoost,
    finalDamageReduce: playerStore.specialAttributes.finalDamageReduce,
    combatBoost: playerStore.specialAttributes.combatBoost,
    resistanceBoost: playerStore.specialAttributes.resistanceBoost
  })

  const enemyName = safeFloor % 10 === 0 ? '秘境首领' : safeFloor % 5 === 0 ? '秘境精英' : '秘境敌人'
  const enemyStats = buildServerDisplayStats({
    maxHealth: 100 + safeDifficulty * safeFloor * 200,
    damage: 8 + safeDifficulty * safeFloor * 2,
    defense: 3 + safeDifficulty * safeFloor * 2,
    speed: 5 + safeDifficulty * safeFloor * 2,
    critRate: 0.05 + safeDifficulty * safeFloor * 0.02,
    comboRate: 0.03 + safeDifficulty * safeFloor * 0.02,
    counterRate: 0.03 + safeDifficulty * safeFloor * 0.02,
    stunRate: 0.02 + safeDifficulty * safeFloor * 0.01,
    dodgeRate: 0.05 + safeDifficulty * safeFloor * 0.02,
    vampireRate: 0.02 + safeDifficulty * safeFloor * 0.01,
    critResist: 0.02 + safeDifficulty * safeFloor * 0.01,
    comboResist: 0.02 + safeDifficulty * safeFloor * 0.01,
    counterResist: 0.02 + safeDifficulty * safeFloor * 0.01,
    stunResist: 0.02 + safeDifficulty * safeFloor * 0.01,
    dodgeResist: 0.02 + safeDifficulty * safeFloor * 0.01,
    vampireResist: 0.02 + safeDifficulty * safeFloor * 0.01,
    healBoost: 0.05 + safeDifficulty * safeFloor * 0.02,
    critDamageBoost: 0.2 + safeDifficulty * safeFloor * 0.1,
    critDamageReduce: 0.1 + safeDifficulty * safeFloor * 0.05,
    finalDamageBoost: 0.05 + safeDifficulty * safeFloor * 0.02,
    finalDamageReduce: 0.05 + safeDifficulty * safeFloor * 0.02,
    combatBoost: 0.03 + safeDifficulty * safeFloor * 0.02,
    resistanceBoost: 0.03 + safeDifficulty * safeFloor * 0.02
  })

  return {
    round: 1,
    maxRounds: 10,
    player: {
      name: playerStore.name || '修士',
      currentHealth: playerStats.maxHealth,
      stats: playerStats
    },
    enemy: {
      name: enemyName,
      currentHealth: enemyStats.maxHealth,
      stats: enemyStats
    }
  }
}

const normalizeDungeonOptions = options => {
  if (!Array.isArray(options)) return []
  return options
    .map(option => ({
      id: option?.id,
      name: option?.name,
      description: option?.description,
      type: option?.type || 'common'
    }))
    .filter(option => option.id && option.name)
}

const applyServerOptionState = result => {
  dungeonState.value.inCombat = false
  dungeonState.value.showingOptions = true
  dungeonState.value.currentOptions = normalizeDungeonOptions(result?.options)
  refreshNumber.value = Number(result?.refreshCount ?? 0)
  if (result?.message) {
    pushDungeonLog(result.message)
  }
}

const resetDungeonViewState = () => {
  clearCombatFlags()
  dungeonState.value = {
    ...dungeonState.value,
    inCombat: false,
    showingOptions: false,
    currentOptions: [],
    combatManager: null
  }
}

const runServerTurns = async initialResult => {
  let pendingResult = initialResult || null
  while (serverDungeonRunning.value) {
    const turnResult = pendingResult || (await dungeonNextTurn())
    pendingResult = null
    if (turnResult?.snapshot) {
      playerStore.applyServerSnapshot(turnResult.snapshot)
    }

    const currentFloor = Number(turnResult?.floor ?? dungeonState.value.floor)
    if (Number.isFinite(currentFloor) && currentFloor > 0) {
      dungeonState.value.floor = currentFloor
      dungeonState.value.combatManager = createServerCombatManager(currentFloor)
    }

    const turnLogs = Array.isArray(turnResult?.logs) ? turnResult.logs : []
    await playServerCombatLogs(turnLogs)

    if (turnResult?.state === 'option_required' || turnResult?.needsOption) {
      applyServerOptionState(turnResult)
      return
    }

    dungeonState.value.showingOptions = false
    dungeonState.value.currentOptions = []

    if (turnResult?.state === 'victory') {
      dungeonState.value.inCombat = true
      message.success(turnResult?.message || `击败了第 ${dungeonState.value.floor} 层的敌人！`)
      await sleep(180)
      continue
    }

    if (turnResult?.state === 'defeat') {
      dungeonState.value.inCombat = false
      serverDungeonRunning.value = false
      clearCombatFlags()
      message.error(turnResult?.message || `在第 ${dungeonState.value.floor} 层被击败了...`)
      return
    }

    if (turnResult?.message) {
      pushDungeonLog(turnResult.message)
    }
    serverDungeonRunning.value = false
    dungeonState.value.inCombat = false
    return
  }
}

const runServerDungeon = async () => {
  const startResult = await dungeonStart(playerStore.dungeonDifficulty)
  if (startResult?.snapshot) {
    playerStore.applyServerSnapshot(startResult.snapshot)
  }
  const startFloor = Number(startResult?.currentFloor ?? floorData.value)
  dungeonState.value = {
    floor: startFloor,
    inCombat: true,
    showingOptions: false,
    currentOptions: [],
    combatManager: createServerCombatManager(startFloor + 1)
  }
  serverDungeonRunning.value = true
  await sleep(0)
  if (startResult?.message) {
    pushDungeonLog(startResult.message)
    await sleep(260)
  }

  if (startResult?.state === 'option_required' || startResult?.needsOption) {
    applyServerOptionState(startResult)
    return
  }

  await runServerTurns()
}

const getOptionColor = type => {
  const types = {
    epic: {
      name: '史诗',
      color: '#e91e63'
    },
    rare: {
      name: '稀有',
      color: '#2196f3'
    },
    common: {
      name: '普通',
      color: '#4caf50'
    }
  }
  return types[type]
}

const startDungeon = async () => {
  if (isSubmitting.value) return
  try {
    isSubmitting.value = true
    serverDungeonRunning.value = false
    resetDungeonViewState()
    logRef.value?.clearLogs()
    infoShow.value = false
    infoType.value = ''
    await runServerDungeon()
  } catch (error) {
    serverDungeonRunning.value = false
    const code = error?.payload?.error
    if (code === 'invalid dungeon difficulty') {
      message.error('秘境难度无效，请重新选择')
    } else if (code === 'dungeon run not active') {
      message.error('当前没有进行中的秘境探索')
    } else if (code === 'invalid dungeon option') {
      message.error('增益选项无效，请重新选择')
    } else if (code === 'dungeon refresh exhausted') {
      message.error('刷新次数不足')
    } else {
      message.error(error?.message || '秘境探索失败')
    }
  } finally {
    isSubmitting.value = false
  }
}

const selectOption = async option => {
  if (!serverDungeonRunning.value || isSubmitting.value) return
  try {
    isSubmitting.value = true
    dungeonState.value.showingOptions = false
    dungeonState.value.currentOptions = []
    dungeonState.value.inCombat = true
    const turnResult = await dungeonNextTurn({ selectedOptionId: option.id })
    await runServerTurns(turnResult)
  } catch (error) {
    const code = error?.payload?.error
    if (code === 'invalid dungeon option') {
      message.error('增益选项无效，请重新选择')
    } else if (code === 'dungeon run not active') {
      serverDungeonRunning.value = false
      message.error('当前没有进行中的秘境探索')
    } else {
      message.error(error?.message || '选择增益失败')
    }
  } finally {
    isSubmitting.value = false
  }
}

const infoCliclk = type => {
  infoShow.value = true
  infoType.value = type
}

const dungeonOptions = [
  {
    label: '简单',
    value: 1
  },
  {
    label: '普通',
    value: 2
  },
  {
    label: '困难',
    value: 5
  },
  {
    label: '地狱',
    value: 10
  },
  {
    label: '通天',
    value: 100
  }
]

const handleUpdateValue = value => {
  if (value === 100) {
    message.warning('警告! 通天难度挑战失败后会跌落境界')
  }
}

const handleRefreshOptions = async () => {
  if (!serverDungeonRunning.value || isSubmitting.value || refreshNumber.value <= 0) return
  try {
    isSubmitting.value = true
    const result = await dungeonNextTurn({ refreshOptions: true })
    if (result?.snapshot) {
      playerStore.applyServerSnapshot(result.snapshot)
    }
    if (result?.state === 'option_required' || result?.needsOption) {
      dungeonState.value.showingOptions = true
      dungeonState.value.currentOptions = normalizeDungeonOptions(result?.options)
      refreshNumber.value = Number(result?.refreshCount ?? refreshNumber.value)
      if (result?.message) {
        pushDungeonLog(result.message)
      }
      return
    }
    await runServerTurns(result)
  } catch (error) {
    const code = error?.payload?.error
    if (code === 'dungeon refresh exhausted') {
      message.error('刷新次数不足')
    } else if (code === 'dungeon run not active') {
      serverDungeonRunning.value = false
      message.error('当前没有进行中的秘境探索')
    } else {
      message.error(error?.message || '刷新增益失败')
    }
  } finally {
    isSubmitting.value = false
  }
}
</script>

<style scoped>
  .control-row {
    width: 100%;
  }

  .option-cards {
    display: flex;
    gap: 16px;
    padding: 16px;
    margin: 0 auto;
  }

  .option-card {
    position: relative;
    padding: 20px;
    border: 2px solid;
    border-radius: 12px;
    background: var(--n-color);
    cursor: pointer;
    transition: all 0.3s ease;
    display: flex;
    flex-direction: column;
    min-height: 100px;
    width: 33%;
  }

  .option-card:hover {
    transform: translateX(5px);
    box-shadow: 4px 4px 12px rgba(0, 0, 0, 0.1);
  }

  .option-name {
    font-size: 1.3em;
    font-weight: bold;
    margin-bottom: 12px;
    padding-right: 80px;
  }

  .option-description {
    flex-grow: 1;
    font-size: 1em;
    color: var(--n-text-color);
    line-height: 1.6;
    margin-bottom: 8px;
  }

  .option-quality {
    position: absolute;
    top: 20px;
    right: 20px;
    font-size: 0.9em;
    font-weight: bold;
    padding: 4px 12px;
    border-radius: 20px;
    background: var(--n-color);
  }

  .combat-scene {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 20px;
    margin-bottom: 20px;
    min-height: 200px;
    background: rgba(0, 0, 0, 0.05);
    border-radius: 8px;
  }

  .character {
    display: flex;
    flex-direction: column;
    align-items: center;
    transition: transform 0.3s ease;
  }

  .character-avatar {
    font-size: 48px;
    margin: 10px 0;
  }

  .character-name {
    font-weight: bold;
    margin-bottom: 8px;
  }

  .health-bar {
    width: 100px;
    height: 10px;
    background: #ff000033;
    border-radius: 5px;
    overflow: hidden;
  }

  .health-fill {
    height: 100%;
    background: #ff0000;
    transition: width 0.3s ease;
  }

  .character.attack {
    animation: attack 0.5s ease;
  }

  .character.hurt {
    animation: hurt 0.5s ease;
  }

  .character-avatar {
    width: 60px;
    height: 60px;
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 24px;
    font-weight: bold;
    margin: 10px 0;
    color: #fff;
  }

  .player-avatar {
    background: linear-gradient(135deg, #4caf50, #2196f3);
    border-radius: 12px;
  }

  .enemy-avatar {
    background: linear-gradient(135deg, #ff5722, #e91e63);
    clip-path: polygon(50% 0%, 100% 38%, 100% 100%, 0 100%, 0% 38%);
  }

  .attack-effect {
    position: absolute;
    width: 20px;
    height: 20px;
    border-radius: 50%;
    pointer-events: none;
  }

  .player-effect {
    background: radial-gradient(circle, #4caf50, #2196f3);
    animation: player-attack-effect 0.5s ease-out;
    right: -10px;
  }

  .enemy-effect {
    background: radial-gradient(circle, #ff5722, #e91e63);
    animation: enemy-attack-effect 0.5s ease-out;
    left: -10px;
  }

  .enemy.attack {
    animation: enemy-attack 0.5s ease;
  }

  @keyframes player-attack-effect {
    0% {
      transform: scale(0.5) translateX(0);
      opacity: 1;
    }
    100% {
      transform: scale(1.5) translateX(200px);
      opacity: 0;
    }
  }

  @keyframes enemy-attack-effect {
    0% {
      transform: scale(0.5) translateX(0);
      opacity: 1;
    }
    100% {
      transform: scale(1.5) translateX(-200px);
      opacity: 0;
    }
  }

  @keyframes attack {
    0% {
      transform: translateX(0) rotate(0deg);
    }
    25% {
      transform: translateX(20px) rotate(5deg);
    }
    50% {
      transform: translateX(40px) rotate(0deg);
    }
    75% {
      transform: translateX(20px) rotate(-5deg);
    }
    100% {
      transform: translateX(0) rotate(0deg);
    }
  }

  @keyframes hurt {
    0% {
      transform: translateX(0);
    }
    25% {
      transform: translateX(-10px);
    }
    75% {
      transform: translateX(10px);
    }
    100% {
      transform: translateX(0);
    }
  }

  @keyframes enemy-attack {
    0% {
      transform: translateX(0) rotate(0deg);
    }
    25% {
      transform: translateX(-20px) rotate(-5deg);
    }
    50% {
      transform: translateX(-40px) rotate(0deg);
    }
    75% {
      transform: translateX(-20px) rotate(5deg);
    }
    100% {
      transform: translateX(0) rotate(0deg);
    }
  }

  @media (max-width: 960px) {
    .control-row {
      flex-wrap: wrap;
      gap: 10px;
    }

    .option-cards {
      flex-direction: column;
    }

    .option-card {
      width: 100%;
    }

    .combat-scene {
      flex-direction: column;
      gap: 14px;
    }
  }

  @media (max-width: 768px) {
    .control-row :deep(.n-base-selection),
    .control-row :deep(.n-button) {
      width: 100% !important;
    }

    .option-cards {
      padding: 0;
      gap: 10px;
    }

    .option-card {
      min-height: auto;
      padding: 14px;
    }

    .option-name {
      padding-right: 0;
      margin-bottom: 8px;
      font-size: 1.05em;
    }

    .option-quality {
      position: static;
      align-self: flex-start;
      margin-top: 4px;
    }

    .combat-scene {
      padding: 12px;
      min-height: auto;
    }

    .character {
      width: 100%;
    }

    .character-name {
      width: 100%;
    }

    .health-bar {
      width: 100%;
      max-width: 240px;
    }

    :deep(.n-modal .n-card) {
      width: calc(100vw - 20px);
      max-width: calc(100vw - 20px);
    }

    :deep(.n-descriptions) {
      --n-td-padding: 8px;
    }
  }
</style>
