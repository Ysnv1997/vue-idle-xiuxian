import { defineStore } from 'pinia'
import { fetchPlayerSnapshot } from '../api/modules/player'

const defaultBaseAttributes = {
  attack: 10,
  health: 100,
  defense: 5,
  speed: 10
}

const defaultCombatAttributes = {
  critRate: 0,
  comboRate: 0,
  counterRate: 0,
  stunRate: 0,
  dodgeRate: 0,
  vampireRate: 0
}

const defaultCombatResistance = {
  critResist: 0,
  comboResist: 0,
  counterResist: 0,
  stunResist: 0,
  dodgeResist: 0,
  vampireResist: 0
}

const defaultSpecialAttributes = {
  healBoost: 0,
  critDamageBoost: 0,
  critDamageReduce: 0,
  finalDamageBoost: 0,
  finalDamageReduce: 0,
  combatBoost: 0,
  resistanceBoost: 0
}

const defaultEquippedArtifacts = {
  weapon: null,
  head: null,
  body: null,
  legs: null,
  feet: null,
  shoulder: null,
  hands: null,
  wrist: null,
  necklace: null,
  ring1: null,
  ring2: null,
  belt: null,
  artifact: null
}

const defaultArtifactBonuses = {
  attack: 0,
  health: 0,
  defense: 0,
  speed: 0,
  critRate: 0,
  comboRate: 0,
  counterRate: 0,
  stunRate: 0,
  dodgeRate: 0,
  vampireRate: 0,
  critResist: 0,
  comboResist: 0,
  counterResist: 0,
  stunResist: 0,
  dodgeResist: 0,
  vampireResist: 0,
  healBoost: 0,
  critDamageBoost: 0,
  critDamageReduce: 0,
  finalDamageBoost: 0,
  finalDamageReduce: 0,
  combatBoost: 0,
  resistanceBoost: 0,
  cultivationRate: 1,
  spiritRate: 1
}

