import { defineStore } from 'pinia'

export const useAuthStore = defineStore('auth', () => {
  const token = ref<string | null>(null)
  const user = ref<any>(null)
  const config = useRuntimeConfig()

  // Load token from cookies (which are shared across localhost ports)
  const tokenCookie = useCookie<string | null>('mms_token', { path: '/' })
  const userCookie = useCookie<any>('mms_user', { path: '/' })

  const loadSession = () => {
    if (tokenCookie.value) {
      token.value = tokenCookie.value
    }
    if (userCookie.value) {
      try {
        user.value = typeof userCookie.value === 'string' ? JSON.parse(userCookie.value) : userCookie.value
      } catch (e) {
        user.value = userCookie.value
      }
    }
  }

  // Load session initially
  loadSession()

  const isAuthenticated = computed(() => !!token.value)

  function logout() {
    token.value = null
    user.value = null
    tokenCookie.value = null
    userCookie.value = null

    // Redirect to main MMS portal gateway with redirect_back query
    if (import.meta.client) {
      const currentUrl = window.location.href
      window.location.href = `${config.public.portalUrl}/login?redirect_back=${encodeURIComponent(currentUrl)}`
    }
  }

  return {
    token,
    user,
    isAuthenticated,
    loadSession,
    logout
  }
})
