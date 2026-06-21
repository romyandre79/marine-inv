<script setup lang="ts">
import { useAuthStore } from '~/stores/auth'
import { useTenantStore } from '~/stores/tenant'

const authStore = useAuthStore()
const tenantStore = useTenantStore()
const config = useRuntimeConfig()

// State
const inventory = ref<any[]>([])
const transfers = ref<any[]>([])
const warehouses = ref<any[]>([])
const loading = ref(false)
const errorMsg = ref('')

// On mount, load all necessary dashboard metrics
onMounted(async () => {
  if (!authStore.isAuthenticated) {
    window.location.href = `${config.public.portalUrl}/login`
  } else {
    await tenantStore.fetchCompanies()
    loadDashboardData()
  }
})

// Re-fetch when active tenant changes
watch(() => tenantStore.activeTenantId, () => {
  loadDashboardData()
})

async function loadDashboardData() {
  loading.value = true
  errorMsg.value = ''
  try {
    await Promise.all([
      fetchInventory(),
      fetchTransfers(),
      fetchWarehouses()
    ])
  } catch (error: any) {
    errorMsg.value = 'Failed to load dashboard metrics.'
  } finally {
    loading.value = false
  }
}

async function fetchInventory() {
  const companyQuery = tenantStore.activeTenantId ? `?company_id=${tenantStore.activeTenantId}` : ''
  const res = await $fetch<any>(`${config.public.apiUrl}/inventory${companyQuery}`, {
    headers: { Authorization: `Bearer ${authStore.token}` }
  })
  if (res.success) {
    inventory.value = res.data
  }
}

async function fetchTransfers() {
  const companyQuery = tenantStore.activeTenantId ? `?company_id=${tenantStore.activeTenantId}` : ''
  const res = await $fetch<any>(`${config.public.apiUrl}/stock-transfers${companyQuery}`, {
    headers: { Authorization: `Bearer ${authStore.token}` }
  })
  if (res.success) {
    transfers.value = res.data
  }
}

async function fetchWarehouses() {
  const companyQuery = tenantStore.activeTenantId ? `?company_id=${tenantStore.activeTenantId}` : ''
  const res = await $fetch<any>(`${config.public.apiUrl}/master-warehouses${companyQuery}`, {
    headers: { Authorization: `Bearer ${authStore.token}` }
  })
  if (res.success) {
    warehouses.value = res.data
  }
}

// Computations
const totalUniqueItems = computed(() => inventory.value.length)
const totalQuantity = computed(() => inventory.value.reduce((sum, item) => sum + item.quantity, 0))

const lowStockItems = computed(() => {
  return inventory.value.filter(item => item.quantity <= item.minimum_stock && item.quantity > 0)
})

const outOfStockItems = computed(() => {
  return inventory.value.filter(item => item.quantity === 0)
})

const pendingTransfers = computed(() => {
  return transfers.value.filter(t => t.status === 'pending')
})

const recentTransfers = computed(() => {
  return [...transfers.value]
    .sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime())
    .slice(0, 5)
})

// Visual distribution data
const warehouseDistribution = computed(() => {
  const map: Record<string, { count: number; quantity: number }> = {}
  
  // Initialize with known warehouses
  warehouses.value.forEach(w => {
    map[w.name] = { count: 0, quantity: 0 }
  })
  
  // Distribute items
  inventory.value.forEach(item => {
    const loc = item.location || 'Unknown Location'
    if (!map[loc]) {
      map[loc] = { count: 0, quantity: 0 }
    }
    map[loc].count++
    map[loc].quantity += item.quantity
  })

  return Object.entries(map).map(([name, data]) => ({
    name,
    count: data.count,
    quantity: data.quantity
  })).sort((a, b) => b.quantity - a.quantity)
})

