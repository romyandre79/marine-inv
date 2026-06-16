<script setup lang="ts">
import { ref, computed, watch, onMounted, onBeforeUnmount, nextTick } from 'vue'

definePageMeta({
  middleware: 'auth'
})

const authStore = useAuthStore()
const config = useRuntimeConfig()

const activeTab = ref<'ai' | 'chat'>('ai')
const messageInput = ref('')
const socket = ref<WebSocket | null>(null)
const messages = ref<any[]>([])
const onlineUsers = ref<string[]>([])
const usersList = ref<any[]>([])
const activeUserChat = ref<any | null>(null)
const isTyping = ref(false)
const messagesContainer = ref<HTMLElement | null>(null)
const unreadCounts = ref<Record<string, number>>({})
const searchFilter = ref('')

// WebSocket Connection URL
const wsUrl = computed(() => {
  const base = config.public.apiUrl
  const wsProto = base && base.startsWith('https') ? 'wss' : 'ws'
  return `${(base || '').replace(/^http[s]?/, wsProto)}/chat/ws?token=${authStore.token}`
})

// Filter users list based on search input
const filteredUsers = computed(() => {
  const query = searchFilter.value.toLowerCase().trim()
  const list = usersList.value.filter(u => u.id !== authStore.user?.id)
  if (!query) return list
  return list.filter(u => u.name.toLowerCase().includes(query) || u.email.toLowerCase().includes(query))
})

// Fetch all registered users to map names/emails
const fetchUsers = async () => {
  if (!authStore.token) return
  try {
    const res = await $fetch<any>(`${config.public.apiUrl}/users`, {
      headers: {
        Authorization: `Bearer ${authStore.token}`
      }
    })
    if (res?.success) {
      usersList.value = res.data || []
    }
  } catch (e) {
    console.error('Failed to fetch users list', e)
  }
}

// Fetch chat history
const fetchHistory = async () => {
  if (!authStore.token) return
  try {
    let url = `${config.public.apiUrl}/chat/history`
    if (activeTab.value === 'ai') {
      url += '?is_ai=true'
    } else if (activeUserChat.value) {
      url += `?receiver_id=${activeUserChat.value.id}`
    } else {
      messages.value = []
      return
    }

    const res = await $fetch<any>(url, {
      headers: {
        Authorization: `Bearer ${authStore.token}`
      }
    })
    if (res?.success) {
      messages.value = res.data || []
      scrollToBottom()
    }
  } catch (e) {
    console.error('Failed to fetch history', e)
  }
}

// Initialize WebSocket Connection
const initWebSocket = () => {
  if (!authStore.token || socket.value) return

  socket.value = new WebSocket(wsUrl.value)

  socket.value.onopen = () => {
    console.log('Chat page WebSocket connected')
  }

  socket.value.onmessage = (event) => {
    const data = JSON.parse(event.data)

    if (data.type === 'online_list') {
      onlineUsers.value = data.data || []
    } else if (data.type === 'typing') {
      if (activeTab.value === 'ai' && data.data.is_typing) {
        isTyping.value = true
        scrollToBottom()
      } else {
        isTyping.value = false
      }
    } else if (data.type === 'message') {
      const msg = data
      
      // Determine if message belongs to active thread
      const isCurrentAI = activeTab.value === 'ai' && msg.is_ai
      const isCurrentDirect = activeTab.value === 'chat' && activeUserChat.value &&
        ((msg.sender_id === activeUserChat.value.id && msg.receiver_id === authStore.user?.id) ||
         (msg.sender_id === authStore.user?.id && msg.receiver_id === activeUserChat.value.id))

      if (isCurrentAI || isCurrentDirect) {
        messages.value.push(msg)
        scrollToBottom()
      } else {
        // Increment unread count
        const senderId = msg.sender_id
        if (senderId !== authStore.user?.id) {
          unreadCounts.value[senderId] = (unreadCounts.value[senderId] || 0) + 1
        }
      }
    }
  }

  socket.value.onclose = () => {
    console.log('Chat page WebSocket disconnected, reconnecting...')
    socket.value = null
    setTimeout(initWebSocket, 3000)
  }
}

// Map User UUID to Name
const getUserName = (id: string) => {
  const user = usersList.value.find(u => u.id === id)
  return user ? user.name : 'Unknown User'
}

const getOnlineStatus = (id: string) => {
  return onlineUsers.value.includes(id)
}

