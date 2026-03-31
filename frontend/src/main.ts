import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import router from './router'
import { initSentry } from './sentry'
import 'vue-sonner/style.css'
import './style.css'
import { useTheme } from './composables/useTheme'

// Initialize theme (registers media query listener)
useTheme()

const app = createApp(App)
app.use(createPinia())
app.use(router)
initSentry(app, router)
app.mount('#app')
