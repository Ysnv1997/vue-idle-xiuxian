<template>
  <n-drawer :show="show" :width="400" :placement="placement" @update:show="$emit('update:show', $event)">
    <n-drawer-content closable>
      <template #header>
        <div class="drawer-header">
          <div class="header-avatar">
            <img v-if="linuxDoAvatar" :src="linuxDoAvatar" alt="Avatar" class="avatar-img" />
            <div v-else class="avatar-placeholder">{{ playerInitial }}</div>
          </div>
          <div class="header-info">
            <h3>{{ playerStore.name }}</h3>
            <p>{{ currentRealmName }}</p>
          </div>
        </div>
      </template>

      <div class="profile-content">
        <!-- 核心属性概览 -->
        <div class="attribute-grid">
          <div class="attr-item">
            <span class="label">攻击力</span>
            <strong class="value">{{ formatInt(playerStore.baseAttributes.attack) }}</strong>
          </div>
          <div class="attr-item">
            <span class="label">生命值</span>
            <strong class="value">{{ formatInt(playerStore.baseAttributes.health) }}</strong>
          </div>
          <div class="attr-item">
            <span class="label">防御力</span>
            <strong class="value">{{ formatInt(playerStore.baseAttributes.defense) }}</strong>
          </div>
          <div class="attr-item">
            <span class="label">速度</span>
            <strong class="value">{{ formatInt(playerStore.baseAttributes.speed) }}</strong>
          </div>
        </div>

        <n-collapse arrow-placement="right" class="detail-collapse" :default-expanded-names="['base']">
          <n-collapse-item title="基础属性" name="base">
            <n-descriptions :column="2" bordered size="small">
              <n-descriptions-item label="攻击力">{{ formatInt(playerStore.baseAttributes.attack) }}</n-descriptions-item>
              <n-descriptions-item label="生命值">{{ formatInt(playerStore.baseAttributes.health) }}</n-descriptions-item>
              <n-descriptions-item label="防御力">{{ formatInt(playerStore.baseAttributes.defense) }}</n-descriptions-item>
              <n-descriptions-item label="速度">{{ formatInt(playerStore.baseAttributes.speed) }}</n-descriptions-item>
            </n-descriptions>
          </n-collapse-item>

          <n-collapse-item title="战斗属性" name="combat">
            <n-descriptions :column="2" bordered size="small">
              <n-descriptions-item label="暴击率">{{ formatPercent(playerStore.combatAttributes.critRate) }}</n-descriptions-item>
              <n-descriptions-item label="眩晕率">{{ formatPercent(playerStore.combatAttributes.stunRate) }}</n-descriptions-item>
              <n-descriptions-item label="连击率">{{ formatPercent(playerStore.combatAttributes.comboRate) }}</n-descriptions-item>
              <n-descriptions-item label="闪避率">{{ formatPercent(playerStore.combatAttributes.dodgeRate) }}</n-descriptions-item>
              <n-descriptions-item label="反击率">{{ formatPercent(playerStore.combatAttributes.counterRate) }}</n-descriptions-item>
              <n-descriptions-item label="吸血率">{{ formatPercent(playerStore.combatAttributes.vampireRate) }}</n-descriptions-item>
            </n-descriptions>
          </n-collapse-item>

          <n-collapse-item title="战斗抗性" name="resistance">
            <n-descriptions :column="2" bordered size="small">
              <n-descriptions-item label="暴击抗性">{{ formatPercent(playerStore.combatResistance.critResist) }}</n-descriptions-item>
              <n-descriptions-item label="眩晕抗性">{{ formatPercent(playerStore.combatResistance.stunResist) }}</n-descriptions-item>
              <n-descriptions-item label="连击抗性">{{ formatPercent(playerStore.combatResistance.comboResist) }}</n-descriptions-item>
              <n-descriptions-item label="闪避抗性">{{ formatPercent(playerStore.combatResistance.dodgeResist) }}</n-descriptions-item>
              <n-descriptions-item label="反击抗性">{{ formatPercent(playerStore.combatResistance.counterResist) }}</n-descriptions-item>
              <n-descriptions-item label="吸血抗性">{{ formatPercent(playerStore.combatResistance.vampireResist) }}</n-descriptions-item>
            </n-descriptions>
          </n-collapse-item>

          <n-collapse-item title="特殊属性" name="special">
            <n-descriptions :column="2" bordered size="small">
              <n-descriptions-item label="治疗提升">{{ formatPercent(playerStore.specialAttributes.healBoost) }}</n-descriptions-item>
              <n-descriptions-item label="战斗提升">{{ formatPercent(playerStore.specialAttributes.combatBoost) }}</n-descriptions-item>
              <n-descriptions-item label="暴伤提升">{{ formatPercent(playerStore.specialAttributes.critDamageBoost) }}</n-descriptions-item>
              <n-descriptions-item label="抗性提升">{{ formatPercent(playerStore.specialAttributes.resistanceBoost) }}</n-descriptions-item>
              <n-descriptions-item label="暴伤减免">{{ formatPercent(playerStore.specialAttributes.critDamageReduce) }}</n-descriptions-item>
              <n-descriptions-item label="最终增伤">{{ formatPercent(playerStore.specialAttributes.finalDamageBoost) }}</n-descriptions-item>
              <n-descriptions-item label="最终减伤">{{ formatPercent(playerStore.specialAttributes.finalDamageReduce) }}</n-descriptions-item>
            </n-descriptions>
          </n-collapse-item>
        </n-collapse>

        <div class="profile-footer" v-if="linuxDoUserId">
          <p class="linuxdo-id">LinuxDo ID: {{ linuxDoUserId }}</p>
        </div>
      </div>
    </n-drawer-content>
  </n-drawer>
