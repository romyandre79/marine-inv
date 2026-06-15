<script setup lang="ts">
const { mobileOpen, closeMobile } = useSidebar()
</script>

<template>
  <div class="flex h-dvh w-screen bg-slate-950 text-slate-100 font-sans overflow-hidden">

    <!-- Mobile backdrop -->
    <Transition
      enter-active-class="transition-opacity duration-300"
      enter-from-class="opacity-0"
      enter-to-class="opacity-100"
      leave-active-class="transition-opacity duration-200"
      leave-from-class="opacity-100"
      leave-to-class="opacity-0"
    >
      <div
        v-if="mobileOpen"
        class="fixed inset-0 z-20 bg-black/60 md:hidden"
        @click="closeMobile"
      />
    </Transition>

    <!-- Sidebar — inline on md+, overlay on mobile -->
    <LayoutSidebar />

    <!-- Main Content Area -->
    <div class="flex-1 flex flex-col min-w-0 overflow-hidden">
      <!-- Header -->
      <LayoutHeader />

      <!-- Page Content — scrollable -->
      <main class="flex-1 min-h-0 overflow-y-auto">
        <div class="p-4 md:p-8">
          <slot />
        </div>
      </main>
    </div>
  </div>
</template>
