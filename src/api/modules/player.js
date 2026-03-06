import { httpRequest } from '../http'

export async function fetchPlayerSnapshot() {
  return httpRequest('/player/snapshot')
}

export async function fetchActivePlayerCount() {
  return httpRequest('/player/active-count')
}
