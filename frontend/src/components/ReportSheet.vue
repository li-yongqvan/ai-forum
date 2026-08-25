<script setup lang="ts">
// 举报原因选择弹窗（#33）：帖子/评论「···」→ 选原因（D4 六枚举）+ 备注（「其他」必填）→ 提交。
import { computed, ref, watch } from 'vue'
import { showToast } from 'vant'
import { createReport, REPORT_REASONS } from '../api/report'
import type { ReportTargetType } from '../api/types'

const props = defineProps<{
  show: boolean
  targetType: ReportTargetType
  targetId: number
}>()

const emit = defineEmits<{
  (e: 'update:show', v: boolean): void
  (e: 'submitted'): void
}>()

const selected = ref(-1)
const note = ref('')
const submitting = ref(false)

// 每次打开重置表单
watch(
  () => props.show,
  (v) => {
    if (v) {
      selected.value = -1
      note.value = ''
    }
  },
)

const title = computed(() => {
  switch (props.targetType) {
    case 'post':
      return '举报帖子'
    case 'comment':
      return '举报评论'
    default:
      return '举报用户'
  }
})

async function submit() {
  if (selected.value < 0) {
    showToast('请选择举报原因')
    return
  }
  const reason = REPORT_REASONS[selected.value]
  if (reason === '其他' && !note.value.trim()) {
    showToast('选「其他」时请填写说明')
    return
  }
  submitting.value = true
  try {
    await createReport({
      target_type: props.targetType,
      target_id: props.targetId,
      reason,
      note: note.value.trim() || undefined,
    })
    showToast('举报已提交，管理员会尽快处理')
    emit('update:show', false)
    emit('submitted')
  } catch (e) {
    // 409 重复举报等错误文案由 ApiError 直接透出（F1/D5 防重复）
    showToast((e as Error).message || '举报失败')
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <van-action-sheet :show="show" :title="title" @update:show="(v: boolean) => emit('update:show', v)">
    <div class="rpsheet">
      <div class="reasons">
        <button
          v-for="(r, i) in REPORT_REASONS"
          :key="r"
          type="button"
          class="reason-item"
          :class="{ on: selected === i }"
          @click="selected = i"
        >
          <span class="dot" />{{ r }}
        </button>
      </div>
      <textarea v-model="note" class="note" rows="3" maxlength="200" placeholder="补充说明（选「其他」时必填）" />
      <van-button type="primary" block round class="submit" :loading="submitting" @click="submit">
        提交举报
      </van-button>
    </div>
  </van-action-sheet>
</template>

<style scoped>
.rpsheet {
  padding: 8px 20px 24px;
}
.reasons {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
}
.reason-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 12px 14px;
  border: 1px solid var(--line);
  border-radius: 10px;
  background: transparent;
  font-size: 13.5px;
  color: var(--ink-2);
  cursor: pointer;
}
.reason-item.on {
  border-color: var(--accent-strong);
  color: var(--accent-ink);
  background: var(--accent-soft);
}
.dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  border: 1.5px solid var(--ink-3);
  flex: none;
}
.reason-item.on .dot {
  background: var(--accent-strong);
  border-color: var(--accent-strong);
}
.note {
  width: 100%;
  box-sizing: border-box;
  margin-top: 12px;
  padding: 10px 12px;
  border: 1px solid var(--line);
  border-radius: 10px;
  background: transparent;
  color: var(--ink);
  font-size: 13px;
  resize: none;
}
.submit {
  margin-top: 12px;
}
</style>
