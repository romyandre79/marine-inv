<script setup lang="ts">
import { useAuthStore } from '~/stores/auth'
import { useTenantStore } from '~/stores/tenant'

const authStore = useAuthStore()
const tenantStore = useTenantStore()
const config = useRuntimeConfig()

// State
const masterItems = ref<any[]>([])
const loading = ref(false)
const errorMsg = ref('')
const search = ref('')

// Compute filtered master items locally by search query
const filteredMasterItems = computed(() => {
  if (!search.value) return masterItems.value
  const query = search.value.toLowerCase()
  return masterItems.value.filter(item => 
    (item.name && item.name.toLowerCase().includes(query)) ||
    (item.part_number && item.part_number.toLowerCase().includes(query)) ||
    (item.description && item.description.toLowerCase().includes(query))
  )
})

const isAuthorized = computed(() => {
  if (!authStore.user) return false
  const role = authStore.user.role
  return role === 'super_admin' || role === 'company_admin' || role === 'admin'
})

// SSO Check: Redirect if not authenticated, redirect if not authorized
onMounted(async () => {
  if (!authStore.isAuthenticated) {
    window.location.href = `${config.public.portalUrl}/login`
  } else if (!isAuthorized.value) {
    // If not admin, redirect to stock view
    navigateTo('/inventory')
  } else {
    await tenantStore.fetchCompanies()
    fetchMasterItems()
  }
})

// Watch active tenant to re-fetch
watch(() => tenantStore.activeTenantId, () => {
  fetchMasterItems()
})

// CRUD State
const showModal = ref(false)
const editingItem = ref<any>(null)
const form = ref({
  name: '',
  part_number: '',
  unit: 'pcs',
  description: '',
  company_id: ''
})

async function fetchMasterItems() {
  loading.value = true
  errorMsg.value = ''
  try {
    const companyQuery = tenantStore.activeTenantId ? `?company_id=${tenantStore.activeTenantId}` : ''
    const res = await $fetch<any>(`${config.public.apiUrl}/master-items${companyQuery}`, {
      headers: {
        Authorization: `Bearer ${authStore.token}`
      }
    })
    if (res.success) {
      masterItems.value = res.data
    }
  } catch (error: any) {
    errorMsg.value = error.data?.message || 'Failed to fetch master items definitions.'
    if (error.status === 401) {
      authStore.logout()
    }
  } finally {
    loading.value = false
  }
}

function openAddModal() {
  editingItem.value = null
  form.value = {
    name: '',
    part_number: '',
    unit: 'pcs',
    description: '',
    company_id: tenantStore.activeTenantId || ''
  }
  showModal.value = true
}

function openEditModal(item: any) {
  editingItem.value = item
  form.value = {
    name: item.name,
    part_number: item.part_number || '',
    unit: item.unit || 'pcs',
    description: item.description || '',
    company_id: item.company_id || tenantStore.activeTenantId || ''
  }
  showModal.value = true
}

async function saveItem() {
  if (!form.value.name) {
    alert('Please enter an item name.')
    return
  }
  try {
    let url = `${config.public.apiUrl}/master-items`
    let method = 'POST'
    if (editingItem.value) {
      url = `${config.public.apiUrl}/master-items/${editingItem.value.id}`
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
      fetchMasterItems()
    }
  } catch (error: any) {
    alert(error.data?.message || 'Failed to save master item')
  }
}

async function deleteItem(id: string) {
  if (!confirm('Are you sure you want to delete this master item definition? Physical stock items with this name will remain, but the definition template will be removed.')) return
  try {
    const res = await $fetch<any>(`${config.public.apiUrl}/master-items/${id}`, {
      method: 'DELETE',
      headers: {
        Authorization: `Bearer ${authStore.token}`
      }
    })
    if (res.success) {
      fetchMasterItems()
    }
  } catch (error: any) {
    alert(error.data?.message || 'Failed to delete master item')
  }
}
</script>