</template>

<script setup>
import { computed } from 'vue'
import { usePlayerStore } from '../stores/player'
import { useSessionStore } from '../stores/session'
import { getRealmName } from '../plugins/realm'

const props = defineProps({
  show: Boolean,
  placement: {
    type: String,
    default: 'right'
  }
})

defineEmits(['update:show'])

const playerStore = usePlayerStore()
const sessionStore = useSessionStore()

const currentRealmName = computed(() => getRealmName(playerStore.level).name)
const linuxDoAvatar = computed(() => String(sessionStore.user?.avatar || '').trim())
const linuxDoUserId = computed(() => String(sessionStore.user?.linuxDoUserId || '').trim())
const playerInitial = computed(() => String(playerStore.name || '').trim().slice(0, 1) || '修')

const formatInt = value => Math.floor(Number(value || 0)).toLocaleString()
const formatPercent = value => `${(Number(value || 0) * 100).toFixed(1)}%`
</script>

<style scoped>
.drawer-header {
  display: flex;
  align-items: center;
  gap: 16px;
}

.header-avatar {
  width: 56px;
  height: 56px;
  flex-shrink: 0;
}

.avatar-img {
  width: 100%;
  height: 100%;
  border-radius: 14px;
  object-fit: cover;
  border: 2px solid var(--panel-border);
}

.avatar-placeholder {
  width: 100%;
  height: 100%;
  border-radius: 14px;
  display: grid;
  place-items: center;
  font-family: var(--font-display);
  font-size: 28px;
  color: #fff;
  background: linear-gradient(145deg, #c6853e, #9b5d26);
}

.header-info h3 {
  font-family: var(--font-display);
  font-size: 22px;
  margin: 0;
  color: var(--ink-main);
}

.header-info p {
  font-size: 14px;
  color: var(--ink-sub);
  margin-top: 4px;
}

.profile-content {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.attribute-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 12px;
  padding: 16px;
  background: var(--accent-muted);
  border-radius: 16px;
}

.attr-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.attr-item .label {
  font-size: 12px;
  color: var(--ink-sub);
}

.attr-item .value {
  font-size: 18px;
  color: var(--accent-primary);
  font-family: var(--font-display);
}

.detail-collapse {
  background: transparent;
}

.profile-footer {
  margin-top: auto;
  padding: 16px 0;
  border-top: 1px dashed var(--panel-border);
  text-align: center;
}

.linuxdo-id {
  font-size: 12px;
  color: var(--ink-sub);
  opacity: 0.7;
}
</style>
