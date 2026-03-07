<template>
  <n-modal 
    :show="show" 
    preset="card" 
    class="player-profile-modal" 
    style="width: min(800px, calc(100vw - 20px))" 
    @update:show="emit('update:show', $event)"
  >
    <template #header>
      <div class="modal-header-profile" v-if="profile">
        <div class="profile-avatar">
          <img v-if="profile.avatar" :src="profile.avatar" alt="Avatar" />
          <div v-else class="avatar-placeholder">{{ profile.name[0] }}</div>
        </div>
        <div class="profile-main">
          <h3 class="profile-name">{{ profile.name }}</h3>
          <p class="profile-realm">{{ profile.realm }} <n-tag size="tiny" type="info" round>Lv.{{ profile.level }}</n-tag></p>
        </div>
      </div>
      <div v-else>修士详情</div>
    </template>

    <n-spin :show="loading">
      <n-empty v-if="!profile && !loading" description="天机难测，资料未寻得" />
      <div class="profile-dialog-content" v-else-if="profile">
        <n-tabs type="segment" animated>
          <!-- 属性概览 -->
          <n-tab-pane name="stats" tab="神通属性">
            <div class="stats-sections">
              <div class="stats-group">
                <div class="group-title">基础六维</div>
                <div class="stats-grid">
                  <div class="stat-item"><span class="label">攻击</span><span class="value">{{ formatStat(profile.baseAttributes?.attack) }}</span></div>
                  <div class="stat-item"><span class="label">生命</span><span class="value">{{ formatStat(profile.baseAttributes?.health) }}</span></div>
                  <div class="stat-item"><span class="label">防御</span><span class="value">{{ formatStat(profile.baseAttributes?.defense) }}</span></div>
                  <div class="stat-item"><span class="label">速度</span><span class="value">{{ formatStat(profile.baseAttributes?.speed) }}</span></div>
                </div>
              </div>

              <div class="stats-group">
                <div class="group-title">战斗斗法</div>
                <div class="stats-grid compact">
                  <div class="stat-item"><span>暴击</span><strong>{{ formatPercent(profile.combatAttributes?.critRate) }}</strong></div>
                  <div class="stat-item"><span>连击</span><strong>{{ formatPercent(profile.combatAttributes?.comboRate) }}</strong></div>
                  <div class="stat-item"><span>反击</span><strong>{{ formatPercent(profile.combatAttributes?.counterRate) }}</strong></div>
                  <div class="stat-item"><span>闪避</span><strong>{{ formatPercent(profile.combatAttributes?.dodgeRate) }}</strong></div>
                  <div class="stat-item"><span>吸血</span><strong>{{ formatPercent(profile.combatAttributes?.vampireRate) }}</strong></div>
                  <div class="stat-item"><span>暴伤</span><strong>{{ formatPercent(profile.combatAttributes?.critDamageBoost) }}</strong></div>
                </div>
              </div>
            </div>
          </n-tab-pane>

          <!-- 装备穿戴 -->
          <n-tab-pane name="equip" tab="法宝神兵">
            <div class="equipment-display-grid">
              <div v-for="slot in equipmentSlots" :key="slot.key" class="slot-mini-card">
                <div class="slot-label">{{ slot.label }}</div>
                <div class="slot-item-info" v-if="equippedMap[slot.key]">
                  <div class="item-name" :style="{ color: getItemColor(equippedMap[slot.key]) }">
                    {{ equippedMap[slot.key].name }}
                  </div>
                  <div class="item-extra">
                    <span v-if="equippedMap[slot.key].enhanceLevel > 0">+{{ equippedMap[slot.key].enhanceLevel }}</span>
                    <span>{{ getItemQualityName(equippedMap[slot.key]) }}</span>
                  </div>
                </div>
                <div v-else class="slot-empty">虚位以待</div>
              </div>
            </div>
          </n-tab-pane>

          <!-- 灵宠 -->
          <n-tab-pane name="pet" tab="镇山灵宠">
            <div class="pet-detail-card" v-if="profile.activePet?.id">
              <div class="pet-header">
                <div class="pet-name">{{ profile.activePet.name }}</div>
                <n-tag size="small" type="warning" round>{{ profile.activePet.rarityInfo?.name || '灵宠' }}</n-tag>
              </div>
              <div class="pet-body">
                <div class="pet-stat"><span>等阶</span><strong>{{ profile.activePet.level }} 级</strong></div>
                <div class="pet-stat"><span>星级</span><strong>{{ profile.activePet.star || 0 }} 星</strong></div>
                <div class="pet-stat"><span>加成</span><strong>+{{ ((profile.activePet.bonusRate || 0) * 100).toFixed(1) }}%</strong></div>
              </div>
            </div>
            <n-empty v-else description="未见灵宠相随" style="padding: 40px 0" />
          </n-tab-pane>
        </n-tabs>
      </div>
    </n-spin>
  </n-modal>