const stockHealthPercentage = computed(() => {
  if (inventory.value.length === 0) return { healthy: 100, low: 0, out: 0 }
  const total = inventory.value.length
  const outCount = outOfStockItems.value.length
  const lowCount = lowStockItems.value.length
  const healthyCount = total - outCount - lowCount

  return {
    healthy: Math.round((healthyCount / total) * 100),
    low: Math.round((lowCount / total) * 100),
    out: Math.round((outCount / total) * 100)
  }
})
</script>

<template>
  <div class="space-y-6 text-slate-100">
    <!-- Welcome Header -->
    <div class="flex flex-col md:flex-row justify-between items-start md:items-center gap-4">
      <div>
        <h1 class="text-2xl font-extrabold tracking-tight text-white flex items-center gap-2">
          <span>{{ $t('nav.dashboard') }}</span>
          <span class="text-xs bg-emerald-600/20 text-emerald-400 border border-emerald-500/20 px-2.5 py-0.5 rounded-full font-bold uppercase tracking-wider">
            Special Inventory
          </span>
        </h1>
        <p class="text-sm text-slate-400 mt-1">
          Welcome back, <span class="text-slate-200 font-semibold">{{ authStore.user?.name || 'User' }}</span>. Here is the operational summary of your vessel inventory.
        </p>
      </div>

      <div class="flex gap-3">
        <NuxtLink
          to="/inventory"
          class="bg-slate-900/50 hover:bg-slate-800 border border-slate-800 text-slate-300 font-semibold text-sm px-4 py-2.5 rounded-xl transition duration-200 flex items-center gap-2"
        >
          <Icon name="heroicons:squares-2x2" class="w-4 h-4" />
          View Full Stock
        </NuxtLink>
        <NuxtLink
          to="/stock-transfer"
          class="bg-emerald-600 hover:bg-emerald-500 text-white font-semibold text-sm px-4 py-2.5 rounded-xl transition duration-200 shadow-lg shadow-emerald-600/20 flex items-center gap-2"
        >
          <Icon name="heroicons:arrows-right-left" class="w-4 h-4" />
          Stock Transfer
        </NuxtLink>
      </div>
    </div>

    <!-- Error message banner -->
    <div v-if="errorMsg" class="p-4 bg-rose-500/10 border border-rose-500/30 text-rose-400 rounded-xl flex items-center space-x-2">
      <Icon name="heroicons:exclamation-triangle" class="w-5 h-5 text-rose-400" />
      <span>{{ errorMsg }}</span>
    </div>

    <!-- Loading skeleton -->
    <div v-if="loading" class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
      <div v-for="n in 4" :key="n" class="bg-slate-900/40 border border-slate-800/80 p-5 rounded-2xl h-28 animate-pulse"></div>
    </div>

    <!-- Main Content -->
    <template v-else>
      <!-- KPI Stats Row -->
      <div class="grid grid-cols-2 lg:grid-cols-4 gap-4">
        <!-- Card: Total Items -->
        <div class="bg-slate-900/50 border border-slate-800 p-5 rounded-2xl flex items-center justify-between shadow-xl relative overflow-hidden group hover:border-slate-700 transition">
          <div class="space-y-1">
            <span class="text-xs font-bold text-slate-500 uppercase tracking-wider block">Total Items</span>
            <span class="text-3xl font-extrabold text-slate-100 block">{{ totalUniqueItems }}</span>
          </div>
          <div class="w-12 h-12 rounded-xl bg-slate-850 flex items-center justify-center text-slate-400 group-hover:text-emerald-400 transition">
            <Icon name="heroicons:circle-stack" class="w-6 h-6" />
          </div>
        </div>

        <!-- Card: Total Quantity -->
        <div class="bg-slate-900/50 border border-slate-800 p-5 rounded-2xl flex items-center justify-between shadow-xl relative overflow-hidden group hover:border-slate-700 transition">
          <div class="space-y-1">
            <span class="text-xs font-bold text-slate-500 uppercase tracking-wider block">Total Stock Volume</span>
            <span class="text-3xl font-extrabold text-slate-100 block">{{ totalQuantity }}</span>
          </div>
          <div class="w-12 h-12 rounded-xl bg-slate-850 flex items-center justify-center text-slate-400 group-hover:text-emerald-400 transition">
            <Icon name="heroicons:archive-box" class="w-6 h-6" />
          </div>
        </div>

        <!-- Card: Low Stock alerts -->
        <NuxtLink
          to="/inventory"
          class="bg-slate-900/50 border border-slate-800 p-5 rounded-2xl flex items-center justify-between shadow-xl relative overflow-hidden group hover:border-amber-500/30 transition text-left"
        >
          <div class="space-y-1">
            <span class="text-xs font-bold text-slate-500 uppercase tracking-wider block">Low Stock Items</span>
            <span class="text-3xl font-extrabold block" :class="lowStockItems.length > 0 ? 'text-amber-400' : 'text-slate-350'">
              {{ lowStockItems.length }}
            </span>
          </div>
          <div class="w-12 h-12 rounded-xl bg-amber-500/10 flex items-center justify-center text-amber-400/80 transition">
            <Icon name="heroicons:exclamation-triangle" class="w-6 h-6" />
          </div>
        </NuxtLink>

        <!-- Card: Pending Transfers -->
        <NuxtLink
          to="/stock-transfer"
          class="bg-slate-900/50 border border-slate-800 p-5 rounded-2xl flex items-center justify-between shadow-xl relative overflow-hidden group hover:border-emerald-500/30 transition text-left"
        >
          <div class="space-y-1">
            <span class="text-xs font-bold text-slate-500 uppercase tracking-wider block">Pending Transfers</span>
            <span class="text-3xl font-extrabold block" :class="pendingTransfers.length > 0 ? 'text-emerald-400' : 'text-slate-350'">
              {{ pendingTransfers.length }}
            </span>
          </div>
          <div class="w-12 h-12 rounded-xl bg-emerald-500/10 flex items-center justify-center text-emerald-400/80 transition">
            <Icon name="heroicons:arrows-right-left" class="w-6 h-6" />
          </div>
        </NuxtLink>
      </div>

      <!-- Charts & Visuals Grid -->
      <div class="grid grid-cols-1 lg:grid-cols-3 gap-6">
        <!-- Stock Health circular breakdown -->
        <div class="bg-slate-900/50 border border-slate-800 p-6 rounded-2xl shadow-xl flex flex-col justify-between">
          <div class="mb-4">
            <h3 class="text-sm font-bold uppercase tracking-wider text-slate-400">Stock Integrity Health</h3>
            <p class="text-xs text-slate-500">Breakdown of inventory warning signals</p>
          </div>

          <div class="relative flex items-center justify-center py-6">
            <!-- Custom circular gauge representation using Tailwind & inline styles -->
            <div class="w-36 h-36 rounded-full border-8 border-slate-800 flex flex-col items-center justify-center relative">
              <span class="text-2xl font-black text-white">{{ stockHealthPercentage.healthy }}%</span>
              <span class="text-[9px] uppercase tracking-wider text-emerald-400 font-bold">Healthy Stock</span>
            </div>
          </div>

          <div class="space-y-2 mt-2">
            <div class="flex items-center justify-between text-xs">
              <span class="flex items-center gap-2 font-medium text-slate-400">
                <span class="w-3 h-3 rounded bg-emerald-500 block shrink-0"></span>
                Healthy Items
              </span>
              <span class="font-bold text-slate-200">{{ stockHealthPercentage.healthy }}%</span>
            </div>
            <div class="flex items-center justify-between text-xs">
              <span class="flex items-center gap-2 font-medium text-slate-400">
                <span class="w-3 h-3 rounded bg-amber-500 block shrink-0 animate-pulse"></span>
                Low Stock Limit
              </span>
              <span class="font-bold text-slate-200">{{ stockHealthPercentage.low }}%</span>
            </div>
            <div class="flex items-center justify-between text-xs">
              <span class="flex items-center gap-2 font-medium text-slate-400">
                <span class="w-3 h-3 rounded bg-rose-500 block shrink-0"></span>
                Out of Stock
              </span>
              <span class="font-bold text-slate-200">{{ stockHealthPercentage.out }}%</span>
            </div>
          </div>
        </div>

        <!-- Warehouse Distribution -->
        <div class="bg-slate-900/50 border border-slate-800 p-6 rounded-2xl shadow-xl lg:col-span-2 flex flex-col justify-between">
          <div class="mb-4">
            <h3 class="text-sm font-bold uppercase tracking-wider text-slate-400">Warehouse Stock Distribution</h3>
            <p class="text-xs text-slate-500">Distribution volume across physical warehouses</p>
          </div>

          <div class="space-y-4 my-auto">
            <div v-for="w in warehouseDistribution.slice(0, 4)" :key="w.name" class="space-y-1.5">
              <div class="flex items-center justify-between text-xs">
                <span class="font-semibold text-slate-350 truncate flex items-center gap-1">
                  <Icon name="heroicons:building-office" class="w-4 h-4 text-slate-500" />
                  {{ w.name }}
                </span>
                <span class="font-bold text-slate-200">{{ w.quantity }} items <span class="text-[10px] text-slate-500 font-medium">({{ w.count }} categories)</span></span>
              </div>
              <div class="w-full bg-slate-950 h-2.5 rounded-full overflow-hidden border border-slate-850">
                <div
                  class="bg-emerald-500 h-full rounded-full transition-all duration-500"
                  :style="{ width: `${totalQuantity ? (w.quantity / totalQuantity) * 100 : 0}%` }"
                ></div>
              </div>
            </div>
            <div v-if="warehouseDistribution.length === 0" class="text-center text-slate-500 py-6 text-sm">
              No warehouses or stock mappings found.
            </div>
          </div>

          <div class="mt-4 pt-4 border-t border-slate-800/60 flex justify-between items-center text-xs">
            <span class="text-slate-500">Registered locations: <span class="text-slate-300 font-bold">{{ warehouses.length }}</span></span>
            <NuxtLink to="/master-warehouses" class="text-emerald-400 hover:underline flex items-center gap-0.5">
              Manage Locations →
            </NuxtLink>
          </div>
        </div>
      </div>

      <!-- Bottom Alerts & Logs Section -->
      <div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <!-- Stock Critical Warnings -->
        <div class="bg-slate-900/50 border border-slate-800 p-6 rounded-2xl shadow-xl flex flex-col">
          <div class="mb-4 flex justify-between items-center">
            <div>
              <h3 class="text-sm font-bold uppercase tracking-wider text-slate-400">Critical Stock Signals</h3>
              <p class="text-xs text-slate-500">Items that require immediate replenishment</p>
            </div>
            <span class="bg-rose-500/10 text-rose-450 border border-rose-500/20 px-2 py-0.5 rounded text-[10px] uppercase font-bold tracking-wider">
              Replenish List
            </span>
          </div>

          <div class="flex-1 overflow-y-auto max-h-[300px] divide-y divide-slate-800/50 pr-1">
            <!-- Out of Stock Loop -->
            <div v-for="item in outOfStockItems" :key="item.id" class="py-3 flex items-center justify-between text-sm group">
              <div>
                <div class="font-bold text-slate-200 group-hover:text-white transition">{{ item.name }}</div>
                <div class="text-xs text-slate-500 flex items-center gap-3 mt-1">
                  <span>PN: <span class="font-mono text-slate-400">{{ item.part_number || '-' }}</span></span>
                  <span class="flex items-center gap-0.5"><Icon name="heroicons:building-office" class="w-3.5 h-3.5" /> {{ item.location }}</span>
                </div>
              </div>
              <div class="flex items-center gap-3">
                <span class="text-xs uppercase font-extrabold text-rose-500 bg-rose-500/10 px-2 py-1 rounded border border-rose-500/20">Out of Stock</span>
                <NuxtLink
                  to="/stock-transfer"
                  class="bg-slate-850 hover:bg-emerald-600 text-slate-350 hover:text-white p-1.5 rounded-lg border border-slate-800 transition"
                  title="Request Stock Transfer"
                >
                  <Icon name="heroicons:arrows-right-left" class="w-4 h-4" />
                </NuxtLink>
              </div>
            </div>

            <!-- Low Stock Loop -->
            <div v-for="item in lowStockItems" :key="item.id" class="py-3 flex items-center justify-between text-sm group">
              <div>
                <div class="font-bold text-slate-200 group-hover:text-white transition">{{ item.name }}</div>
                <div class="text-xs text-slate-500 flex items-center gap-3 mt-1">
                  <span>PN: <span class="font-mono text-slate-400">{{ item.part_number || '-' }}</span></span>
                  <span class="flex items-center gap-0.5"><Icon name="heroicons:building-office" class="w-3.5 h-3.5" /> {{ item.location }}</span>
                </div>
              </div>
              <div class="flex items-center gap-3">
                <div class="text-right">
                  <div class="text-xs font-extrabold text-amber-400">{{ item.quantity }} {{ item.unit }} left</div>
                  <div class="text-[9px] text-slate-500 font-semibold uppercase">Min: {{ item.minimum_stock }}</div>
                </div>
                <NuxtLink
                  to="/stock-transfer"
                  class="bg-slate-850 hover:bg-emerald-600 text-slate-350 hover:text-white p-1.5 rounded-lg border border-slate-800 transition"
                  title="Request Stock Transfer"
                >
                  <Icon name="heroicons:arrows-right-left" class="w-4 h-4" />
                </NuxtLink>
              </div>
            </div>

            <div v-if="outOfStockItems.length === 0 && lowStockItems.length === 0" class="text-center text-slate-500 py-12 text-sm italic">
              ✨ All stock items are healthy.
            </div>
          </div>
        </div>

        <!-- Recent Activity Logs / Transfers -->
        <div class="bg-slate-900/50 border border-slate-800 p-6 rounded-2xl shadow-xl flex flex-col">
          <div class="mb-4 flex justify-between items-center">
            <div>
              <h3 class="text-sm font-bold uppercase tracking-wider text-slate-400">Recent Stock Movements</h3>
              <p class="text-xs text-slate-500">Latest transfer request actions</p>
            </div>
            <NuxtLink to="/stock-transfer" class="text-xs text-emerald-450 hover:underline">
              See All
            </NuxtLink>
          </div>

          <div class="flex-1 overflow-y-auto max-h-[300px] divide-y divide-slate-800/50 pr-1">
            <div v-for="t in recentTransfers" :key="t.id" class="py-3 flex items-center justify-between text-sm">
              <div class="min-w-0 pr-2">
                <div class="font-bold text-slate-200 truncate">{{ t.item_name }}</div>
                <div class="text-xs text-slate-500 flex items-center gap-1.5 mt-1">
                  <span class="truncate font-semibold">{{ t.source_warehouse }}</span>
                  <Icon name="heroicons:arrow-small-right" class="w-3.5 h-3.5" />
                  <span class="truncate font-semibold">{{ t.target_warehouse }}</span>
                </div>
              </div>
              <div class="flex items-center gap-3 shrink-0">
                <div class="text-right">
                  <div class="font-bold text-slate-200 text-xs">{{ t.quantity }} {{ t.unit }}</div>
                  <span
                    class="text-[9px] uppercase font-bold tracking-wider"
                    :class="{
                      'text-amber-450': t.status === 'pending',
                      'text-emerald-400': t.status === 'approved',
                      'text-rose-500': t.status === 'rejected'
                    }"
                  >
                    {{ t.status }}
                  </span>
                </div>
              </div>
            </div>

            <div v-if="recentTransfers.length === 0" class="text-center text-slate-500 py-12 text-sm italic">
              No recent stock transfer activity recorded.
            </div>
          </div>
        </div>
      </div>
    </template>
  </div>
</template>
