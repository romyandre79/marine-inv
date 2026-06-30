import { useAuthStore } from '~/stores/auth'

export default defineNuxtRouteMiddleware((to, from) => {
  // Only execute on client-side
  if (!import.meta.client) return

  // Prevent logging login page or redirect paths
  if (to.path === '/login') return

  // Don't log if route path is identical
  if (to.path === from.path) return

  const authStore = useAuthStore()
  const token = authStore.token || useCookie('mms_token').value

  if (token) {
    const config = useRuntimeConfig()
    const mmsApiUrl = config.public.mmsApiUrl || config.public.apiUrl
    const appName = config.public.appName || document.title || 'Inventory Management System'

    $fetch(`${mmsApiUrl}/logs`, {
      method: 'POST',
      headers: {
        'Authorization': `Bearer ${token}`,
        'Content-Type': 'application/json'
      },
      body: {
        action: 'open_menu',
        entity_type: 'menu',
        details: JSON.stringify({
          path: to.path,
          app: appName
        })
      }
    }).catch((err) => {
      console.error('Failed to send navigation log:', err)
    })
  }
})
