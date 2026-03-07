<template>
  <n-modal :show="show" preset="card" class="player-profile-modal" style="width: min(880px, calc(100vw - 24px))" title="修士资料" @update:show="emit('update:show', $event)">
    <n-spin :show="loading">
      <n-empty v-if="!profile && !loading" description="暂无资料" />
      <n-space v-else vertical>
        <n-descriptions bordered :column="3" size="small">
          <n-descriptions-item label="道号">{{ profile?.name || '--' }}</n-descriptions-item>
          <n-descriptions-item label="等级">{{ profile?.level || 0 }}</n-descriptions-item>
          <n-descriptions-item label="境界">{{ profile?.realm || '--' }}</n-descriptions-item>
        </n-descriptions>

        <n-card size="small" title="基础属性">
          <n-descriptions bordered :column="2" size="small">
            <n-descriptions-item label="攻击">{{ formatStat(profile?.baseAttributes?.attack) }}</n-descriptions-item>
            <n-descriptions-item label="生命">{{ formatStat(profile?.baseAttributes?.health) }}</n-descriptions-item>
            <n-descriptions-item label="防御">{{ formatStat(profile?.baseAttributes?.defense) }}</n-descriptions-item>
            <n-descriptions-item label="速度">{{ formatStat(profile?.baseAttributes?.speed) }}</n-descriptions-item>
          </n-descriptions>
        </n-card>

        <n-card size="small" title="战斗属性">
          <n-descriptions bordered :column="2" size="small">
            <n-descriptions-item label="暴击率">{{ formatPercent(profile?.combatAttributes?.critRate) }}</n-descriptions-item>
            <n-descriptions-item label="连击率">{{ formatPercent(profile?.combatAttributes?.comboRate) }}</n-descriptions-item>
            <n-descriptions-item label="反击率">{{ formatPercent(profile?.combatAttributes?.counterRate) }}</n-descriptions-item>
            <n-descriptions-item label="闪避率">{{ formatPercent(profile?.combatAttributes?.dodgeRate) }}</n-descriptions-item>
            <n-descriptions-item label="吸血率">{{ formatPercent(profile?.combatAttributes?.vampireRate) }}</n-descriptions-item>
            <n-descriptions-item label="暴击增伤">{{ formatPercent(profile?.combatAttributes?.critDamageBoost) }}</n-descriptions-item>
          </n-descriptions>
        </n-card>

        <n-card size="small" title="战斗抗性">
          <n-descriptions bordered :column="2" size="small">
            <n-descriptions-item label="暴击抗性">{{ formatPercent(profile?.combatResistance?.critResist) }}</n-descriptions-item>
            <n-descriptions-item label="连击抗性">{{ formatPercent(profile?.combatResistance?.comboResist) }}</n-descriptions-item>
            <n-descriptions-item label="反击抗性">{{ formatPercent(profile?.combatResistance?.counterResist) }}</n-descriptions-item>
            <n-descriptions-item label="眩晕抗性">{{ formatPercent(profile?.combatResistance?.stunResist) }}</n-descriptions-item>
            <n-descriptions-item label="闪避抗性">{{ formatPercent(profile?.combatResistance?.dodgeResist) }}</n-descriptions-item>
            <n-descriptions-item label="吸血抗性">{{ formatPercent(profile?.combatResistance?.vampireResist) }}</n-descriptions-item>
          </n-descriptions>
        </n-card>

        <n-card size="small" title="装备一览">
          <div class="equipment-grid">
            <div v-for="slot in equipmentSlots" :key="slot.key" class="equipment-item">
              <strong>{{ slot.label }}</strong>
              <template v-if="equippedMap[slot.key]">
                <span>{{ equippedMap[slot.key].name || '未知装备' }}</span>
                <small>{{ equippedMap[slot.key].qualityInfo?.name || equippedMap[slot.key].quality || '未知品质' }}</small>
              </template>
              <span v-else class="empty-text">未装备</span>
            </div>
          </div>
        </n-card>

        <n-card size="small" title="出战灵宠">
          <n-empty v-if="!profile?.activePet?.id" description="暂无出战灵宠" />
          <n-descriptions v-else bordered :column="2" size="small">
            <n-descriptions-item label="名称">{{ profile.activePet.name }}</n-descriptions-item>
            <n-descriptions-item label="品阶">{{ profile.activePet.rarityInfo?.name || profile.activePet.rarity || '--' }}</n-descriptions-item>
            <n-descriptions-item label="攻击">{{ formatStat(profile.activePet.combatAttributes?.attack) }}</n-descriptions-item>
            <n-descriptions-item label="生命">{{ formatStat(profile.activePet.combatAttributes?.health) }}</n-descriptions-item>
            <n-descriptions-item label="防御">{{ formatStat(profile.activePet.combatAttributes?.defense) }}</n-descriptions-item>
            <n-descriptions-item label="速度">{{ formatStat(profile.activePet.combatAttributes?.speed) }}</n-descriptions-item>
          </n-descriptions>
        </n-card>
      </n-space>
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
    { key: 'weapon', label: '武器' },
    { key: 'head', label: '头部' },
    { key: 'body', label: '衣服' },
    { key: 'legs', label: '裤子' },
    { key: 'feet', label: '鞋子' },
    { key: 'shoulder', label: '肩甲' },
    { key: 'hands', label: '手套' },
    { key: 'wrist', label: '护腕' },
    { key: 'necklace', label: '项链' },
    { key: 'ring1', label: '戒指一' },
    { key: 'ring2', label: '戒指二' },
    { key: 'belt', label: '腰带' },
    { key: 'artifact', label: '法宝' }
  ]

  const equippedMap = computed(() => (props.profile?.equippedArtifacts && typeof props.profile.equippedArtifacts === 'object' ? props.profile.equippedArtifacts : {}))

  const formatStat = value => Number(value || 0).toLocaleString('zh-CN', { maximumFractionDigits: 2 })
  const formatPercent = value => `${(Number(value || 0) * 100).toFixed(2)}%`
</script>

<style scoped>
  .equipment-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(160px, 1fr));
    gap: 10px;
  }

  .equipment-item {
    display: flex;
    flex-direction: column;
    gap: 4px;
    padding: 10px;
    border: 1px solid rgba(127, 127, 127, 0.18);
    border-radius: 10px;
  }

  .equipment-item small,
  .empty-text {
    color: rgba(127, 127, 127, 0.9);
  }

  @media (max-width: 768px) {
    .equipment-grid {
      grid-template-columns: 1fr;
    }

    :deep(.n-descriptions) {
      --n-td-padding: 8px;
    }

    .player-profile-modal {
      width: calc(100vw - 16px) !important;
    }
  }
</style>
