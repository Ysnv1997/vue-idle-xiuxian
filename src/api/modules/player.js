import { httpRequest } from '../http'

export async function fetchPlayerSnapshot() {
  return httpRequest('/player/snapshot')
}
