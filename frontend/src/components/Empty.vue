<script setup lang="ts">
defineProps<{ icon?: string; title: string; desc?: string }>()
</script>

<template>
  <div class="empty">
    <div class="eicon">{{ icon ?? '🫧' }}</div>
    <div class="etitle">{{ title }}</div>
    <div v-if="desc" class="edesc">{{ desc }}</div>
    <slot />
  </div>
</template>

<style scoped>
.empty {
  padding: 56px 32px;
  text-align: center;
  color: var(--ink-2);
}
.eicon {
  width: 72px;
  height: 72px;
  margin: 0 auto 14px;
  border-radius: 24px;
  background: var(--surface-2);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 32px;
}
.etitle {
  font-size: 14.5px;
  font-weight: 600;
  color: var(--ink);
  margin-bottom: 6px;
}
.edesc {
  font-size: 12.5px;
  line-height: 1.6;
  margin-bottom: 16px;
}
</style>
