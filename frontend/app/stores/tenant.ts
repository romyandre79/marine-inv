import { defineStore } from 'pinia'
import { useAuthStore } from './auth'

export interface Company {
  id: string
  name: string
  code?: string
}

export const useTenantStore = defineStore('tenant', () => {
  const authStore = useAuthStore()
  const companies = ref<Company[]>([])
  
  // Read tenant cookies (shared across localhost)
  const activeTenantId = useCookie<string | null>('tenant_id', { path: '/' })
  const lastTenantId = useCookie<string | null>('last_tenant_id', { maxAge: 60 * 60 * 24 * 365, path: '/' })
  
  const currentTenant = computed(() => 
    companies.value.find(c => c.id === activeTenantId.value) || null
  )

  const fetchCompanies = async () => {
    try {
      const res = await $fetch<any>('http://localhost:3004/api/v1/companies', {
        headers: {
          Authorization: `Bearer ${authStore.token}`
        }
      })
      if (res.success && Array.isArray(res.data)) {
        companies.value = res.data.map((c: any) => ({
          id: c.ID !== undefined ? c.ID : c.id,
          name: c.Name !== undefined ? c.Name : c.name,
          code: c.Code !== undefined ? c.Code : c.code
        }))

        // Handle activeTenantId initialization
        if (!activeTenantId.value && companies.value.length > 0) {
          const hasAccess = companies.value.some(c => c.id === lastTenantId.value)
          activeTenantId.value = hasAccess ? lastTenantId.value : companies.value[0].id
        }
      }
    } catch (e: any) {
      console.error('Failed to fetch companies from MMS API:', e)
    }
  }

  const selectTenant = (id: string) => {
    activeTenantId.value = id
    lastTenantId.value = id
    if (import.meta.client) {
      window.location.reload()
    }
  }

  return {
    companies,
    activeTenantId,
    currentTenant,
    fetchCompanies,
    selectTenant
  }
})
