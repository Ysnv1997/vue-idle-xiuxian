import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import router from './router'
import { useSessionStore } from './stores/session'

const app = createApp(App)
const pinia = createPinia()

app.use(pinia)

const sessionStore = useSessionStore(pinia)

router.beforeEach(to => {
  if (!sessionStore.accessToken && !sessionStore.refreshToken) {
    sessionStore.hydrateTokens()
  }

  const publicPaths = new Set(['/', '/auth/callback'])
  const isPublicPath = publicPaths.has(to.path)

  if (!sessionStore.isAuthenticated && !isPublicPath) {
    return '/'
  }

  if (sessionStore.isAuthenticated && to.path === '/') {
    return '/cultivation'
  }

  return true
})

app.use(router)
app.mount('#app')