<template>
  <div class="space-y-6">
    <!-- Breadcrumbs / Top Header section -->
    <div class="flex flex-col md:flex-row justify-between items-start md:items-center gap-4">
      <div>
        <h1 class="text-2xl font-extrabold tracking-tight text-white">Master Item Registry</h1>
        <p class="text-sm text-slate-400">Manage definitions and part specifications for your company inventory.</p>
      </div>

      <button
        @click="openAddModal"
        class="bg-emerald-600 hover:bg-emerald-500 text-white font-semibold text-sm px-5 py-2.5 rounded-xl transition duration-200 shadow-lg shadow-emerald-600/20 flex items-center gap-2"
      >
        <Icon name="heroicons:plus" class="w-4 h-4" />
        New Item Definition
      </button>
    </div>

    <!-- Toolbar: Search -->
    <div class="bg-slate-900/40 border border-slate-800 p-4 rounded-2xl flex flex-col md:flex-row items-center gap-4">
      <div class="relative w-full md:max-w-md">
        <Icon name="heroicons:magnifying-glass" class="absolute left-3.5 top-3 w-4 h-4 text-slate-500" />
        <input
          v-model="search"
          type="text"
          placeholder="Search items by name, part number or description..."
          class="w-full bg-slate-950 border border-slate-800 focus:border-emerald-500 rounded-xl pl-10 pr-4 py-2 text-sm text-slate-200 focus:outline-none transition"
        />
      </div>
      <div class="text-xs text-slate-500 font-medium">
        Showing {{ filteredMasterItems.length }} of {{ masterItems.length }} registered items
      </div>
    </div>

    <!-- Error message banner -->
    <div v-if="errorMsg" class="p-4 bg-rose-500/10 border border-rose-500/30 text-rose-400 rounded-xl flex items-center space-x-2">
      <Icon name="heroicons:exclamation-triangle" class="w-5 h-5 text-rose-400" />
      <span>{{ errorMsg }}</span>
    </div>

    <!-- Items Table -->
    <div v-if="loading" class="flex justify-center items-center py-12">
      <div class="animate-spin rounded-full h-8 w-8 border-b-2 border-emerald-500"></div>
    </div>

    <div v-else class="bg-slate-900/50 border border-slate-800 rounded-2xl overflow-hidden shadow-xl">
      <div class="overflow-x-auto w-full">
        <table class="w-full min-w-[800px] text-left border-collapse">
        <thead>
          <tr class="border-b border-slate-800 bg-slate-900/80 text-xs font-semibold text-slate-400 uppercase tracking-wider">
            <th class="px-6 py-4">Item Name</th>
            <th class="px-6 py-4">Part Number</th>
            <th class="px-6 py-4">Default Unit</th>
            <th class="px-6 py-4">Description</th>
            <th class="px-6 py-4 text-right">Actions</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-slate-800/60 text-sm">
          <tr v-for="item in filteredMasterItems" :key="item.id" class="hover:bg-slate-900/30 transition-colors">
            <td class="px-6 py-4">
              <div class="font-bold text-slate-200">{{ item.name }}</div>
            </td>
            <td class="px-6 py-4 font-mono text-slate-300 text-xs">{{ item.part_number || '-' }}</td>
            <td class="px-6 py-4 text-slate-400 font-medium">{{ item.unit }}</td>
            <td class="px-6 py-4 text-slate-400 max-w-xs truncate" :title="item.description">
              {{ item.description || '-' }}
            </td>
            <td class="px-6 py-4 text-right space-x-3 whitespace-nowrap">
              <button @click="openEditModal(item)" class="text-emerald-400 hover:text-emerald-300 text-sm font-semibold transition">Edit</button>
              <button @click="deleteItem(item.id)" class="text-rose-400 hover:text-rose-300 text-sm font-semibold transition">Delete</button>
            </td>
          </tr>
          <tr v-if="filteredMasterItems.length === 0">
            <td colspan="5" class="px-6 py-8 text-center text-slate-500">No master items found. Select a different company or register a new definition.</td>
          </tr>
        </tbody>
        </table>
      </div>
    </div>

    <!-- Modal Dialog -->
    <div v-if="showModal" class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm p-4">
      <div class="bg-slate-900 border border-slate-800 rounded-2xl w-full max-w-lg shadow-2xl flex flex-col">
        <div class="px-6 py-4 border-b border-slate-800 flex justify-between items-center">
          <h3 class="text-lg font-bold text-slate-200">{{ editingItem ? 'Edit Item Definition' : 'Add Item Definition' }}</h3>
          <button @click="showModal = false" class="text-slate-400 hover:text-slate-200 text-xl font-bold transition">×</button>
        </div>

        <div class="p-6 space-y-4">
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

          <div>
            <label class="block text-xs font-semibold text-slate-400 uppercase tracking-wider mb-1.5">Default Unit</label>
            <input
              v-model="form.unit"
              type="text"
              placeholder="e.g. pcs, Liters, drums"
              class="w-full bg-slate-950 border border-slate-800 focus:border-emerald-500 rounded-xl px-4 py-2 text-sm text-slate-200 focus:outline-none transition"
            />
          </div>

          <div>
            <label class="block text-xs font-semibold text-slate-400 uppercase tracking-wider mb-1.5">Description / Specs</label>
            <textarea
              v-model="form.description"
              rows="3"
              placeholder="Enter dimensions, specifications or general comments..."
              class="w-full bg-slate-950 border border-slate-800 focus:border-emerald-500 rounded-xl px-4 py-2 text-sm text-slate-200 focus:outline-none transition resize-none"
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
