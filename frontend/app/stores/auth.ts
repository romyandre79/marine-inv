import { defineStore } from 'pinia'

export const useAuthStore = defineStore('auth', () => {
  const token = ref<string | null>(null)
  const refreshToken = ref<string | null>(null)
  const user = ref<any>(null)
  const config = useRuntimeConfig()

  // Load token from cookies (which are shared across localhost ports)
  const tokenCookie = useCookie<string | null>('mms_token', { path: '/' })
  const refreshTokenCookie = useCookie<string | null>('mms_refresh_token', { path: '/' })
  const userCookie = useCookie<any>('mms_user', { path: '/' })

  const loadSession = () => {
    if (tokenCookie.value) {
      token.value = tokenCookie.value
    }
    if (refreshTokenCookie.value) {
      refreshToken.value = refreshTokenCookie.value
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

  async function handleTokenRefresh(): Promise<boolean> {
    const rToken = refreshToken.value || refreshTokenCookie.value
    if (!rToken) return false

    try {
      const response = await $fetch<any>(`${config.public.mmsApiUrl || config.public.apiUrl}/auth/refresh`, {
        method: 'POST',
        body: { refresh_token: rToken }
      })

      if (response.success) {
        token.value = response.data.token
        refreshToken.value = response.data.refresh_token
        user.value = response.data.user

        tokenCookie.value = response.data.token
        refreshTokenCookie.value = response.data.refresh_token
        userCookie.value = response.data.user
        return true
      }
      return false
    } catch (e) {
      console.error('Token refresh failed:', e)
      return false
    }
  }

  function logout() {
    token.value = null
    refreshToken.value = null
    user.value = null
    tokenCookie.value = null
    refreshTokenCookie.value = null
    userCookie.value = null

    // Redirect to main MMS portal gateway with redirect_back query
    if (import.meta.client) {
      const currentUrl = window.location.href
      window.location.href = `${config.public.portalUrl}/login?redirect_back=${encodeURIComponent(currentUrl)}`
    }
  }

  return {
    token,
    refreshToken,
    user,
    isAuthenticated,
    loadSession,
    logout,
    handleTokenRefresh
  }
})
