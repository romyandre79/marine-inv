import tailwindcss from '@tailwindcss/vite'

// https://nuxt.com/docs/api/configuration/nuxt-config
export default defineNuxtConfig({
  compatibilityDate: '2026-06-12',
  devtools: { enabled: true },
  devServer: {
    port: parseInt(process.env.PORT || '3007')
  },
  
  // Enabled Nuxt 4 directory structure
  future: {
    compatibilityVersion: 4,
  },

  app: {
    head: {
      title: 'Inventory Management System',
      meta: [
        { charset: 'utf-8' },
        { name: 'viewport', content: 'width=device-width, initial-scale=1' },
        { name: 'description', content: 'Inventory Management System for Marine vessel tracking' }
      ],
      link: [
        { rel: 'icon', type: 'image/svg+xml', href: '/favicon.svg' }
      ]
    }
  },

  modules: [
    '@pinia/nuxt',
    '@nuxt/icon',
    '@nuxtjs/i18n'
  ],

  i18n: {
    langDir: 'app/i18n/locales',
    restructureDir: false,
    defaultLocale: 'id',
    locales: [
      { code: 'id', name: 'Indonesia', file: 'id.json' },
      { code: 'en', name: 'English', file: 'en.json' },
      { code: 'zh', name: '中文', file: 'zh.json' }
    ],
    strategy: 'no_prefix',
    bundle: {
      optimizeTranslationDirective: false
    }
  },

  css: ['~/assets/css/main.css'],

  vite: {
    plugins: [
      tailwindcss(),
    ]
  },

  runtimeConfig: {
    public: {
      apiUrl: process.env.NUXT_PUBLIC_API_URL || 'http://localhost:3013/api/v1',
      portalUrl: process.env.NUXT_PUBLIC_PORTAL_URL || 'http://localhost:3003',
      fmsApiUrl: process.env.NUXT_PUBLIC_FMS_API_URL || 'http://localhost:3006/api/v1',
      mmsApiUrl: process.env.NUXT_PUBLIC_MMS_API_URL || 'http://localhost:3004/api/v1'
    }
  }
})
