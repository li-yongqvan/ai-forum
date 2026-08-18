<script setup lang="ts">
defineProps<{ options: { key: string; label: string }[]; modelValue: string }>()
const emit = defineEmits<{ (e: 'update:modelValue', v: string): void }>()
</script>

<template>
  <div class="seg">
    <button
      v-for="o in options"
      :key="o.key"
      :class="['seg-item', { on: modelValue === o.key }]"
      @click="emit('update:modelValue', o.key)"
    >
      {{ o.label }}
    </button>
  </div>
</template>

<style scoped>
.seg {
  display: flex;
  background: var(--surface-2);
  border-radius: 12px;
  padding: 3px;
  margin: 10px 16px 4px;
}
.seg-item {
  flex: 1;
  height: 36px;
  border-radius: 9px;
  font-size: 13.5px;
  font-weight: 600;
  color: var(--ink-2);
  transition: all 0.18s;
}
.seg-item.on {
  background: var(--brand);
  color: #fff;
}
</style>