// Send Message
const sendMessage = () => {
  if (!messageInput.value.trim() || !socket.value) return

  const payload: any = {
    content: messageInput.value
  }

  if (activeTab.value === 'ai') {
    payload.is_ai = true
  } else if (activeUserChat.value) {
    payload.receiver_id = activeUserChat.value.id
    payload.is_ai = false
  } else {
    return
  }

  socket.value.send(JSON.stringify(payload))
  messageInput.value = ''
}

// Select a user to direct chat
const selectUserChat = (user: any) => {
  activeUserChat.value = user
  unreadCounts.value[user.id] = 0
  fetchHistory()
}

// Scroll to bottom helper
const scrollToBottom = () => {
  nextTick(() => {
    if (messagesContainer.value) {
      messagesContainer.value.scrollTop = messagesContainer.value.scrollHeight
    }
  })
}

// Watchers
watch(activeTab, (newTab) => {
  if (newTab === 'ai') {
    activeUserChat.value = null
  } else {
    // Select first user as default if available
    const nonSelfUsers = usersList.value.filter(u => u.id !== authStore.user?.id)
    if (nonSelfUsers.length > 0 && !activeUserChat.value) {
      selectUserChat(nonSelfUsers[0])
    }
  }
  fetchHistory()
})

onMounted(() => {
  if (authStore.isAuthenticated) {
    fetchUsers()
    fetchHistory()
    initWebSocket()
  }
})

onBeforeUnmount(() => {
  if (socket.value) {
    socket.value.close()
  }
})
</script>

