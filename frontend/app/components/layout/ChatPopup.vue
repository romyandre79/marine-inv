<script setup lang="ts">
import { ref, computed, watch, onMounted, onBeforeUnmount, nextTick } from 'vue'

const authStore = useAuthStore()
const config = useRuntimeConfig()

const isOpen = ref(false)
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

// WebSocket Connection URL
const wsUrl = computed(() => {
  const base = config.public.apiUrl
  const wsProto = base && base.startsWith('https') ? 'wss' : 'ws'
  return `${(base || '').replace(/^http[s]?/, wsProto)}/chat/ws?token=${authStore.token}`
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
    console.log('Chat WebSocket connected')
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
      
      const isCurrentAI = activeTab.value === 'ai' && msg.is_ai
      const isCurrentDirect = activeTab.value === 'chat' && activeUserChat.value &&
        ((msg.sender_id === activeUserChat.value.id && msg.receiver_id === authStore.user?.id) ||
         (msg.sender_id === authStore.user?.id && msg.receiver_id === activeUserChat.value.id))

      // Always push to the current messages list if it matches the active thread
      if (isCurrentAI || isCurrentDirect) {
        messages.value.push(msg)
        scrollToBottom()
      }

      // ALWAYS trigger notification, chime sound, and increment unread count for other users' messages
      const senderId = msg.sender_id
      if (senderId !== authStore.user?.id) {
        // If we are currently active on this thread and the chat box is open, we can skip incrementing the badge
        const isReadingNow = isCurrentDirect && isOpen.value
        
        if (!isReadingNow) {
          unreadCounts.value[senderId] = (unreadCounts.value[senderId] || 0) + 1
        }
        
        // Trigger browser notification and sound in any condition
        playNotificationSound()
        showWebNotification(msg)
      }
    }
  }

  socket.value.onclose = () => {
    console.log('Chat WebSocket disconnected, reconnecting...')
    socket.value = null
    setTimeout(initWebSocket, 3000)
  }
}

// Total unread count
const totalUnread = computed(() => {
  return Object.values(unreadCounts.value).reduce((sum, count) => sum + count, 0)
})

// Play simple browser-native notification audio chime
const playNotificationSound = () => {
  try {
    const context = new (window.AudioContext || (window as any).webkitAudioContext)()
    const osc = context.createOscillator()
    const gain = context.createGain()
    osc.connect(gain)
    gain.connect(context.destination)
    
    // Play a friendly chime (two-tone beep)
    osc.frequency.setValueAtTime(523.25, context.currentTime) // C5
    gain.gain.setValueAtTime(0.1, context.currentTime)
    osc.start(context.currentTime)
    osc.stop(context.currentTime + 0.1)
    
    setTimeout(() => {
      const osc2 = context.createOscillator()
      const gain2 = context.createGain()
      osc2.connect(gain2)
      gain2.connect(context.destination)
      osc2.frequency.setValueAtTime(659.25, context.currentTime) // E5
      gain2.gain.setValueAtTime(0.1, context.currentTime)
      osc2.start(context.currentTime)
      osc2.stop(context.currentTime + 0.15)
    }, 120)
  } catch (e) {
    console.warn('AudioContext not supported or blocked by user gesture', e)
  }
}

// Show standard browser HTML5 desktop notifications
const showWebNotification = (msg: any) => {
  if (!('Notification' in window)) return
  
  const senderName = getUserName(msg.sender_id)
  
  if (Notification.permission === 'granted') {
    new Notification(`New Message from ${senderName}`, {
      body: msg.content,
      icon: '/favicon.svg'
    })
  } else if (Notification.permission !== 'denied') {
    Notification.requestPermission().then(permission => {
      if (permission === 'granted') {
        new Notification(`New Message from ${senderName}`, {
          body: msg.content,
          icon: '/favicon.svg'
        })
      }
    })
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
  }
  fetchHistory()
})