export const usePlayerStore = defineStore('player', {
  state: () => ({
    isNewPlayer: true,
    isGMMode: false,
    isDarkMode: localStorage.getItem('darkMode') === 'true',

    activePet: null,
    petEssence: 0,
    petConfig: {
      rarityMap: {
        divine: { name: '神品', color: '#FF0000', probability: 0.02, essenceBonus: 50 },
        celestial: { name: '仙品', color: '#FFD700', probability: 0.08, essenceBonus: 30 },
        mystic: { name: '玄品', color: '#9932CC', probability: 0.15, essenceBonus: 20 },
        spiritual: { name: '灵品', color: '#1E90FF', probability: 0.25, essenceBonus: 10 },
        mortal: { name: '凡品', color: '#32CD32', probability: 0.5, essenceBonus: 5 }
      }
    },

    name: '无名修士',
    nameChangeCount: 0,
    level: 1,
    realm: '练气期一层',
    cultivation: 0,
    maxCultivation: 100,
    spirit: 0,
    spiritRate: 1,
    luck: 1,
    cultivationRate: 1,
    herbRate: 1,
    alchemyRate: 1,

    pills: [],
    pillFragments: {},
    pillRecipes: [],
    activeEffects: [],
    pillsCrafted: 0,
    pillsConsumed: 0,

    baseAttributes: { ...defaultBaseAttributes },
    combatAttributes: { ...defaultCombatAttributes },
    combatResistance: { ...defaultCombatResistance },
    specialAttributes: { ...defaultSpecialAttributes },

    spiritStones: 0,
    reinforceStones: 0,
    refinementStones: 0,
    herbs: [],
    items: [],
    artifacts: [],

    equippedArtifacts: { ...defaultEquippedArtifacts },
    artifactBonuses: { ...defaultArtifactBonuses },

    totalCultivationTime: 0,
    breakthroughCount: 0,
    explorationCount: 0,
    itemsFound: 0,
    eventTriggered: 0,
    unlockedPillRecipes: 0,

    dungeonDifficulty: 1,
    dungeonHighestFloor: 0,
    dungeonHighestFloor_2: 0,
    dungeonHighestFloor_5: 0,
    dungeonHighestFloor_10: 0,
    dungeonHighestFloor_100: 0,
    dungeonLastFailedFloor: 0,
    dungeonTotalRuns: 0,
    dungeonBossKills: 0,
    dungeonEliteKills: 0,
    dungeonTotalKills: 0,
    dungeonDeathCount: 0,
    dungeonTotalRewards: 0,

    autoSellQualities: [],
    autoReleaseRarities: [],

    wishlistEnabled: false,
    selectedWishEquipQuality: null,
    selectedWishPetRarity: null,

    unlockedRealms: ['练气一层'],
    unlockedLocations: ['新手村'],
    unlockedSkills: [],
    completedAchievements: []
  }),
  actions: {
    updateHtmlDarkMode(isDarkMode) {
      const htmlEl = document.documentElement
      if (isDarkMode) {
        htmlEl.classList.add('dark')
      } else {
        htmlEl.classList.remove('dark')
      }
    },

    applyServerSnapshot(snapshot) {
      if (!snapshot) return

      const snapshotActivePetId =
        typeof snapshot.activePetId === 'string' || typeof snapshot.activePetId === 'number' ? snapshot.activePetId : null

      Object.assign(this.$state, {
        name: snapshot.name ?? this.name,
        level: snapshot.level ?? this.level,
        realm: snapshot.realm ?? this.realm,
        cultivation: snapshot.cultivation ?? this.cultivation,
        maxCultivation: snapshot.maxCultivation ?? this.maxCultivation,
        spirit: snapshot.spirit ?? this.spirit,
        spiritRate: snapshot.spiritRate ?? this.spiritRate,
        luck: snapshot.luck ?? this.luck,
        cultivationRate: snapshot.cultivationRate ?? this.cultivationRate,

        spiritStones: snapshot.spiritStones ?? this.spiritStones,
        reinforceStones: snapshot.reinforceStones ?? this.reinforceStones,
        refinementStones: snapshot.refinementStones ?? this.refinementStones,
        petEssence: snapshot.petEssence ?? this.petEssence,

        explorationCount: snapshot.explorationCount ?? this.explorationCount,
        eventTriggered: snapshot.eventTriggered ?? this.eventTriggered,

        dungeonHighestFloor: snapshot.dungeonHighestFloor ?? this.dungeonHighestFloor,
        dungeonHighestFloor_2: snapshot.dungeonHighestFloor_2 ?? this.dungeonHighestFloor_2,
        dungeonHighestFloor_5: snapshot.dungeonHighestFloor_5 ?? this.dungeonHighestFloor_5,
        dungeonHighestFloor_10: snapshot.dungeonHighestFloor_10 ?? this.dungeonHighestFloor_10,
        dungeonHighestFloor_100: snapshot.dungeonHighestFloor_100 ?? this.dungeonHighestFloor_100,
        dungeonLastFailedFloor: snapshot.dungeonLastFailedFloor ?? this.dungeonLastFailedFloor,
        dungeonTotalRuns: snapshot.dungeonTotalRuns ?? this.dungeonTotalRuns,
        dungeonBossKills: snapshot.dungeonBossKills ?? this.dungeonBossKills,
        dungeonEliteKills: snapshot.dungeonEliteKills ?? this.dungeonEliteKills,
        dungeonTotalKills: snapshot.dungeonTotalKills ?? this.dungeonTotalKills,
        dungeonDeathCount: snapshot.dungeonDeathCount ?? this.dungeonDeathCount,
        dungeonTotalRewards: snapshot.dungeonTotalRewards ?? this.dungeonTotalRewards,

        baseAttributes: snapshot.baseAttributes ?? this.baseAttributes,
        combatAttributes: snapshot.combatAttributes ?? this.combatAttributes,
        combatResistance: snapshot.combatResistance ?? this.combatResistance,
        specialAttributes: snapshot.specialAttributes ?? this.specialAttributes,

        herbs: snapshot.herbs ?? this.herbs,
        pillFragments: snapshot.pillFragments ?? this.pillFragments,
        pillRecipes: snapshot.pillRecipes ?? this.pillRecipes,
        activeEffects: snapshot.activeEffects ?? this.activeEffects,
        items: snapshot.items ?? this.items,
        equippedArtifacts: snapshot.equippedArtifacts ?? this.equippedArtifacts,

        unlockedPillRecipes: Array.isArray(snapshot.pillRecipes) ? snapshot.pillRecipes.length : this.unlockedPillRecipes,
        isNewPlayer: false
      })

      if (snapshotActivePetId !== null && String(snapshotActivePetId) !== '') {
        const currentActivePet = this.items.find(item => String(item.id) === String(snapshotActivePetId))
        this.activePet = currentActivePet || null
      } else if (snapshot.activePetId !== undefined) {
        this.activePet = null
      } else if (this.activePet) {
        const currentActivePet = this.items.find(item => String(item.id) === String(this.activePet.id))
        this.activePet = currentActivePet || null
      }
    },

    async initializePlayer() {
      try {
        const snapshot = await fetchPlayerSnapshot()
        this.applyServerSnapshot(snapshot)
      } catch (error) {
        if (error?.status === 401) {
          console.warn('未登录，等待鉴权完成后拉取快照')
        } else {
          console.error('加载服务器快照失败:', error)
        }
      }

      this.isDarkMode = localStorage.getItem('darkMode') === 'true'
      this.updateHtmlDarkMode(this.isDarkMode)
    },

    async refreshSnapshot() {
      try {
        const snapshot = await fetchPlayerSnapshot()
        this.applyServerSnapshot(snapshot)
      } catch (error) {
        if (error?.status !== 401) {
          console.error('刷新玩家快照失败:', error)
        }
      }
    },

    toggle() {
      this.isDarkMode = !this.isDarkMode
      localStorage.setItem('darkMode', this.isDarkMode)
      this.updateHtmlDarkMode(this.isDarkMode)
    }
  }
})
