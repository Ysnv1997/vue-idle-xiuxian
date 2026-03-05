import { httpRequest } from '../http'

export async function fetchAchievements() {
  return httpRequest('/game/achievements')
}

export async function syncAchievements() {
  return httpRequest('/game/achievements/sync', {
    method: 'POST'
  })
}
