<script setup lang="ts">
import { useAuthStore } from '~/stores/auth'
import { useTenantStore } from '~/stores/tenant'

const authStore = useAuthStore()
const tenantStore = useTenantStore()
const config = useRuntimeConfig()

// State
const transfers = ref<any[]>([])
const warehouses = ref<any[]>([])
const inventory = ref<any[]>([])
const loading = ref(false)
const errorMsg = ref('')
const search = ref('')

// Form State
const showRequestModal = ref(false)
const showRejectModal = ref(false)
const selectedTransferToReject = ref<any>(null)
const rejectComment = ref('')

const form = ref({
  source_warehouse: '',
  target_warehouse: '',
  inventory_item_id: '',
  quantity: 1,
  company_id: ''
})

// Current user details
const userEmail = computed(() => authStore.user?.email || '')
const userRole = computed(() => authStore.user?.role || 'viewer')
const isAdmin = computed(() => ['super_admin', 'company_admin', 'admin'].includes(userRole.value))

// Load data on mount
onMounted(async () => {
  if (!authStore.isAuthenticated) {
    window.location.href = `${config.public.portalUrl}/login`
  } else {
    await tenantStore.fetchCompanies()
    fetchTransfers()
    fetchWarehouses()
    fetchInventory()
  }
})

// Watch tenant change
watch(() => tenantStore.activeTenantId, () => {
  fetchTransfers()
  fetchWarehouses()
  fetchInventory()
})

async function fetchTransfers() {
  loading.value = true
  errorMsg.value = ''
  try {
    const companyQuery = tenantStore.activeTenantId ? `?company_id=${tenantStore.activeTenantId}` : ''
    const res = await $fetch<any>(`${config.public.apiUrl}/stock-transfers${companyQuery}`, {
      headers: { Authorization: `Bearer ${authStore.token}` }
    })
    if (res.success) {
      transfers.value = res.data
    }
  } catch (error: any) {
    errorMsg.value = error.data?.message || 'Failed to fetch stock transfers.'
  } finally {
    loading.value = false
  }
}

async function fetchWarehouses() {
  try {
    const companyQuery = tenantStore.activeTenantId ? `?company_id=${tenantStore.activeTenantId}` : ''
    const res = await $fetch<any>(`${config.public.apiUrl}/master-warehouses${companyQuery}`, {
      headers: { Authorization: `Bearer ${authStore.token}` }
    })
    if (res.success) {
      warehouses.value = res.data
    }
  } catch (error) {
    console.error('Failed to fetch warehouses', error)
  }
}

async function fetchInventory() {
  try {
    const companyQuery = tenantStore.activeTenantId ? `?company_id=${tenantStore.activeTenantId}` : ''
    const res = await $fetch<any>(`${config.public.apiUrl}/inventory${companyQuery}`, {
      headers: { Authorization: `Bearer ${authStore.token}` }
    })
    if (res.success) {
      inventory.value = res.data
    }
  } catch (error) {
    console.error('Failed to fetch inventory', error)
  }
}

// Items available in the selected source warehouse
const sourceItems = computed(() => {
  if (!form.value.source_warehouse) return []
  return inventory.value.filter(item => item.location === form.value.source_warehouse && item.quantity > 0)
})

// The currently selected item object to transfer
const selectedSourceItem = computed(() => {
  return sourceItems.value.find(item => item.id === form.value.inventory_item_id)
})

// Maximum available quantity for transfer
const maxAvailableQuantity = computed(() => {
  return selectedSourceItem.value ? selectedSourceItem.value.quantity : 0
})

// Filtered transfers based on search query
const filteredTransfers = computed(() => {
  if (!search.value) return transfers.value
  const query = search.value.toLowerCase()
  return transfers.value.filter(t => 
    t.item_name.toLowerCase().includes(query) ||
    t.source_warehouse.toLowerCase().includes(query) ||
    t.target_warehouse.toLowerCase().includes(query) ||
    t.requested_by.toLowerCase().includes(query)
  )
})