watch(isOpen, (newVal) => {
  if (newVal) {
    fetchUsers()
    fetchHistory()
    initWebSocket()
    scrollToBottom()
    // Reset unread count for the active direct user chat when opening window
    if (activeUserChat.value) {
      unreadCounts.value[activeUserChat.value.id] = 0
    }
  }
})

watch(() => authStore.isAuthenticated, (newVal) => {
  if (newVal) {
    fetchUsers()
    initWebSocket()
  } else {
    if (socket.value) {
      socket.value.close()
      socket.value = null
    }
    unreadCounts.value = {}
  }
})

onMounted(() => {
  if (authStore.isAuthenticated) {
    fetchUsers()
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
  <div v-if="authStore.isAuthenticated" class="fixed bottom-6 right-6 z-50 font-sans">
    <!-- Trigger Button -->
    <button
      @click="isOpen = !isOpen"
      class="relative flex h-14 w-14 items-center justify-center rounded-full bg-gradient-to-tr from-blue-600 to-indigo-600 text-white shadow-xl shadow-blue-500/30 transition-transform duration-300 hover:scale-105 active:scale-95"
    >
      <Icon v-if="!isOpen" name="heroicons:chat-bubble-left-right" class="h-6 w-6" />
      <Icon v-else name="heroicons:x-mark" class="h-6 w-6" />
      
      <!-- Unread Badge Indicator -->
      <span
        v-if="totalUnread > 0 && !isOpen"
        class="absolute -top-1.5 -right-1.5 flex h-5.5 min-w-[22px] items-center justify-center rounded-full bg-rose-500 px-1 text-[10px] font-bold text-white ring-2 ring-slate-950"
      >
        {{ totalUnread }}
      </span>
    </button>

    <!-- Chat Box Window -->
    <Transition
      enter-active-class="transition-all duration-300 ease-out"
      enter-from-class="opacity-0 translate-y-8 scale-95"
      enter-to-class="opacity-100 translate-y-0 scale-100"
      leave-active-class="transition-all duration-200 ease-in"
      leave-from-class="opacity-100 translate-y-0 scale-100"
      leave-to-class="opacity-0 translate-y-8 scale-95"
    >
      <div
        v-if="isOpen"
        class="absolute bottom-18 right-0 flex h-[500px] w-[380px] flex-col rounded-2xl border border-slate-800 bg-slate-950/95 shadow-2xl backdrop-blur-xl"
      >
        <!-- Header -->
        <div class="flex items-center justify-between border-b border-slate-800 p-4">
          <div class="flex items-center gap-2">
            <div class="h-2 w-2 rounded-full bg-green-500 animate-pulse" />
            <span class="text-sm font-bold text-white uppercase tracking-wider">Marines System Chat</span>
          </div>
          <button @click="isOpen = false" class="text-slate-400 hover:text-slate-200">
            <Icon name="heroicons:minus" class="h-5 w-5" />
          </button>
        </div>

        <!-- Navigation Tabs -->
        <div class="grid grid-cols-2 border-b border-slate-800 bg-slate-900/50 text-xs font-semibold">
          <button
            @click="activeTab = 'ai'"
            class="flex items-center justify-center gap-2 py-3 border-b-2 transition-colors"
            :class="activeTab === 'ai' ? 'border-blue-500 text-blue-400' : 'border-transparent text-slate-400 hover:text-slate-200'"
          >
            <Icon name="heroicons:cpu-chip" class="h-4 w-4" />
            <span>AI Assistant</span>
          </button>
          <button
            @click="activeTab = 'chat'"
            class="flex items-center justify-center gap-2 py-3 border-b-2 transition-colors relative"
            :class="activeTab === 'chat' ? 'border-blue-500 text-blue-400' : 'border-transparent text-slate-400 hover:text-slate-200'"
          >
            <Icon name="heroicons:users" class="h-4 w-4" />
            <span>Online Chat</span>
            <span v-if="Object.values(unreadCounts).some(c => c > 0)" class="absolute top-2 right-4 flex h-2 w-2 rounded-full bg-rose-500" />
          </button>
        </div>

        <!-- Content Area -->
        <div class="flex-1 flex flex-col min-h-0">
          <!-- AI Assistant Tab -->
          <div v-if="activeTab === 'ai'" class="flex-1 flex flex-col min-h-0">
            <div ref="messagesContainer" class="flex-1 overflow-y-auto p-4 space-y-4">
              <!-- Welcome message if empty -->
              <div v-if="messages.length === 0" class="space-y-4">
                <div class="rounded-xl border border-slate-800 bg-slate-900/40 p-4 text-xs text-slate-400">
                  <p class="font-bold text-slate-300 mb-2">Hello! How can I assist you today?</p>
                  <p>You can ask me questions about any of the systems in our Marine Portal ecosystem:</p>
                  <ul class="list-disc list-inside mt-2 space-y-1 text-slate-400">
                    <li><strong>MMS</strong>: Marine Management System</li>
                    <li><strong>CMS</strong>: Crew Management System</li>
                    <li><strong>DMS</strong>: Document Management System</li>
                    <li><strong>FIN</strong>: Financial Management System</li>
                    <li><strong>FMS</strong>: Fleet Management System</li>
                    <li><strong>INV</strong>: Inventory System</li>
                    <li><strong>RMS</strong>: Resource Management System</li>
                  </ul>
                  <p class="mt-3">Or inquire about <strong>Master Documents</strong> like Bill of Lading, Manifests, and Crew lists.</p>
                </div>
              </div>

              <!-- Message History -->
              <div
                v-for="msg in messages"
                :key="msg.id"
                class="flex flex-col"
                :class="msg.sender_id === authStore.user?.id && !msg.is_ai ? 'items-end' : 'items-start'"
              >
                <div
                  class="max-w-[85%] rounded-2xl px-4 py-2.5 text-sm leading-relaxed"
                  :class="msg.sender_id === authStore.user?.id && !msg.is_ai
                    ? 'bg-blue-600 text-white rounded-tr-none'
                    : 'bg-slate-800/80 text-slate-100 border border-slate-700/60 rounded-tl-none'"
                >
                  <p class="whitespace-pre-line text-xs">{{ msg.content }}</p>
                </div>
                <span class="text-[10px] text-slate-500 mt-1 px-1">
                  {{ new Date(msg.created_at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }) }}
                </span>
              </div>

              <!-- Typing Indicator -->
              <div v-if="isTyping" class="flex items-center gap-1.5 p-2 bg-slate-800/40 border border-slate-700/50 rounded-xl w-fit">
                <div class="h-1.5 w-1.5 rounded-full bg-slate-400 animate-bounce" />
                <div class="h-1.5 w-1.5 rounded-full bg-slate-400 animate-bounce [animation-delay:0.2s]" />
                <div class="h-1.5 w-1.5 rounded-full bg-slate-400 animate-bounce [animation-delay:0.4s]" />
              </div>
            </div>

            <!-- Input bar -->
            <div class="border-t border-slate-800 p-3 flex gap-2">
              <input
                v-model="messageInput"
                type="text"
                placeholder="Ask AI Assistant..."
                class="flex-1 rounded-xl border border-slate-800 bg-slate-900 px-3.5 py-2 text-xs text-white placeholder-slate-500 outline-none focus:border-blue-500"
                @keyup.enter="sendMessage"
              />
              <button
                @click="sendMessage"
                class="flex h-9 w-9 items-center justify-center rounded-xl bg-blue-600 text-white hover:bg-blue-500 transition-colors"
              >
                <Icon name="heroicons:paper-airplane" class="h-4 w-4" />
              </button>
            </div>
          </div>

          <!-- Direct Online User Chat Tab -->
          <div v-if="activeTab === 'chat'" class="flex-1 flex flex-col min-h-0">
            <!-- User selection list -->
            <div v-if="!activeUserChat" class="flex-1 overflow-y-auto p-4 space-y-2">
              <p class="text-[10px] font-bold text-slate-500 uppercase tracking-widest px-2 mb-2">Team Members</p>
              
              <div
                v-for="user in usersList.filter(u => u.id !== authStore.user?.id)"
                :key="user.id"
                @click="selectUserChat(user)"
                class="flex items-center justify-between rounded-xl p-3 cursor-pointer transition-colors bg-slate-900/30 hover:bg-slate-900 border border-slate-800/40 hover:border-slate-800"
              >
                <div class="flex items-center gap-3">
                  <div class="relative">
                    <div class="h-8 w-8 rounded-full bg-slate-800 flex items-center justify-center text-xs font-bold text-slate-300">
                      {{ user.name.charAt(0).toUpperCase() }}
                    </div>
                    <div
                      class="absolute -bottom-0.5 -right-0.5 h-2.5 w-2.5 rounded-full border border-slate-950"
                      :class="getOnlineStatus(user.id) ? 'bg-green-500' : 'bg-slate-600'"
                    />
                  </div>
                  <div>
                    <p class="text-xs font-semibold text-slate-200">{{ user.name }}</p>
                    <p class="text-[10px] text-slate-500">{{ user.email }}</p>
                  </div>
                </div>
                
                <span v-if="unreadCounts[user.id] > 0" class="rounded-full bg-rose-500 px-2 py-0.5 text-[9px] font-bold text-white">
                  {{ unreadCounts[user.id] }}
                </span>
              </div>
            </div>

            <!-- Direct chat thread -->
            <div v-else class="flex-1 flex flex-col min-h-0">
              <!-- Sub-header to go back -->
              <div class="flex items-center gap-3 bg-slate-900/60 border-b border-slate-800 px-3 py-2">
                <button @click="activeUserChat = null" class="text-slate-400 hover:text-slate-200">
                  <Icon name="heroicons:arrow-left" class="h-4 w-4" />
                </button>
                <div class="flex items-center gap-2">
                  <div class="relative">
                    <div class="h-7 w-7 rounded-full bg-slate-800 flex items-center justify-center text-[10px] font-bold text-slate-300">
                      {{ activeUserChat.name.charAt(0).toUpperCase() }}
                    </div>
                    <div
                      class="absolute -bottom-0.5 -right-0.5 h-2 w-2 rounded-full border border-slate-950"
                      :class="getOnlineStatus(activeUserChat.id) ? 'bg-green-500' : 'bg-slate-600'"
                    />
                  </div>
                  <span class="text-xs font-bold text-slate-200">{{ activeUserChat.name }}</span>
                </div>
              </div>

              <!-- Message History list -->
              <div ref="messagesContainer" class="flex-1 overflow-y-auto p-4 space-y-4">
                <div
                  v-for="msg in messages"
                  :key="msg.id"
                  class="flex flex-col"
                  :class="msg.sender_id === authStore.user?.id ? 'items-end' : 'items-start'"
                >
                  <div
                    class="max-w-[85%] rounded-2xl px-4 py-2.5 text-sm leading-relaxed"
                    :class="msg.sender_id === authStore.user?.id
                      ? 'bg-blue-600 text-white rounded-tr-none'
                      : 'bg-slate-800/80 text-slate-100 border border-slate-700/60 rounded-tl-none'"
                  >
                    <p class="text-xs">{{ msg.content }}</p>
                  </div>
                  <span class="text-[10px] text-slate-500 mt-1 px-1">
                    {{ new Date(msg.created_at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }) }}
                  </span>
                </div>
              </div>

              <!-- Input bar -->
              <div class="border-t border-slate-800 p-3 flex gap-2">
                <input
                  v-model="messageInput"
                  type="text"
                  placeholder="Type message..."
                  class="flex-1 rounded-xl border border-slate-800 bg-slate-900 px-3.5 py-2 text-xs text-white placeholder-slate-500 outline-none focus:border-blue-500"
                  @keyup.enter="sendMessage"
                />
                <button
                  @click="sendMessage"
                  class="flex h-9 w-9 items-center justify-center rounded-xl bg-blue-600 text-white hover:bg-blue-500 transition-colors"
                >
                  <Icon name="heroicons:paper-airplane" class="h-4 w-4" />
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>
    </Transition>
  </div>
</template>
