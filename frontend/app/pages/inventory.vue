<script setup lang="ts">
import { useAuthStore } from '~/stores/auth'
import { useTenantStore } from '~/stores/tenant'

const authStore = useAuthStore()
const tenantStore = useTenantStore()
const config = useRuntimeConfig()

// State
const inventory = ref<any[]>([])
const loading = ref(false)
const errorMsg = ref('')
const masterItems = ref<any[]>([])

// Computed metrics
const totalItems = computed(() => inventory.value.length)
const lowStockCount = computed(() => inventory.value.filter((i: any) => i.quantity <= i.minimum_stock && i.quantity > 0).length)
const outOfStockCount = computed(() => inventory.value.filter((i: any) => i.quantity === 0).length)
const healthyStockCount = computed(() => inventory.value.filter((i: any) => i.quantity > i.minimum_stock).length)

// SSO Check: Redirect to login if not authenticated
onMounted(async () => {
  if (!authStore.isAuthenticated) {
    window.location.href = `${config.public.portalUrl}/login`
  } else {
    await tenantStore.fetchCompanies()
    fetchInventory()
    fetchMasterItems()
  }
})

// Watch active tenant to re-fetch
watch(() => tenantStore.activeTenantId, () => {
  fetchInventory()
  fetchMasterItems()
})

// CRUD State
const showModal = ref(false)
const editingItem = ref<any>(null)
const form = ref({
  name: '',
  part_number: '',
  quantity: 0,
  unit: 'pcs',
  location: '',
  minimum_stock: 5,
  company_id: ''
})

const selectedMasterItemId = ref('')

// Watch selected master item to auto-fill form
watch(selectedMasterItemId, (newId) => {
  if (!newId) return
  const match = masterItems.value.find(m => m.id === newId)
  if (match) {
    form.value.name = match.name
    form.value.part_number = match.part_number || ''
    form.value.unit = match.unit || 'pcs'
  }
})

function hasPermission(permission: string) {
  if (!authStore.user) return false
  if (authStore.user.role === 'super_admin') return true
  return authStore.user.permissions?.includes(permission) || false
}

async function fetchInventory() {
  loading.value = true
  errorMsg.value = ''
  try {
    const companyQuery = tenantStore.activeTenantId ? `?company_id=${tenantStore.activeTenantId}` : ''
    const res = await $fetch<any>(`${config.public.apiUrl}/inventory${companyQuery}`, {
      headers: {
        Authorization: `Bearer ${authStore.token}`
      }
    })
    if (res.success) {
      inventory.value = res.data
    }
  } catch (error: any) {
    errorMsg.value = error.data?.message || 'Failed to connect to Inventory Backend service.'
    if (error.status === 401) {
      authStore.logout()
    }
  } finally {
    loading.value = false
  }
}

async function fetchMasterItems() {
  try {
    const companyQuery = tenantStore.activeTenantId ? `?company_id=${tenantStore.activeTenantId}` : ''
    const res = await $fetch<any>(`${config.public.apiUrl}/master-items${companyQuery}`, {
      headers: {
        Authorization: `Bearer ${authStore.token}`
      }
    })
    if (res.success && Array.isArray(res.data)) {
      masterItems.value = res.data
    }
  } catch (error) {
    console.error('Failed to load master items:', error)
  }
}

function openAddModal() {
  editingItem.value = null
  selectedMasterItemId.value = ''
  form.value = {
    name: '',
    part_number: '',
    quantity: 0,
    unit: 'pcs',
    location: '',
    minimum_stock: 5,
    company_id: tenantStore.activeTenantId || ''
  }
  showModal.value = true
}

function openEditModal(item: any) {
  editingItem.value = item
  
  // Find matching master item by name/part number if any
  const match = masterItems.value.find(m => m.name === item.name && m.part_number === item.part_number)
  selectedMasterItemId.value = match ? match.id : ''

  form.value = {
    name: item.name,
    part_number: item.part_number || '',
    quantity: item.quantity,
    unit: item.unit || 'pcs',
    location: item.location || '',
    minimum_stock: item.minimum_stock || 0,
    company_id: item.company_id || tenantStore.activeTenantId || ''
  }
  showModal.value = true
}