</template>

<script setup>
  import { computed } from 'vue'

  const props = defineProps({
    show: { type: Boolean, default: false },
    loading: { type: Boolean, default: false },
    profile: { type: Object, default: null }
  })

  const emit = defineEmits(['update:show'])

  const equipmentSlots = [
    { key: 'weapon', label: '武器' }, { key: 'head', label: '头部' }, { key: 'body', label: '衣服' },
    { key: 'legs', label: '裤子' }, { key: 'feet', label: '鞋子' }, { key: 'shoulder', label: '肩甲' },
    { key: 'hands', label: '手套' }, { key: 'wrist', label: '护腕' }, { key: 'necklace', label: '项链' },
    { key: 'ring1', label: '戒一' }, { key: 'ring2', label: '戒二' }, { key: 'belt', label: '腰带' },
    { key: 'artifact', label: '法宝' }
  ]

  const equippedMap = computed(() => props.profile?.equippedArtifacts || {})

  const formatStat = v => Number(v || 0).toLocaleString('zh-CN', { maximumFractionDigits: 1 })
  const formatPercent = v => `${(Number(v || 0) * 100).toFixed(1)}%`

  const getItemColor = item => {
    const q = item?.quality || 'common'
    const colors = { common: '#94a3b8', uncommon: '#18a058', rare: '#2080f0', epic: '#a042ff', legendary: '#f0a020', mythic: '#d03050' }
    return colors[q] || colors.common
  }

  const getItemQualityName = item => {
    const q = item?.quality || 'common'
    const names = { common: '凡', uncommon: '下', rare: '中', epic: '上', legendary: '极', mythic: '仙' }
    return names[q] || '凡'
  }
</script>

<style scoped>
.modal-header-profile { display: flex; align-items: center; gap: 16px; padding: 10px 0; }
.profile-avatar { width: 56px; height: 56px; border-radius: 12px; overflow: hidden; border: 2px solid var(--panel-border); }
.profile-avatar img { width: 100%; height: 100%; object-fit: cover; }
.avatar-placeholder { width: 100%; height: 100%; background: var(--accent-primary); color: white; display: grid; place-items: center; font-family: var(--font-display); font-size: 28px; }

.profile-name { margin: 0; font-family: var(--font-display); font-size: 22px; line-height: 1.2; }
.profile-realm { font-size: 13px; color: var(--ink-sub); margin-top: 4px; display: flex; align-items: center; gap: 8px; }

.profile-dialog-content { padding: 10px 0; }

.stats-sections { display: flex; flex-direction: column; gap: 20px; }
.stats-group { background: rgba(0,0,0,0.02); border-radius: 16px; padding: 16px; }
.group-title { font-size: 12px; font-weight: bold; color: var(--accent-primary); margin-bottom: 12px; text-transform: uppercase; letter-spacing: 1px; }

.stats-grid { display: grid; grid-template-columns: repeat(2, 1fr); gap: 12px; }
.stats-grid.compact { grid-template-columns: repeat(3, 1fr); }

.stat-item { display: flex; justify-content: space-between; align-items: baseline; font-size: 13px; }
.stat-item .label { color: var(--ink-sub); }
.stat-item .value { font-weight: bold; font-variant-numeric: tabular-nums; }

/* 装备迷你卡片 */
.equipment-display-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
  gap: 10px;
}

.slot-mini-card {
  padding: 12px;
  background: var(--panel-bg);
  border: 1px solid var(--panel-border);
  border-radius: 12px;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.slot-label { font-size: 10px; opacity: 0.5; font-weight: bold; }
.item-name { font-size: 14px; font-weight: bold; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.item-extra { font-size: 10px; opacity: 0.6; display: flex; gap: 8px; }
.slot-empty { font-size: 12px; opacity: 0.3; font-style: italic; }

/* 灵宠卡片 */
.pet-detail-card {
  background: var(--panel-bg);
  border: 1px solid var(--panel-border);
  border-radius: 20px;
  padding: 24px;
  display: flex;
  flex-direction: column;
  gap: 16px;
  max-width: 400px;
  margin: 0 auto;
}
.pet-header { display: flex; justify-content: space-between; align-items: center; border-bottom: 1px dashed var(--panel-border); padding-bottom: 12px; }
.pet-name { font-size: 18px; font-weight: bold; font-family: var(--font-display); }
.pet-body { display: grid; grid-template-columns: repeat(3, 1fr); gap: 12px; }
.pet-stat { text-align: center; }
.pet-stat span { font-size: 11px; color: var(--ink-sub); display: block; }
.pet-stat strong { font-size: 14px; color: var(--accent-primary); }

@media (max-width: 768px) {
  .stats-grid.compact { grid-template-columns: repeat(2, 1fr); }
  .equipment-display-grid { grid-template-columns: 1fr; }
}
</style>
