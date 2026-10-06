import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { VitePWA } from 'vite-plugin-pwa'
import { couleurs, site } from './theme.js'

// Remplace les %CLE% de index.html par les valeurs de theme.js : une seule source pour les couleurs.
const valeursHtml = {
  THEME_COLOR: couleurs.safran,
  SITE_NOM: site.nom,
  SITE_DESCRIPTION: site.description,
  SITE_URL: process.env.SITE_URL || site.url,
}
const injecterTheme = {
  name: 'injecter-theme',
  transformIndexHtml: (html) => html.replace(/%(THEME_COLOR|SITE_NOM|SITE_DESCRIPTION|SITE_URL)%/g, (_, cle) => valeursHtml[cle]),
}

export default defineConfig({
  plugins: [
    vue(),
    injecterTheme,
    VitePWA({
      registerType: 'autoUpdate',
      includeAssets: ['favicon.ico', 'icon.svg', 'apple-touch-icon.png', 'capture-tableau-de-bord.webp'],
      manifest: {
        name: site.nom,
        short_name: site.nom,
        description: site.description,
        lang: 'fr',
        theme_color: couleurs.safran,
        background_color: couleurs.papier,
        display: 'standalone',
        start_url: '/tableau-de-bord',
        icons: [
          { src: 'icon-192.png', sizes: '192x192', type: 'image/png' },
          { src: 'icon-512.png', sizes: '512x512', type: 'image/png' },
          { src: 'icon-maskable-512.png', sizes: '512x512', type: 'image/png', purpose: 'maskable' },
        ],
      },
      workbox: {
        navigateFallbackDenylist: [/^\/api\//],
        runtimeCaching: [
          {
            // Lecture : réseau d'abord, dernières données en cache hors ligne. Les exports restent hors cache.
            urlPattern: ({ url, request }) =>
              url.pathname.startsWith('/api/') && !url.pathname.startsWith('/api/exports/') && request.method === 'GET',
            handler: 'NetworkFirst',
            options: { cacheName: 'api', networkTimeoutSeconds: 5 },
          },
          ...['POST', 'PUT', 'DELETE'].map((method) => ({
            // Saisies hors ligne : mises en file d'attente et rejouées au retour du réseau.
            urlPattern: ({ url }) => url.pathname.startsWith('/api/transactions'),
            handler: 'NetworkOnly',
            method,
            options: { backgroundSync: { name: 'saisies', options: { maxRetentionTime: 7 * 24 * 60 } } },
          })),
        ],
      },
    }),
  ],
  server: {
    proxy: { '/api': process.env.API_URL || 'http://localhost:8080' },
  },
})
