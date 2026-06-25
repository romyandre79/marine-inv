<script setup lang="ts">
const route = useRoute()
const auth = useAuth()
const { isCollapsed, toggle, mobileOpen, closeMobile } = useSidebar()
const config = useRuntimeConfig()

const portalUrl = config.public.portalUrl || 'http://localhost:3003'

const navItems = [
  { labelKey: 'nav.back_to_portal', label: 'Back to Portal', path: portalUrl, icon: 'heroicons:home', external: true },
  { labelKey: 'nav.dashboard', label: 'Dashboard', path: '/', icon: 'heroicons:chart-bar' },
  { labelKey: 'nav.inventory_stock', label: 'Inventory Stock', path: '/inventory', icon: 'heroicons:squares-2x2', permission: 'inv:read' },
  { labelKey: 'nav.master_items', label: 'Master Items', path: '/master-items', icon: 'heroicons:circle-stack', permission: 'master_items:read' },
  { labelKey: 'nav.master_warehouses', label: 'Master Warehouses', path: '/master-warehouses', icon: 'heroicons:building-office-2', permission: 'master_warehouses:read' },
  { labelKey: 'nav.master_units', label: 'Master Units', path: '/master-units', icon: 'heroicons:scale', permission: 'master_units:read' },
  { labelKey: 'nav.stock_transfer', label: 'Stock Transfer', path: '/stock-transfer', icon: 'heroicons:arrows-right-left', permission: 'inv:read' },
  { labelKey: 'nav.chat', label: 'Chat & Asisten AI', path: '/chat', icon: 'heroicons:chat-bubble-left-right' }
]

const userRole = computed(() => auth.user?.role || 'viewer')
const userPermissions = computed(() => auth.user?.permissions || [])

const filteredNavItems = computed(() => {
  return navItems.filter(item => {
    if (userRole.value === 'super_admin') return true
    
    if (item.roles && !item.roles.includes(userRole.value)) return false
    
    if (item.permission) {
      const hasRead = userPermissions.value.includes(item.permission)
      const baseName = item.permission.split(':')[0]
      const hasAll = userPermissions.value.includes(`${baseName}:all`)
      if (!hasRead && !hasAll) return false
    }
    
    return true
  })
})
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
      <div class="w-8 h-8 rounded-lg bg-emerald-600 flex items-center justify-center font-bold text-white shrink-0 shadow-lg shadow-emerald-500/20">
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
    <div class="flex-1 py-6 flex flex-col gap-6 overflow-y-auto"
      :class="isCollapsed ? 'px-2 items-center' : 'px-4'"
    >
      <nav class="flex flex-col gap-1 w-full">
        <template v-for="item in filteredNavItems" :key="item.path">
          <!-- External Links -->
          <a
            v-if="item.external"
            :href="item.path"
            class="flex items-center rounded-xl text-sm font-medium transition-all duration-200 text-slate-400 hover:bg-slate-800 hover:text-slate-200"
            :class="isCollapsed ? 'w-10 h-10 justify-center p-0' : 'gap-3 px-4 py-3 w-full'"
            :title="isCollapsed ? $t(item.labelKey) : ''"
            @click="closeMobile"
          >
            <Icon :name="item.icon" class="w-5 h-5 shrink-0" />
            <span v-if="!isCollapsed" class="truncate">{{ $t(item.labelKey) }}</span>
          </a>
          <!-- Internal Route Links -->
          <NuxtLink
            v-else
            :to="item.path"
            class="flex items-center rounded-xl text-sm font-medium transition-all duration-200"
            :class="[
              isCollapsed ? 'w-10 h-10 justify-center p-0' : 'gap-3 px-4 py-3 w-full',
              route.path === item.path
                ? 'bg-emerald-600 text-white shadow-lg shadow-emerald-600/20'
                : 'text-slate-400 hover:bg-slate-800 hover:text-slate-200'
            ]"
            :title="isCollapsed ? $t(item.labelKey) : ''"
            @click="closeMobile"
          >
            <Icon :name="item.icon" class="w-5 h-5 shrink-0" />
            <span v-if="!isCollapsed" class="truncate">{{ $t(item.labelKey) }}</span>
          </NuxtLink>
        </template>
      </nav>

    </div>

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


