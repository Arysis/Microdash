import { createApp } from 'vue'
import '@fontsource/bricolage-grotesque/latin-600.css'
import '@fontsource/bricolage-grotesque/latin-700.css'
import '@fontsource/ibm-plex-sans/latin-400.css'
import '@fontsource/ibm-plex-sans/latin-500.css'
import '@fontsource/ibm-plex-sans/latin-600.css'
import App from './App.vue'
import InfoBulle from './components/InfoBulle.vue'
import router from './router.js'
import './style.css'

// InfoBulle sert sur presque tous les écrans : enregistré une fois pour toute l'app.
createApp(App).use(router).component('InfoBulle', InfoBulle).mount('#app')
