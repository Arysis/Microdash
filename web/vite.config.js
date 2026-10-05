import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { VitePWA } from 'vite-plugin-pwa'

export default defineConfig({
  plugins: [
    vue(),
    VitePWA({
      registerType: 'autoUpdate',
      includeAssets: ['icon.svg', 'apple-touch-icon.png'],
      manifest: {
        name: 'Microdash',
        short_name: 'Microdash',
        description: 'Gestion de ta micro-entreprise : trésorerie, cotisations, échéances.',
        lang: 'fr',
        theme_color: '#1f5f8b',
        background_color: '#f6f7f9',
        display: 'standalone',
        start_url: '/',
        icons: [
          { src: 'icon-192.png', sizes: '192x192', type: 'image/png' },
          { src: 'icon-512.png', sizes: '512x512', type: 'image/png' },
          { src: 'icon-512.png', sizes: '512x512', type: 'image/png', purpose: 'maskable' },
        ],
      },
      workbox: {
        navigateFallbackDenylist: [/^\/api\//],
        runtimeCaching: [
          {
            // Lecture : réseau d'abord, dernières données en cache hors ligne.
            urlPattern: ({ url, request }) => url.pathname.startsWith('/api/') && request.method === 'GET',
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