function openRequestModal() {
  form.value = {
    source_warehouse: '',
    target_warehouse: '',
    inventory_item_id: '',
    quantity: 1,
    company_id: tenantStore.activeTenantId || ''
  }
  showRequestModal.value = true
}

async function submitTransferRequest() {
  if (!form.value.source_warehouse || !form.value.target_warehouse || !form.value.inventory_item_id) {
    alert('Please fill out all required fields.')
    return
  }

  if (form.value.source_warehouse === form.value.target_warehouse) {
    alert('Source and Target warehouses cannot be the same.')
    return
  }

  if (form.value.quantity <= 0 || form.value.quantity > maxAvailableQuantity.value) {
    alert(`Quantity must be between 1 and ${maxAvailableQuantity.value}`)
    return
  }

  const item = selectedSourceItem.value
  if (!item) return

  try {
    const res = await $fetch<any>(`${config.public.apiUrl}/stock-transfers`, {
      method: 'POST',
      headers: {
        Authorization: `Bearer ${authStore.token}`,
        'Content-Type': 'application/json'
      },
      body: {
        source_warehouse: form.value.source_warehouse,
        target_warehouse: form.value.target_warehouse,
        item_name: item.name,
        part_number: item.part_number,
        quantity: form.value.quantity,
        unit: item.unit || 'pcs',
        company_id: form.value.company_id || null
      }
    })

    if (res.success) {
      showRequestModal.value = false
      fetchTransfers()
    }
  } catch (error: any) {
    alert(error.data?.message || 'Failed to submit transfer request.')
  }
}

async function approveTransfer(transfer: any) {
  if (!confirm(`Are you sure you want to approve this transfer of ${transfer.quantity} ${transfer.unit} of "${transfer.item_name}"?`)) return
  try {
    const res = await $fetch<any>(`${config.public.apiUrl}/stock-transfers/${transfer.id}/approve`, {
      method: 'POST',
      headers: { Authorization: `Bearer ${authStore.token}` }
    })
    if (res.success) {
      fetchTransfers();
      fetchInventory();
    }
  } catch (error: any) {
    alert(error.data?.message || 'Failed to approve transfer.')
  }
}

function openRejectModal(transfer: any) {
  selectedTransferToReject.value = transfer
  rejectComment.value = ''
  showRejectModal.value = true
}

async function submitRejection() {
  if (!rejectComment.value.trim()) {
    alert('Comment/Reason is required for rejection.')
    return
  }
  try {
    const res = await $fetch<any>(`${config.public.apiUrl}/stock-transfers/${selectedTransferToReject.value.id}/reject`, {
      method: 'POST',
      headers: {
        Authorization: `Bearer ${authStore.token}`,
        'Content-Type': 'application/json'
      },
      body: { comments: rejectComment.value }
    })
    if (res.success) {
      showRejectModal.value = false
      fetchTransfers()
    }
  } catch (error: any) {
    alert(error.data?.message || 'Failed to reject transfer.')
  }
}

// Check if current user can approve/reject a given transfer
function canAction(transfer: any): boolean {
  if (transfer.status !== 'pending') return false

  const isReqAdmin = ['super_admin', 'company_admin', 'admin'].includes(transfer.requested_role)
  if (isReqAdmin) {
    // Requested by Admin, so only Operator (non-admin) can approve/reject
    return !isAdmin.value
  } else {
    // Requested by Operator, so only Admin can approve/reject
    return isAdmin.value
  }
}
</script>