async function saveItem() {
  if (!form.value.name) {
    alert('Please select or write a item name.')
    return
  }
  try {
    let url = `${config.public.apiUrl}/inventory`
    let method = 'POST'
    if (editingItem.value) {
      url = `${config.public.apiUrl}/inventory/${editingItem.value.id}`
      method = 'PUT'
    }

    const res = await $fetch<any>(url, {
      method,
      headers: {
        Authorization: `Bearer ${authStore.token}`,
        'Content-Type': 'application/json'
      },
      body: {
        ...form.value,
        company_id: form.value.company_id || null
      }
    })

    if (res.success) {
      showModal.value = false
      fetchInventory()
    }
  } catch (error: any) {
    alert(error.data?.message || 'Failed to save inventory item')
  }
}

async function deleteItem(id: string) {
  if (!confirm('Are you sure you want to delete this inventory item?')) return
  try {
    const res = await $fetch<any>(`${config.public.apiUrl}/inventory/${id}`, {
      method: 'DELETE',
      headers: {
        Authorization: `Bearer ${authStore.token}`
      }
    })
    if (res.success) {
      fetchInventory()
    }
  } catch (error: any) {
    alert(error.data?.message || 'Failed to delete inventory item')
  }
}
</script>

<template>
  <div class="space-y-6">
    <!-- Breadcrumbs / Top Header section -->
    <div class="flex flex-col md:flex-row justify-between items-start md:items-center gap-4">
      <div>
        <h1 class="text-2xl font-extrabold tracking-tight text-white">Inventory Stock</h1>
        <p class="text-sm text-slate-400">Track and manage your physical stock levels here.</p>
      </div>

      <button
        v-if="hasPermission('inventory:create')"
        @click="openAddModal"
        class="bg-emerald-600 hover:bg-emerald-500 text-white font-semibold text-sm px-5 py-2.5 rounded-xl transition duration-200 shadow-lg shadow-emerald-600/20 flex items-center gap-2"
      >
        <Icon name="heroicons:plus" class="w-4 h-4" />
        Add Stock Item
      </button>
    </div>

    <!-- Error message banner -->
    <div v-if="errorMsg" class="p-4 bg-rose-500/10 border border-rose-500/30 text-rose-400 rounded-xl flex items-center space-x-2">
      <Icon name="heroicons:exclamation-triangle" class="w-5 h-5 text-rose-400" />
      <span>{{ errorMsg }}</span>
    </div>

    <!-- Stats Cards Row -->
    <div class="grid grid-cols-2 lg:grid-cols-4 gap-4">
      <div class="bg-slate-900/50 border border-slate-800 p-5 rounded-2xl flex flex-col justify-between shadow-xl">
        <span class="text-xs font-bold text-slate-500 uppercase tracking-wider">Total Items</span>
        <span class="text-3xl font-extrabold text-slate-100 mt-2">{{ totalItems }}</span>
      </div>
      <div class="bg-slate-900/50 border border-slate-800 p-5 rounded-2xl flex flex-col justify-between shadow-xl">
        <span class="text-xs font-bold text-slate-500 uppercase tracking-wider">Healthy Stock</span>
        <span class="text-3xl font-extrabold text-emerald-400 mt-2">{{ healthyStockCount }}</span>
      </div>
      <div class="bg-slate-900/50 border border-slate-800 p-5 rounded-2xl flex flex-col justify-between shadow-xl">
        <span class="text-xs font-bold text-slate-500 uppercase tracking-wider">Low Stock Warning</span>
        <span class="text-3xl font-extrabold text-amber-400 mt-2">{{ lowStockCount }}</span>
      </div>
      <div class="bg-slate-900/50 border border-slate-800 p-5 rounded-2xl flex flex-col justify-between shadow-xl">
        <span class="text-xs font-bold text-slate-500 uppercase tracking-wider">Out of Stock</span>
        <span class="text-3xl font-extrabold text-rose-500 mt-2">{{ outOfStockCount }}</span>
      </div>
    </div>

    <!-- Inventory Table -->
    <div v-if="loading" class="flex justify-center items-center py-12">
      <div class="animate-spin rounded-full h-8 w-8 border-b-2 border-emerald-500"></div>
    </div>

    <div v-else class="bg-slate-900/50 border border-slate-800 rounded-2xl overflow-hidden shadow-xl">
      <div class="overflow-x-auto w-full">
        <table class="w-full min-w-[800px] text-left border-collapse">
        <thead>
          <tr class="border-b border-slate-800 bg-slate-900/80 text-xs font-semibold text-slate-400 uppercase tracking-wider">
            <th class="px-6 py-4">Item Details</th>
            <th class="px-6 py-4">Part Number</th>
            <th class="px-6 py-4">Warehouse Location</th>
            <th class="px-6 py-4">Stock Level</th>
            <th v-if="hasPermission('inventory:update') || hasPermission('inventory:delete')" class="px-6 py-4 text-right">Actions</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-slate-800/60 text-sm">
          <tr v-for="i in inventory" :key="i.id" class="hover:bg-slate-900/30 transition-colors">
            <td class="px-6 py-4">
              <div class="font-bold text-slate-200">{{ i.name }}</div>
              <div class="text-xs text-slate-500 mt-0.5 font-medium">Min Stock Target: {{ i.minimum_stock }} {{ i.unit }}</div>
            </td>
            <td class="px-6 py-4 font-mono text-slate-300 text-xs">{{ i.part_number || '-' }}</td>
            <td class="px-6 py-4 text-slate-400">{{ i.location || '-' }}</td>
            <td class="px-6 py-4">
              <div class="flex items-center space-x-2">
                <span :class="{
                  'text-emerald-400 font-bold': i.quantity > i.minimum_stock,
                  'text-amber-400 font-bold animate-pulse': i.quantity <= i.minimum_stock && i.quantity > 0,
                  'text-rose-500 font-extrabold': i.quantity === 0
                }">
                  {{ i.quantity }} {{ i.unit }}
                </span>
                <span v-if="i.quantity === 0" class="text-[10px] uppercase font-bold text-rose-500 bg-rose-500/10 px-2 py-0.5 rounded">Out of Stock</span>
                <span v-else-if="i.quantity <= i.minimum_stock" class="text-[10px] uppercase font-bold text-amber-400 bg-amber-500/10 px-2 py-0.5 rounded">Low Stock</span>
              </div>
            </td>
            <td v-if="hasPermission('inventory:update') || hasPermission('inventory:delete')" class="px-6 py-4 text-right space-x-3 whitespace-nowrap">
              <button v-if="hasPermission('inventory:update')" @click="openEditModal(i)" class="text-emerald-400 hover:text-emerald-300 text-sm font-semibold transition">Edit</button>
              <button v-if="hasPermission('inventory:delete')" @click="deleteItem(i.id)" class="text-rose-400 hover:text-rose-300 text-sm font-semibold transition">Delete</button>
            </td>
          </tr>
          <tr v-if="inventory.length === 0">
            <td colspan="5" class="px-6 py-8 text-center text-slate-500">No inventory items found. Select a different company or register items.</td>
          </tr>
        </tbody>
        </table>
      </div>
    </div>

    <!-- Modal Dialog -->
    <div v-if="showModal" class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm p-4">
      <div class="bg-slate-900 border border-slate-800 rounded-2xl w-full max-w-lg shadow-2xl flex flex-col">
        <div class="px-6 py-4 border-b border-slate-800 flex justify-between items-center">
          <h3 class="text-lg font-bold text-slate-200">{{ editingItem ? 'Edit Stock Level' : 'Add Stock Item' }}</h3>
          <button @click="showModal = false" class="text-slate-400 hover:text-slate-200 text-xl font-bold transition">×</button>
        </div>

        <div class="p-6 space-y-4">
          <!-- Master Item selection -->
          <div>
            <label class="block text-xs font-semibold text-slate-400 uppercase tracking-wider mb-1.5 flex justify-between">
              <span>Link to Master Item</span>
              <NuxtLink to="/master-items" class="text-emerald-400 hover:underline text-[10px] normal-case" @click="showModal = false">
                Manage Master Items →
              </NuxtLink>
            </label>
            <select
              v-model="selectedMasterItemId"
              class="w-full bg-slate-950 border border-slate-800 focus:border-emerald-500 rounded-xl px-4 py-2 text-sm text-slate-200 focus:outline-none transition"
            >
              <option value="">-- Manual Input / Select Master Item --</option>
              <option v-for="m in masterItems" :key="m.id" :value="m.id">
                {{ m.name }} ({{ m.part_number || 'No PN' }})
              </option>
            </select>
          </div>

          <div class="grid grid-cols-2 gap-4">
            <div>
              <label class="block text-xs font-semibold text-slate-400 uppercase tracking-wider mb-1.5">Item Name</label>
              <input
                v-model="form.name"
                type="text"
                placeholder="e.g. Engine Oil 15W-40"
                class="w-full bg-slate-950 border border-slate-800 focus:border-emerald-500 rounded-xl px-4 py-2 text-sm text-slate-200 focus:outline-none transition"
              />
            </div>
            <div>
              <label class="block text-xs font-semibold text-slate-400 uppercase tracking-wider mb-1.5">Part Number</label>
              <input
                v-model="form.part_number"
                type="text"
                placeholder="e.g. PN-OIL-15W40"
                class="w-full bg-slate-950 border border-slate-800 focus:border-emerald-500 rounded-xl px-4 py-2 text-sm text-slate-200 focus:outline-none transition"
              />
            </div>
          </div>

          <div class="grid grid-cols-3 gap-4">
            <div>
              <label class="block text-xs font-semibold text-slate-400 uppercase tracking-wider mb-1.5">Quantity</label>
              <input
                v-model.number="form.quantity"
                type="number"
                placeholder="0"
                class="w-full bg-slate-950 border border-slate-800 focus:border-emerald-500 rounded-xl px-4 py-2 text-sm text-slate-200 focus:outline-none transition"
              />
            </div>
            <div>
              <label class="block text-xs font-semibold text-slate-400 uppercase tracking-wider mb-1.5">Unit</label>
              <input
                v-model="form.unit"
                type="text"
                placeholder="e.g. pcs, Liters"
                class="w-full bg-slate-950 border border-slate-800 focus:border-emerald-500 rounded-xl px-4 py-2 text-sm text-slate-200 focus:outline-none transition"
              />
            </div>
            <div>
              <label class="block text-xs font-semibold text-slate-400 uppercase tracking-wider mb-1.5">Min Stock Limit</label>
              <input
                v-model.number="form.minimum_stock"
                type="number"
                placeholder="5"
                class="w-full bg-slate-950 border border-slate-800 focus:border-emerald-500 rounded-xl px-4 py-2 text-sm text-slate-200 focus:outline-none transition"
              />
            </div>
          </div>

          <div>
            <label class="block text-xs font-semibold text-slate-400 uppercase tracking-wider mb-1.5">Storage Location</label>
            <input
              v-model="form.location"
              type="text"
              placeholder="e.g. Warehouse A - Shelf 2"
              class="w-full bg-slate-950 border border-slate-800 focus:border-emerald-500 rounded-xl px-4 py-2 text-sm text-slate-200 focus:outline-none transition"
            />
          </div>
        </div>

        <div class="px-6 py-4 border-t border-slate-800 flex justify-end space-x-3">
          <button @click="showModal = false" class="bg-slate-800 hover:bg-slate-700 text-slate-300 text-sm font-semibold px-4 py-2 rounded-xl transition">
            Cancel
          </button>
          <button @click="saveItem" class="bg-emerald-600 hover:bg-emerald-500 text-white text-sm font-semibold px-4 py-2 rounded-xl transition shadow-lg shadow-emerald-600/20">
            Save
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
