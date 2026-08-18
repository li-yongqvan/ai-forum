<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(defineProps<{ name: string; color?: string; size?: number }>(), {
  size: 38,
})
const palette = ['#1F3A5F', '#2B5FA8', '#C07A10', '#8A3B7E', '#2F7D50', '#B5474C']
const initial = computed(() => (props.name ? props.name.slice(0, 1) : '?'))
const bg = computed(
  () => props.color ?? palette[props.name ? props.name.charCodeAt(0) % palette.length : 0],
)
</script>

<template>
  <span
    class="avatar"
    :style="{
      background: bg,
      width: size + 'px',
      height: size + 'px',
      fontSize: Math.round(size * 0.4) + 'px',
    }"
  >
    {{ initial }}
  </span>
</template>

<style scoped>
.avatar {
  flex: none;
  border-radius: 50%;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: #fff;
  font-weight: 700;
}
</style>
