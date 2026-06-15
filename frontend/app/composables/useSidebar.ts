const isCollapsed = ref(false)
const mobileOpen = ref(false)

let hydrated = false

export const useSidebar = () => {
  if (import.meta.client && !hydrated) {
    hydrated = true
    const saved = localStorage.getItem('sidebar-collapsed')
    if (saved !== null) isCollapsed.value = saved === 'true'
  }

  const toggle = () => {
    isCollapsed.value = !isCollapsed.value
    if (import.meta.client) {
      localStorage.setItem('sidebar-collapsed', String(isCollapsed.value))
    }
  }

  const openMobile = () => { mobileOpen.value = true }
  const closeMobile = () => { mobileOpen.value = false }
  const toggleMobile = () => { mobileOpen.value = !mobileOpen.value }

  return { isCollapsed, toggle, mobileOpen, openMobile, closeMobile, toggleMobile }
}
