import { createRouter, createWebHashHistory } from 'vue-router'
import Home from '../views/Home.vue'
import Cultivation from '../views/Cultivation.vue'
import Inventory from '../views/Inventory.vue'
import Exploration from '../views/Exploration.vue'
import Achievements from '../views/Achievements.vue'
import Settings from '../views/Settings.vue'
import Alchemy from '../views/Alchemy.vue'
import Dungeon from '../views/Dungeon.vue'
import Gacha from '../views/Gacha.vue'
import AuthCallback from '../views/AuthCallback.vue'
import Ranking from '../views/Ranking.vue'
import Auction from '../views/Auction.vue'
import Chat from '../views/Chat.vue'

const routes = [
  {
    path: '/',
    name: 'Home',
    component: Home
  },
  {
    path: '/cultivation',
    name: 'Cultivation',
    component: Cultivation
  },
  {
    path: '/inventory',
    name: 'Inventory',
    component: Inventory
  },
  {
    path: '/exploration',
    name: 'Exploration',
    component: Exploration
  },
  {
    path: '/achievements',
    name: 'Achievements',
    component: Achievements
  },
  {
    path: '/settings',
    name: 'Settings',
    component: Settings
  },
  {
    path: '/alchemy',
    name: 'alchemy',
    component: Alchemy
  },
  {
    path: '/dungeon',
    name: 'Dungeon',
    component: Dungeon
  },
  {
    path: '/gacha',
    name: 'Gacha',
    component: Gacha
  },
  {
    path: '/ranking',
    name: 'Ranking',
    component: Ranking
  },
  {
    path: '/auction',
    name: 'Auction',
    component: Auction
  },
  {
    path: '/chat',
    name: 'Chat',
    component: Chat
  },
  {
    path: '/auth/callback',
    name: 'AuthCallback',
    component: AuthCallback
  }
]

const router = createRouter({
  history: createWebHashHistory(),
  routes
})

export default router
