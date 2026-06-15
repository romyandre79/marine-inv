<script setup lang="ts">
definePageMeta({
  middleware: [
    function (to, from) {
      return navigateTo('/inventory')
    }
  ]
})
</script>

<template>
  <div class="flex items-center justify-center min-h-[200px]">
    <div class="animate-pulse text-sm text-slate-400">Loading inventory context...</div>
  </div>
</template>
