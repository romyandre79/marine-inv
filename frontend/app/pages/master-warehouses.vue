<script setup lang="ts">
import { useAuthStore } from '~/stores/auth'
import { useTenantStore } from '~/stores/tenant'

const authStore = useAuthStore()
const tenantStore = useTenantStore()
const config = useRuntimeConfig()

// State
const warehouses = ref<any[]>([])
const vessels = ref<any[]>([])
const loading = ref(false)
const errorMsg = ref('')
const search = ref('')

// Computed search list
const filteredWarehouses = computed(() => {
  if (!search.value) return warehouses.value
  const query = search.value.toLowerCase()
  return warehouses.value.filter(w => 
    (w.name && w.name.toLowerCase().includes(query)) ||
    (w.code && w.code.toLowerCase().includes(query)) ||
    (w.address && w.address.toLowerCase().includes(query))
  )
})

const isAuthorized = computed(() => {
  if (!authStore.user) return false
  const role = authStore.user.role
  return role === 'super_admin' || role === 'company_admin' || role === 'admin'
})

onMounted(async () => {
  if (!authStore.isAuthenticated) {
    window.location.href = `${config.public.portalUrl}/login`
  } else if (!isAuthorized.value) {
    navigateTo('/inventory')
  } else {
    await tenantStore.fetchCompanies()
    fetchWarehouses()
    fetchVessels()
  }
})

// Watch active tenant to re-fetch
watch(() => tenantStore.activeTenantId, () => {
  fetchWarehouses()
})

// CRUD State
const showModal = ref(false)
const editingItem = ref<any>(null)
const form = ref({
  name: '',
  code: '',
  address: '',
  vessel_id: '',
  company_id: ''
})

async function fetchWarehouses() {
  loading.value = true
  errorMsg.value = ''
  try {
    const companyQuery = tenantStore.activeTenantId ? `?company_id=${tenantStore.activeTenantId}` : ''
    const res = await $fetch<any>(`${config.public.apiUrl}/master-warehouses${companyQuery}`, {
      headers: {
        Authorization: `Bearer ${authStore.token}`
      }
    })
    if (res.success) {
      warehouses.value = res.data
    }
  } catch (error: any) {
    errorMsg.value = error.data?.message || 'Failed to fetch master warehouses.'
  } finally {
    loading.value = false
  }
}

async function fetchVessels() {
  try {
    // Fetch from FMS Backend via configured runtime config
    const res = await $fetch<any>(`${config.public.fmsApiUrl}/vessels`, {
      headers: {
        Authorization: `Bearer ${authStore.token}`
      }
    })
    if (res.success && Array.isArray(res.data)) {
      vessels.value = res.data
    }
  } catch (error) {
    console.error('Failed to load FMS vessel list:', error)
  }
}

function getVesselName(vesselId: string) {
  if (!vesselId) return '-'
  const match = vessels.value.find(v => v.id === vesselId)
  return match ? match.name : 'Unknown Vessel'
}

function openAddModal() {
  editingItem.value = null
  form.value = {
    name: '',
    code: '',
    address: '',
    vessel_id: '',
    company_id: tenantStore.activeTenantId || ''
  }
  showModal.value = true
}

function openEditModal(item: any) {
  editingItem.value = item
  form.value = {
    name: item.name,
    code: item.code || '',
    address: item.address || '',
    vessel_id: item.vessel_id || '',
    company_id: item.company_id || tenantStore.activeTenantId || ''
  }
  showModal.value = true
}

async function saveItem() {
  if (!form.value.name) {
    alert('Please enter a warehouse name.')
    return
  }
  try {
    let url = `${config.public.apiUrl}/master-warehouses`
    let method = 'POST'
    if (editingItem.value) {
      url = `${config.public.apiUrl}/master-warehouses/${editingItem.value.id}`
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
        vessel_id: form.value.vessel_id || null,
        company_id: form.value.company_id || null
      }
    })

    if (res.success) {
      showModal.value = false
      fetchWarehouses()
    }
  } catch (error: any) {
    alert(error.data?.message || 'Failed to save master warehouse')
  }
}

async function deleteItem(id: string) {
  if (!confirm('Are you sure you want to delete this warehouse location definition?')) return
  try {
    const res = await $fetch<any>(`${config.public.apiUrl}/master-warehouses/${id}`, {
      method: 'DELETE',
      headers: {
        Authorization: `Bearer ${authStore.token}`
      }
    })
    if (res.success) {
      fetchWarehouses()
    }
  } catch (error: any) {
    alert(error.data?.message || 'Failed to delete warehouse')
  }
}
</script>

