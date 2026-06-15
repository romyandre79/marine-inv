<script setup lang="ts">
const route = useRoute()
const auth = useAuth()
const { isCollapsed, toggle, mobileOpen, closeMobile } = useSidebar()
const config = useRuntimeConfig()

const portalUrl = config.public.portalUrl || 'http://localhost:3003'

const navItems = [
  { label: 'Back to Portal', path: portalUrl, icon: 'heroicons:arrow-left-on-rectangle' },
  { label: 'Inventory Stock', path: '/inventory', icon: 'heroicons:squares-2x2' },
  { label: 'Master Items', path: '/master-items', icon: 'heroicons:circle-stack', roles: ['super_admin', 'company_admin', 'admin'] }
]

const userRole = computed(() => auth.user?.role || 'viewer')
const userPermissions = computed(() => auth.user?.permissions || [])

const filteredNavItems = computed(() => {
  return navItems.filter(item => {
    if (!item.roles && !item.permissions) return true
    
    if (userRole.value === 'super_admin') return true
    
    const hasRole = item.roles ? item.roles.includes(userRole.value) : false
    const hasPermission = item.permissions ? item.permissions.some(p => userPermissions.value.includes(p)) : false
    
    if (item.roles && !item.permissions) return hasRole
    if (item.permissions && !item.roles) return hasPermission
    return hasRole || hasPermission
  })
})

const isExternal = (path?: string) => {
  return path ? path.startsWith('http') : false
}
</script>

<template>
  <aside
    class="bg-slate-900 text-slate-100 flex flex-col border-r border-slate-800 shrink-0 transition-all duration-300 ease-in-out fixed inset-y-0 left-0 z-30 md:relative md:z-auto"
    :class="[
      isCollapsed ? 'w-16' : 'w-72',
      mobileOpen ? 'translate-x-0' : '-translate-x-full md:translate-x-0'
    ]"
  >
    <!-- Branding -->
    <div class="h-[72px] flex items-center border-b border-slate-800 shrink-0 relative"
      :class="isCollapsed ? 'justify-center px-0' : 'gap-3 px-4'"
    >
      <div class="w-8 h-8 rounded-lg bg-emerald-600 flex items-center justify-center font-bold text-white shrink-0">
        INV
      </div>
      <div v-if="!isCollapsed" class="flex-1 min-w-0 overflow-hidden">
        <h1 class="text-sm font-bold tracking-wide text-white uppercase whitespace-nowrap">Inventory System</h1>
        <span class="text-[10px] text-slate-500 font-semibold tracking-wider uppercase">Marine Vessel Portal</span>
      </div>

      <!-- Collapse toggle button -->
      <button
        @click="toggle"
        class="shrink-0 w-7 h-7 rounded-lg flex items-center justify-center text-slate-400 hover:bg-slate-800 hover:text-slate-200 transition-colors"
        :class="isCollapsed ? 'absolute -right-3.5 top-1/2 -translate-y-1/2 bg-slate-900 border border-slate-700 shadow-md z-10' : ''"
        :title="isCollapsed ? 'Expand sidebar' : 'Collapse sidebar'"
      >
        <Icon
          :name="isCollapsed ? 'heroicons:chevron-right' : 'heroicons:chevron-left'"
          class="w-4 h-4"
        />
      </button>
    </div>

    <!-- Navigation -->
    <nav class="flex-1 py-6 flex flex-col gap-1 overflow-y-auto"
      :class="isCollapsed ? 'px-2 items-center' : 'px-4'"
    >
      <template v-for="item in filteredNavItems" :key="item.path">
        <!-- External Links (Back to Portal) -->
        <a
          v-if="isExternal(item.path)"
          :href="item.path"
          class="flex items-center rounded-lg text-sm font-medium transition-all duration-200 text-slate-400 hover:bg-slate-800 hover:text-slate-200"
          :class="isCollapsed ? 'w-10 h-10 justify-center p-0' : 'gap-3 px-4 py-3 w-full'"
          :title="isCollapsed ? item.label : ''"
          @click="closeMobile"
        >
          <Icon :name="item.icon" class="w-5 h-5 shrink-0" />
          <span v-if="!isCollapsed" class="truncate">{{ item.label }}</span>
        </a>
        <!-- Internal Route Links -->
        <NuxtLink
          v-else
          :to="item.path"
          class="flex items-center rounded-lg text-sm font-medium transition-all duration-200"
          :class="[
            isCollapsed ? 'w-10 h-10 justify-center p-0' : 'gap-3 px-4 py-3 w-full',
            route.path === item.path
              ? 'bg-emerald-600 text-white shadow-md shadow-emerald-600/20'
              : 'text-slate-400 hover:bg-slate-800 hover:text-slate-200'
          ]"
          :title="isCollapsed ? item.label : ''"
          @click="closeMobile"
        >
          <Icon :name="item.icon" class="w-5 h-5 shrink-0" />
          <span v-if="!isCollapsed" class="truncate">{{ item.label }}</span>
        </NuxtLink>
      </template>
    </nav>

    <!-- Footer user summary -->
    <div
      class="border-t border-slate-800/80 bg-slate-950/40 flex items-center"
      :class="isCollapsed ? 'p-2 justify-center flex-col gap-2' : 'p-4 gap-3'"
    >
      <div class="w-9 h-9 rounded-full bg-slate-800 flex items-center justify-center font-bold text-slate-300 ring-2 ring-slate-800 shrink-0">
        {{ auth.user?.name ? auth.user.name.charAt(0).toUpperCase() : 'U' }}
      </div>
      <div v-if="!isCollapsed" class="flex-1 min-w-0">
        <p class="text-xs font-bold text-slate-200 truncate">{{ auth.user?.name }}</p>
        <p class="text-[10px] font-medium text-slate-500 capitalize">{{ auth.user?.role }}</p>
      </div>
      <button
        @click="auth.logout()"
        class="p-1.5 rounded-lg hover:bg-slate-800 text-slate-400 hover:text-slate-200 transition-colors shrink-0"
        title="Logout"
      >
        <Icon name="heroicons:arrow-right-on-rectangle" class="w-5 h-5" />
      </button>
    </div>
  </aside>
</template>
