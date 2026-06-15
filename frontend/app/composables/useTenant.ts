import { useTenantStore } from '../stores/tenant'

export const useTenant = () => {
  return useTenantStore()
}