<template>
  <div class="h-[calc(100vh-120px)] flex rounded-2xl border border-slate-800 bg-slate-950/60 overflow-hidden font-sans">
    
    <!-- Sidebar Area -->
    <div class="w-80 border-r border-slate-800 flex flex-col bg-slate-950/40">
      
      <!-- Tabs Switcher -->
      <div class="grid grid-cols-2 border-b border-slate-800 p-2 gap-1 text-xs font-bold">
        <button
          @click="activeTab = 'ai'"
          class="flex items-center justify-center gap-2 py-2.5 rounded-xl transition-all"
          :class="activeTab === 'ai' ? 'bg-blue-600/20 text-blue-400 border border-blue-500/20' : 'text-slate-400 hover:bg-slate-900'"
        >
          <Icon name="heroicons:cpu-chip" class="h-4 w-4" />
          <span>AI Assistant</span>
        </button>
        <button
          @click="activeTab = 'chat'"
          class="flex items-center justify-center gap-2 py-2.5 rounded-xl transition-all relative"
          :class="activeTab === 'chat' ? 'bg-blue-600/20 text-blue-400 border border-blue-500/20' : 'text-slate-400 hover:bg-slate-900'"
        >
          <Icon name="heroicons:users" class="h-4 w-4" />
          <span>Team Chat</span>
          <span v-if="Object.values(unreadCounts).some(c => c > 0)" class="absolute top-1.5 right-3 h-2 w-2 rounded-full bg-rose-500" />
        </button>
      </div>

      <!-- User Search (Visible only on chat tab) -->
      <div v-if="activeTab === 'chat'" class="p-3 border-b border-slate-800/80">
        <div class="relative flex items-center">
          <Icon name="heroicons:magnifying-glass" class="absolute left-3.5 h-4 w-4 text-slate-500" />
          <input
            v-model="searchFilter"
            type="text"
            placeholder="Search team members..."
            class="w-full rounded-xl border border-slate-800 bg-slate-900/60 pl-10 pr-4 py-2 text-xs text-white placeholder-slate-500 outline-none focus:border-blue-500"
          />
        </div>
      </div>

      <!-- Left Sidebar Threads List -->
      <div class="flex-1 overflow-y-auto p-3 space-y-1.5">
        
        <!-- AI Option -->
        <div
          v-if="activeTab === 'ai'"
          class="flex items-center gap-3.5 rounded-xl p-3.5 border transition-all cursor-pointer bg-blue-600/10 border-blue-500/20 text-blue-400"
        >
          <div class="h-9 w-9 rounded-xl bg-blue-600 flex items-center justify-center text-white shrink-0">
            <Icon name="heroicons:cpu-chip" class="h-5 w-5" />
          </div>
          <div>
            <p class="text-xs font-bold text-slate-200">System AI Assistant</p>
            <p class="text-[10px] text-slate-400">Context Help & Docs</p>
          </div>
        </div>

        <!-- Online Users list -->
        <template v-else>
          <div
            v-for="user in filteredUsers"
            :key="user.id"
            @click="selectUserChat(user)"
            class="flex items-center justify-between rounded-xl p-3 cursor-pointer border transition-all"
            :class="activeUserChat?.id === user.id
              ? 'bg-blue-600/10 border-blue-500/25 text-white'
              : 'bg-transparent border-transparent text-slate-400 hover:bg-slate-900 hover:text-slate-200'"
          >
            <div class="flex items-center gap-3">
              <div class="relative">
                <div class="h-9 w-9 rounded-xl bg-slate-800 flex items-center justify-center text-xs font-bold text-slate-300 border border-slate-700/50">
                  {{ user.name.charAt(0).toUpperCase() }}
                </div>
                <div
                  class="absolute -bottom-0.5 -right-0.5 h-2.5 w-2.5 rounded-full border border-slate-950"
                  :class="getOnlineStatus(user.id) ? 'bg-green-500 animate-pulse' : 'bg-slate-600'"
                />
              </div>
              <div>
                <p class="text-xs font-semibold text-slate-200">{{ user.name }}</p>
                <p class="text-[10px] text-slate-500 truncate max-w-[150px]">{{ user.email }}</p>
              </div>
            </div>
            
            <span v-if="unreadCounts[user.id] > 0" class="rounded-full bg-rose-500 px-2 py-0.5 text-[9px] font-bold text-white">
              {{ unreadCounts[user.id] }}
            </span>
          </div>
          <div v-if="filteredUsers.length === 0" class="text-center py-8 text-xs text-slate-500">
            No team members found.
          </div>
        </template>
      </div>
    </div>

    <!-- Main Message Box Area -->
    <div class="flex-1 flex flex-col bg-slate-950/20">
      
      <!-- Thread Header -->
      <div class="h-16 border-b border-slate-800 px-6 flex items-center justify-between bg-slate-950/30">
        <div class="flex items-center gap-3">
          <div class="relative">
            <div class="h-9 w-9 rounded-xl bg-slate-800 flex items-center justify-center text-xs font-bold text-slate-300 border border-slate-700/50">
              <Icon v-if="activeTab === 'ai'" name="heroicons:cpu-chip" class="h-5 w-5 text-blue-400" />
              <span v-else>{{ activeUserChat?.name.charAt(0).toUpperCase() }}</span>
            </div>
            <div
              v-if="activeTab === 'chat' && activeUserChat"
              class="absolute -bottom-0.5 -right-0.5 h-2.5 w-2.5 rounded-full border border-slate-950"
              :class="getOnlineStatus(activeUserChat.id) ? 'bg-green-500' : 'bg-slate-600'"
            />
          </div>
          <div>
            <p class="text-xs font-bold text-white">
              {{ activeTab === 'ai' ? 'Marines System AI Assistant' : (activeUserChat?.name || 'Select a Conversation') }}
            </p>
            <p class="text-[10px] text-slate-500">
              {{ activeTab === 'ai' ? 'Online • Virtual Guide' : (activeUserChat ? (getOnlineStatus(activeUserChat.id) ? 'Online' : 'Offline') : 'Offline') }}
            </p>
          </div>
        </div>
      </div>

      <!-- Messages List -->
      <div ref="messagesContainer" class="flex-1 overflow-y-auto p-6 space-y-4">
        
        <!-- Welcome message for AI -->
        <div v-if="activeTab === 'ai' && messages.length === 0" class="max-w-2xl mx-auto space-y-4 py-8">
          <div class="rounded-2xl border border-slate-800 bg-slate-900/20 p-6 text-sm text-slate-400 space-y-3">
            <p class="font-bold text-slate-200 text-base">Welcome to System AI Assistant!</p>
            <p>I am a helper engine specialized in navigating and explaining the Marine ecosystem apps and master shipping documentation. You can ask queries like:</p>
            <div class="grid grid-cols-2 gap-2 mt-4 text-xs font-medium">
              <button @click="messageInput = 'What are the main functions of DMS?'; sendMessage()" class="text-left rounded-xl p-3 border border-slate-800/80 bg-slate-900/60 hover:bg-slate-900 text-slate-300 hover:border-blue-500/30 transition-all">
                What are the main functions of DMS?
              </button>
              <button @click="messageInput = 'Explain the Bill of Lading master document'; sendMessage()" class="text-left rounded-xl p-3 border border-slate-800/80 bg-slate-900/60 hover:bg-slate-900 text-slate-300 hover:border-blue-500/30 transition-all">
                Explain the Bill of Lading master document
              </button>
              <button @click="messageInput = 'What features does CMS offer?'; sendMessage()" class="text-left rounded-xl p-3 border border-slate-800/80 bg-slate-900/60 hover:bg-slate-900 text-slate-300 hover:border-blue-500/30 transition-all">
                What features does CMS offer?
              </button>
              <button @click="messageInput = 'Tell me about the Financial (FIN) system'; sendMessage()" class="text-left rounded-xl p-3 border border-slate-800/80 bg-slate-900/60 hover:bg-slate-900 text-slate-300 hover:border-blue-500/30 transition-all">
                Tell me about the Financial (FIN) system
              </button>
            </div>
          </div>
        </div>

        <!-- Active direct message empty state -->
        <div v-else-if="activeTab === 'chat' && !activeUserChat" class="h-full flex items-center justify-center flex-col text-slate-500 space-y-2">
          <Icon name="heroicons:chat-bubble-left-right" class="h-12 w-12 text-slate-700" />
          <p class="text-sm">Select a team member to start chatting.</p>
        </div>

        <div v-else-if="activeTab === 'chat' && messages.length === 0" class="h-full flex items-center justify-center flex-col text-slate-600 space-y-2">
          <Icon name="heroicons:chat-bubble-left" class="h-10 w-10 text-slate-800" />
          <p class="text-xs">No messages yet. Start the conversation!</p>
        </div>

        <!-- Chat History Messages -->
        <template v-if="(activeTab === 'ai') || (activeTab === 'chat' && activeUserChat)">
          <div
            v-for="msg in messages"
            :key="msg.id"
            class="flex flex-col"
            :class="msg.sender_id === authStore.user?.id && !msg.is_ai ? 'items-end' : 'items-start'"
          >
            <div class="flex items-end gap-2 max-w-[75%]" :class="msg.sender_id === authStore.user?.id && !msg.is_ai ? 'flex-row-reverse' : ''">
              <!-- Avatar -->
              <div class="h-7 w-7 rounded-xl bg-slate-800 flex items-center justify-center text-[10px] font-bold text-slate-400 shrink-0 border border-slate-700/30">
                <Icon v-if="msg.is_ai" name="heroicons:cpu-chip" class="h-4 w-4 text-blue-400" />
                <span v-else>{{ getUserName(msg.sender_id).charAt(0).toUpperCase() }}</span>
              </div>
              
              <!-- Message Bubble -->
              <div
                class="rounded-2xl px-4 py-3 text-sm leading-relaxed"
                :class="msg.sender_id === authStore.user?.id && !msg.is_ai
                  ? 'bg-blue-600 text-white rounded-tr-none'
                  : 'bg-slate-800/80 text-slate-100 border border-slate-700/60 rounded-tl-none'"
              >
                <div class="whitespace-pre-line prose prose-invert prose-xs text-xs">
                  <!-- Custom rendering for markdown headers inside AI response -->
                  <p>{{ msg.content }}</p>
                </div>
              </div>
            </div>
            
            <span class="text-[10px] text-slate-500 mt-1.5 px-10">
              {{ new Date(msg.created_at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }) }}
            </span>
          </div>

          <!-- Typing Indicator -->
          <div v-if="isTyping" class="flex items-center gap-2 max-w-[70%]">
            <div class="h-7 w-7 rounded-xl bg-slate-800 flex items-center justify-center text-[10px] font-bold text-slate-400 shrink-0 border border-slate-700/30">
              <Icon name="heroicons:cpu-chip" class="h-4 w-4 text-blue-400" />
            </div>
            <div class="flex items-center gap-1.5 p-3.5 bg-slate-800/40 border border-slate-700/50 rounded-2xl rounded-tl-none">
              <div class="h-2 w-2 rounded-full bg-slate-400 animate-bounce" />
              <div class="h-2 w-2 rounded-full bg-slate-400 animate-bounce [animation-delay:0.2s]" />
              <div class="h-2 w-2 rounded-full bg-slate-400 animate-bounce [animation-delay:0.4s]" />
            </div>
          </div>
        </template>
      </div>

      <!-- Message Input Bar -->
      <div v-if="(activeTab === 'ai') || (activeTab === 'chat' && activeUserChat)" class="p-4 border-t border-slate-800 bg-slate-950/40">
        <div class="flex gap-2 max-w-4xl mx-auto">
          <input
            v-model="messageInput"
            type="text"
            :placeholder="activeTab === 'ai' ? 'Ask System AI Assistant...' : 'Type message here...'"
            class="flex-1 rounded-xl border border-slate-800 bg-slate-900 px-4 py-3 text-xs text-white placeholder-slate-500 outline-none focus:border-blue-500"
            @keyup.enter="sendMessage"
          />
          <button
            @click="sendMessage"
            class="flex h-11 w-11 items-center justify-center rounded-xl bg-blue-600 text-white hover:bg-blue-500 hover:shadow-lg hover:shadow-blue-500/20 active:scale-95 transition-all"
          >
            <Icon name="heroicons:paper-airplane" class="h-5 w-5" />
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