<template>
  <div class="space-y-6">
    <div class="flex flex-col md:flex-row justify-between items-start md:items-center gap-4">
      <div>
        <h1 class="text-2xl font-extrabold tracking-tight text-white">Master Warehouse Locations</h1>
        <p class="text-sm text-slate-400">Define storage sites and link warehouses directly to FMS vessels/ships.</p>
      </div>

      <button
        @click="openAddModal"
        class="bg-emerald-600 hover:bg-emerald-500 text-white font-semibold text-sm px-5 py-2.5 rounded-xl transition duration-200 shadow-lg shadow-emerald-600/20 flex items-center gap-2"
      >
        <Icon name="heroicons:plus" class="w-4 h-4" />
        New Warehouse
      </button>
    </div>

    <!-- Toolbar: Search -->
    <div class="bg-slate-900/40 border border-slate-800 p-4 rounded-2xl flex flex-col md:flex-row items-center gap-4">
      <div class="relative w-full md:max-w-md">
        <Icon name="heroicons:magnifying-glass" class="absolute left-3.5 top-3 w-4 h-4 text-slate-500" />
        <input
          v-model="search"
          type="text"
          placeholder="Search warehouses by name, code or address..."
          class="w-full bg-slate-950 border border-slate-800 focus:border-emerald-500 rounded-xl pl-10 pr-4 py-2 text-sm text-slate-200 focus:outline-none transition"
        />
      </div>
      <div class="text-xs text-slate-500 font-medium">
        Showing {{ filteredWarehouses.length }} of {{ warehouses.length }} registered warehouses
      </div>
    </div>

    <!-- Error message banner -->
    <div v-if="errorMsg" class="p-4 bg-rose-500/10 border border-rose-500/30 text-rose-400 rounded-xl flex items-center space-x-2">
      <Icon name="heroicons:exclamation-triangle" class="w-5 h-5 text-rose-400" />
      <span>{{ errorMsg }}</span>
    </div>

    <!-- Warehouses Table -->
    <div v-if="loading" class="flex justify-center items-center py-12">
      <div class="animate-spin rounded-full h-8 w-8 border-b-2 border-emerald-500"></div>
    </div>

    <div v-else class="bg-slate-900/50 border border-slate-800 rounded-2xl overflow-hidden shadow-xl">
      <div class="overflow-x-auto w-full">
        <table class="w-full min-w-[800px] text-left border-collapse">
        <thead>
          <tr class="border-b border-slate-800 bg-slate-900/80 text-xs font-semibold text-slate-400 uppercase tracking-wider">
            <th class="px-6 py-4">Warehouse Name</th>
            <th class="px-6 py-4">Code</th>
            <th class="px-6 py-4">Vessel (Kapal FMS)</th>
            <th class="px-6 py-4">Address / Details</th>
            <th class="px-6 py-4 text-right">Actions</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-slate-800/60 text-sm">
          <tr v-for="w in filteredWarehouses" :key="w.id" class="hover:bg-slate-900/30 transition-colors">
            <td class="px-6 py-4 font-bold text-slate-200">{{ w.name }}</td>
            <td class="px-6 py-4 font-mono text-slate-300 text-xs">{{ w.code || '-' }}</td>
            <td class="px-6 py-4 text-emerald-400 font-medium">
              <span class="flex items-center gap-1.5">
                <Icon name="heroicons:ship-wheel" class="w-4 h-4 text-emerald-500" />
                {{ getVesselName(w.vessel_id) }}
              </span>
            </td>
            <td class="px-6 py-4 text-slate-400 max-w-xs truncate" :title="w.address">
              {{ w.address || '-' }}
            </td>
            <td class="px-6 py-4 text-right space-x-3 whitespace-nowrap">
              <button @click="openEditModal(w)" class="text-emerald-400 hover:text-emerald-300 text-sm font-semibold transition">Edit</button>
              <button @click="deleteItem(w.id)" class="text-rose-400 hover:text-rose-300 text-sm font-semibold transition">Delete</button>
            </td>
          </tr>
          <tr v-if="filteredWarehouses.length === 0">
            <td colspan="5" class="px-6 py-8 text-center text-slate-500">No warehouses registered for this tenant.</td>
          </tr>
        </tbody>
        </table>
      </div>
    </div>

    <!-- Modal Dialog -->
    <div v-if="showModal" class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm p-4">
      <div class="bg-slate-900 border border-slate-800 rounded-2xl w-full max-w-lg shadow-2xl flex flex-col">
        <div class="px-6 py-4 border-b border-slate-800 flex justify-between items-center">
          <h3 class="text-lg font-bold text-slate-200">{{ editingItem ? 'Edit Warehouse' : 'Add Warehouse' }}</h3>
          <button @click="showModal = false" class="text-slate-400 hover:text-slate-200 text-xl font-bold transition">×</button>
        </div>

        <div class="p-6 space-y-4">
          <div class="grid grid-cols-2 gap-4">
            <div>
              <label class="block text-xs font-semibold text-slate-400 uppercase tracking-wider mb-1.5">Warehouse Name</label>
              <input
                v-model="form.name"
                type="text"
                placeholder="e.g. Main Engine Spare Room"
                class="w-full bg-slate-950 border border-slate-800 focus:border-emerald-500 rounded-xl px-4 py-2 text-sm text-slate-200 focus:outline-none transition"
              />
            </div>
            <div>
              <label class="block text-xs font-semibold text-slate-400 uppercase tracking-wider mb-1.5">Code</label>
              <input
                v-model="form.code"
                type="text"
                placeholder="e.g. WH-ENG-01"
                class="w-full bg-slate-950 border border-slate-800 focus:border-emerald-500 rounded-xl px-4 py-2 text-sm text-slate-200 focus:outline-none transition"
              />
            </div>
          </div>

          <div>
            <label class="block text-xs font-semibold text-slate-400 uppercase tracking-wider mb-1.5">Vessel (Kapal FMS)</label>
            <select
              v-model="form.vessel_id"
              class="w-full bg-slate-950 border border-slate-800 focus:border-emerald-500 rounded-xl px-4 py-2 text-sm text-slate-200 focus:outline-none transition"
            >
              <option value="">-- No Ship Associated (Shore Warehouse) --</option>
              <option v-for="v in vessels" :key="v.id" :value="v.id">
                {{ v.name }} [{{ v.type }}]
              </option>
            </select>
          </div>

          <div>
            <label class="block text-xs font-semibold text-slate-400 uppercase tracking-wider mb-1.5">Address / Description</label>
            <textarea
              v-model="form.address"
              rows="3"
              placeholder="Enter specific deck details or shore address..."
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