<template>
  <div class="space-y-6 text-slate-100">
    <!-- Header -->
    <div class="flex flex-col md:flex-row justify-between items-start md:items-center gap-4">
      <div>
        <h1 class="text-2xl font-extrabold tracking-tight text-white">Stock Transfer</h1>
        <p class="text-sm text-slate-400">Request and approve transfer of goods between warehouses.</p>
      </div>

      <button
        @click="openRequestModal"
        class="bg-emerald-600 hover:bg-emerald-500 text-white font-semibold text-sm px-5 py-2.5 rounded-xl transition duration-200 shadow-lg shadow-emerald-600/20 flex items-center gap-2"
      >
        <Icon name="heroicons:plus" class="w-4 h-4" />
        New Transfer Request
      </button>
    </div>

    <!-- Toolbar: Search -->
    <div class="bg-slate-900/40 border border-slate-800 p-4 rounded-2xl flex flex-col md:flex-row items-center justify-between gap-4">
      <div class="relative w-full md:max-w-md">
        <Icon name="heroicons:magnifying-glass" class="absolute left-3.5 top-3 w-4 h-4 text-slate-500" />
        <input
          v-model="search"
          type="text"
          placeholder="Search by item, source, or requester..."
          class="w-full bg-slate-950 border border-slate-800 focus:border-emerald-500 rounded-xl pl-10 pr-4 py-2 text-sm text-slate-200 focus:outline-none transition"
        />
      </div>
      <div class="text-xs text-slate-500 font-semibold bg-slate-950/60 border border-slate-800 px-3 py-1.5 rounded-lg flex items-center gap-1.5">
        <span class="w-2.5 h-2.5 rounded-full" :class="isAdmin ? 'bg-blue-500' : 'bg-emerald-500'"></span>
        Your Role: <span class="capitalize text-slate-300 font-bold">{{ userRole }}</span>
      </div>
    </div>

    <!-- Error message banner -->
    <div v-if="errorMsg" class="p-4 bg-rose-500/10 border border-rose-500/30 text-rose-400 rounded-xl flex items-center space-x-2">
      <Icon name="heroicons:exclamation-triangle" class="w-5 h-5 text-rose-400" />
      <span>{{ errorMsg }}</span>
    </div>

    <!-- List table -->
    <div v-if="loading" class="flex justify-center items-center py-12">
      <div class="animate-spin rounded-full h-8 w-8 border-b-2 border-emerald-500"></div>
    </div>

    <div v-else class="bg-slate-900/50 border border-slate-800 rounded-2xl overflow-hidden shadow-xl">
      <div class="overflow-x-auto w-full">
        <table class="w-full min-w-[950px] text-left border-collapse">
          <thead>
            <tr class="border-b border-slate-800 bg-slate-900/80 text-xs font-semibold text-slate-400 uppercase tracking-wider">
              <th class="px-6 py-4">Item Details</th>
              <th class="px-6 py-4">Route</th>
              <th class="px-6 py-4">Qty</th>
              <th class="px-6 py-4">Requester</th>
              <th class="px-6 py-4">Status</th>
              <th class="px-6 py-4">Verification / Comments</th>
              <th class="px-6 py-4 text-right">Actions</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-slate-800/60 text-sm">
            <tr v-for="t in filteredTransfers" :key="t.id" class="hover:bg-slate-900/30 transition-colors">
              <td class="px-6 py-4">
                <div class="font-bold text-slate-200">{{ t.item_name }}</div>
                <div class="text-xs text-slate-500 mt-0.5 font-medium">Part No: {{ t.part_number || '-' }}</div>
              </td>
              <td class="px-6 py-4">
                <div class="flex items-center gap-2 text-slate-300">
                  <span class="bg-slate-950 px-2.5 py-1 rounded-md text-xs font-semibold border border-slate-800 text-slate-400">{{ t.source_warehouse }}</span>
                  <Icon name="heroicons:arrow-long-right" class="w-4 h-4 text-slate-600" />
                  <span class="bg-slate-950 px-2.5 py-1 rounded-md text-xs font-semibold border border-slate-800 text-slate-200">{{ t.target_warehouse }}</span>
                </div>
              </td>
              <td class="px-6 py-4 font-mono font-bold text-emerald-400">
                {{ t.quantity }} <span class="text-slate-500 font-medium text-xs">{{ t.unit }}</span>
              </td>
              <td class="px-6 py-4">
                <div class="text-slate-300 font-medium text-xs max-w-[150px] truncate" :title="t.requested_by">{{ t.requested_by }}</div>
                <div class="text-[10px] text-slate-500 font-bold uppercase tracking-wider capitalize">{{ t.requested_role }}</div>
              </td>
              <td class="px-6 py-4">
                <span v-if="t.status === 'pending'" class="px-2.5 py-1 text-[10px] uppercase font-bold text-amber-400 bg-amber-400/10 rounded-full border border-amber-400/20">Pending</span>
                <span v-else-if="t.status === 'approved'" class="px-2.5 py-1 text-[10px] uppercase font-bold text-emerald-400 bg-emerald-400/10 rounded-full border border-emerald-400/20">Approved</span>
                <span v-else class="px-2.5 py-1 text-[10px] uppercase font-bold text-rose-500 bg-rose-500/10 rounded-full border border-rose-500/20">Rejected</span>
              </td>
              <td class="px-6 py-4 text-xs max-w-[250px]">
                <div v-if="t.approved_rejected_by" class="text-slate-400 font-medium">
                  Verified by: <span class="text-slate-300" :title="t.approved_rejected_by">{{ t.approved_rejected_by }}</span>
                </div>
                <div v-if="t.comments" class="text-rose-400/80 italic mt-0.5 mt-1 border-l-2 border-rose-500/30 pl-2">
                  "{{ t.comments }}"
                </div>
                <div v-else-if="t.status === 'pending'" class="text-slate-500 italic">Waiting verification</div>
              </td>
              <td class="px-6 py-4 text-right space-x-2 whitespace-nowrap">
                <template v-if="canAction(t)">
                  <button
                    @click="approveTransfer(t)"
                    class="bg-emerald-600/20 hover:bg-emerald-600 text-emerald-400 hover:text-white border border-emerald-500/30 px-3 py-1.5 rounded-lg text-xs font-bold transition"
                  >
                    Approve
                  </button>
                  <button
                    @click="openRejectModal(t)"
                    class="bg-rose-500/20 hover:bg-rose-500 text-rose-400 hover:text-white border border-rose-500/30 px-3 py-1.5 rounded-lg text-xs font-bold transition"
                  >
                    Reject
                  </button>
                </template>
                <span v-else-if="t.status === 'pending'" class="text-[10px] font-bold text-slate-600 italic">
                  {{ isAdmin ? 'Operator Action Required' : 'Admin Action Required' }}
                </span>
                <span v-else class="text-[10px] font-bold text-slate-600">Processed</span>
              </td>
            </tr>
            <tr v-if="filteredTransfers.length === 0">
              <td colspan="7" class="px-6 py-10 text-center text-slate-500">No stock transfers found.</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- Request Modal -->
    <div v-if="showRequestModal" class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm p-4">
      <div class="bg-slate-900 border border-slate-800 rounded-2xl w-full max-w-lg shadow-2xl flex flex-col">
        <div class="px-6 py-4 border-b border-slate-800 flex justify-between items-center">
          <h3 class="text-lg font-bold text-slate-200">Request Stock Transfer</h3>
          <button @click="showRequestModal = false" class="text-slate-400 hover:text-slate-200 text-xl font-bold transition">×</button>
        </div>

        <div class="p-6 space-y-4">
          <!-- Source warehouse selector -->
          <div>
            <label class="block text-xs font-semibold text-slate-400 uppercase tracking-wider mb-1.5">Source Warehouse</label>
            <select
              v-model="form.source_warehouse"
              class="w-full bg-slate-950 border border-slate-800 focus:border-emerald-500 rounded-xl px-4 py-2 text-sm text-slate-200 focus:outline-none transition"
            >
              <option value="">-- Select Source Warehouse --</option>
              <option v-for="w in warehouses" :key="w.id" :value="w.name">
                {{ w.name }} ({{ w.code || 'No Code' }})
              </option>
            </select>
          </div>

          <!-- Target warehouse selector -->
          <div>
            <label class="block text-xs font-semibold text-slate-400 uppercase tracking-wider mb-1.5">Target Warehouse</label>
            <select
              v-model="form.target_warehouse"
              class="w-full bg-slate-950 border border-slate-800 focus:border-emerald-500 rounded-xl px-4 py-2 text-sm text-slate-200 focus:outline-none transition"
            >
              <option value="">-- Select Target Warehouse --</option>
              <option v-for="w in warehouses" :key="w.id" :value="w.name">
                {{ w.name }} ({{ w.code || 'No Code' }})
              </option>
            </select>
          </div>

          <!-- Item Selection -->
          <div v-if="form.source_warehouse">
            <label class="block text-xs font-semibold text-slate-400 uppercase tracking-wider mb-1.5">Item to Transfer</label>
            <select
              v-model="form.inventory_item_id"
              class="w-full bg-slate-950 border border-slate-800 focus:border-emerald-500 rounded-xl px-4 py-2 text-sm text-slate-200 focus:outline-none transition"
            >
              <option value="">-- Select Item from Source Warehouse --</option>
              <option v-for="item in sourceItems" :key="item.id" :value="item.id">
                {{ item.name }} (Available: {{ item.quantity }} {{ item.unit }})
              </option>
            </select>
          </div>

          <!-- Quantity to transfer -->
          <div v-if="form.inventory_item_id">
            <label class="block text-xs font-semibold text-slate-400 uppercase tracking-wider mb-1.5 flex justify-between">
              <span>Quantity to Transfer</span>
              <span class="text-[10px] text-emerald-400 font-bold normal-case">Available: {{ maxAvailableQuantity }} {{ selectedSourceItem?.unit }}</span>
            </label>
            <input
              v-model.number="form.quantity"
              type="number"
              min="1"
              :max="maxAvailableQuantity"
              class="w-full bg-slate-950 border border-slate-800 focus:border-emerald-500 rounded-xl px-4 py-2 text-sm text-slate-200 focus:outline-none transition"
            />
          </div>
        </div>

        <div class="px-6 py-4 border-t border-slate-800 flex justify-end space-x-3">
          <button @click="showRequestModal = false" class="bg-slate-800 hover:bg-slate-700 text-slate-300 text-sm font-semibold px-4 py-2 rounded-xl transition">
            Cancel
          </button>
          <button @click="submitTransferRequest" class="bg-emerald-600 hover:bg-emerald-500 text-white text-sm font-semibold px-4 py-2 rounded-xl transition shadow-lg shadow-emerald-600/20">
            Submit Request
          </button>
        </div>
      </div>
    </div>

    <!-- Reject Comment Modal -->
    <div v-if="showRejectModal" class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm p-4">
      <div class="bg-slate-900 border border-slate-800 rounded-2xl w-full max-w-lg shadow-2xl flex flex-col">
        <div class="px-6 py-4 border-b border-slate-800 flex justify-between items-center">
          <h3 class="text-lg font-bold text-slate-200">Reject Transfer Request</h3>
          <button @click="showRejectModal = false" class="text-slate-400 hover:text-slate-200 text-xl font-bold transition">×</button>
        </div>

        <div class="p-6 space-y-4">
          <div>
            <label class="block text-xs font-semibold text-rose-400 uppercase tracking-wider mb-1.5">Reason / Comments for Rejection (Required)</label>
            <textarea
              v-model="rejectComment"
              rows="3"
              placeholder="Provide a clear comment or reason why this request is being rejected..."
              class="w-full bg-slate-950 border border-slate-800 focus:border-rose-500 rounded-xl px-4 py-2 text-sm text-slate-200 focus:outline-none transition resize-none"
            />
          </div>
        </div>

        <div class="px-6 py-4 border-t border-slate-800 flex justify-end space-x-3">
          <button @click="showRejectModal = false" class="bg-slate-800 hover:bg-slate-700 text-slate-300 text-sm font-semibold px-4 py-2 rounded-xl transition">
            Cancel
          </button>
          <button @click="submitRejection" class="bg-rose-600 hover:bg-rose-500 text-white text-sm font-semibold px-4 py-2 rounded-xl transition shadow-lg shadow-rose-600/20">
            Reject Request
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
