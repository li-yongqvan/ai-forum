<script setup lang="ts">
// 图片选择器（#12 D4）：选择 → 上传 → 返回 URL → 交给调用方插入 markdown 光标处。
// 上传中禁用按钮（van-loading 反馈）；失败 toast；成功不在此处理，交由父组件 insertAtCursor。
import { ref } from 'vue'
import { showToast } from 'vant'
import { uploadImage } from '../api/upload'
import AppIcon from './AppIcon.vue'

const props = withDefaults(
  defineProps<{
    /** 打开选择器前的守卫：返回 false 则取消打开（如未登录跳登录墙）。 */
    beforeOpen?: () => boolean
  }>(),
  { beforeOpen: undefined },
)
const emit = defineEmits<{ (e: 'uploaded', url: string): void }>()

const input = ref<HTMLInputElement | null>(null)
const uploading = ref(false)

function open() {
  if (uploading.value) return
  if (props.beforeOpen && !props.beforeOpen()) return
  input.value?.click()
}

async function onPick(e: Event) {
  const target = e.target as HTMLInputElement
  const file = target.files?.[0]
  target.value = '' // 允许重复选择同一文件
  if (!file) return
  uploading.value = true
  try {
    const res = await uploadImage(file)
    emit('uploaded', res.url)
  } catch (err) {
    showToast((err as Error).message || '图片上传失败')
  } finally {
    uploading.value = false
  }
}
</script>

<template>
  <button
    type="button"
    class="img-btn"
    :disabled="uploading"
    :aria-label="uploading ? '图片上传中' : '插入图片'"
    @click="open"
  >
    <van-loading v-if="uploading" size="16" color="var(--ink-2)" />
    <AppIcon v-else name="image" :size="18" />
  </button>
  <input ref="input" type="file" accept="image/*" hidden @change="onPick" />
</template>

<style scoped>
.img-btn {
  width: 40px;
  height: 40px;
  flex: none;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 12px;
  color: var(--ink-2);
  background: var(--surface-2);
}
.img-btn:active {
  opacity: 0.7;
}
.img-btn:disabled {
  opacity: 0.5;
}
</style>
