import { httpRequest } from '../http'

export async function cultivateOnce() {
  return httpRequest('/game/cultivation/once', {
    method: 'POST'
  })
}

export async function cultivateUntilBreakthrough() {
  return httpRequest('/game/cultivation/until-breakthrough', {
    method: 'POST'
  })
}

export async function listHuntingMaps() {
  return httpRequest('/game/hunting/maps')
}

export async function getHuntingStatus() {
  return httpRequest('/game/hunting/status')
}

export async function startHuntingRun(mapId) {
  return httpRequest('/game/hunting/start', {
    method: 'POST',
    body: { mapId }
  })
}

export async function tickHuntingRun() {
  return httpRequest('/game/hunting/tick', {
    method: 'POST'
  })
}

export async function stopHuntingRun() {
  return httpRequest('/game/hunting/stop', {
    method: 'POST'
  })
}

export async function huntMonster(mapId) {
  return httpRequest('/game/hunting/fight', {
    method: 'POST',
    body: { mapId }
  })
}

export async function breakthrough() {
  return httpRequest('/game/breakthrough', {
    method: 'POST'
  })
}

export async function startExploration(locationId) {
  return httpRequest('/game/exploration/start', {
    method: 'POST',
    body: { locationId }
  })
}

export async function craftAlchemyPill(recipeId) {
  return httpRequest('/game/alchemy/craft', {
    method: 'POST',
    body: { recipeId }
  })
}

export async function drawGacha(payload) {
  return httpRequest('/game/gacha/draw', {
    method: 'POST',
    body: payload
  })
}

export async function inventorySellEquipment(itemId) {
  return httpRequest('/game/inventory/equipment/sell', {
    method: 'POST',
    body: { itemId }
  })
}

export async function inventoryBatchSellEquipment(payload) {
  return httpRequest('/game/inventory/equipment/sell-batch', {
    method: 'POST',
    body: payload
  })
}

export async function inventoryReleasePet(itemId) {
  return httpRequest('/game/inventory/pet/release', {
    method: 'POST',
    body: { itemId }
  })
}

export async function inventoryBatchReleasePets(rarity) {
  return httpRequest('/game/inventory/pet/release-batch', {
    method: 'POST',
    body: { rarity }
  })
}

export async function inventoryUpgradePet(itemId) {
  return httpRequest('/game/inventory/pet/upgrade', {
    method: 'POST',
    body: { itemId }
  })
}

export async function inventoryEvolvePet(itemId, foodItemId) {
  return httpRequest('/game/inventory/pet/evolve', {
    method: 'POST',
    body: { itemId, foodItemId }
  })
}

export async function gameUseItem(itemId) {
  return httpRequest('/game/item/use', {
    method: 'POST',
    body: { itemId }
  })
}

export async function dungeonStart(difficulty) {
  return httpRequest('/game/dungeon/start', {
    method: 'POST',
    body: { difficulty }
  })
}

export async function dungeonNextTurn(payload = null) {
  return httpRequest('/game/dungeon/next-turn', {
    method: 'POST',
    body: payload
  })
}

export async function inventoryEquipEquipment(itemId) {
  return httpRequest('/game/inventory/equipment/equip', {
    method: 'POST',
    body: { itemId }
  })
}

export async function inventoryUnequipEquipment(slot) {
  return httpRequest('/game/inventory/equipment/unequip', {
    method: 'POST',
    body: { slot }
  })
}

export async function inventoryEnhanceEquipment(itemId) {
  return httpRequest('/game/inventory/equipment/enhance', {
    method: 'POST',
    body: { itemId }
  })
}

export async function inventoryReforgeEquipment(itemId) {
  return httpRequest('/game/inventory/equipment/reforge', {
    method: 'POST',
    body: { itemId }
  })
}
