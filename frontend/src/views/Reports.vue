<script setup lang="ts">
// 举报处理队列（#33，IA §4 /reports + §5.6：mod 队列 → 处理弹窗按目标类型给动作 → 审计 + 结果通知）
import { computed, onMounted, ref } from 'vue'
import { showConfirmDialog, showToast } from 'vant'
import { useAuthStore } from '../stores/auth'
import * as api from '../api/report'
import type { Report, ReportTargetType } from '../api/types'
import { formatTime } from '../utils/format'
import Empty from '../components/Empty.vue'

const auth = useAuthStore()

const items = ref<Report[]>([])
const loading = ref(false)

async function load() {
  loading.value = true
  try {
    items.value = (await api.listReports({ status: 'pending', pageSize: 100 })).items
  } catch (e) {
    showToast((e as Error).message || '加载失败')
  } finally {
    loading.value = false
  }
}
onMounted(load)

// ---- 处理弹窗 ----
const handleShow = ref(false)
const current = ref<Report | null>(null)
const action = ref('')
const note = ref('')
const submitting = ref(false)

// 按目标类型给动作集（D1：忽略/删帖删评/警告；#60 admin 额外可见「封禁用户」）。
// #53 D2：警告会通知被举报人（反转 O4）；#60 ban 也会通知被举报人。
const allowedActions = computed(() => {
  const r = current.value
  if (!r) return []
  const acts: { key: string; label: string; danger?: boolean }[] = [{ key: 'dismiss', label: '忽略' }]
  if (r.target_type === 'post') acts.push({ key: 'delete_post', label: '删除帖子' })
  if (r.target_type === 'comment') acts.push({ key: 'delete_comment', label: '删除评论' })
  acts.push({ key: 'warn', label: '警告（会通知被举报人）' })
  if (auth.isAdmin) acts.push({ key: 'ban_user', label: '封禁用户', danger: true })
  return acts
})

const isBan = computed(() => action.value === 'ban_user')
const notePlaceholder = computed(() =>
  isBan.value ? '封禁原因（必填，≤500 字）' : '处理备注（可选，≤500 字）'
)
const submitLabel = computed(() => (isBan.value ? '确认封禁' : '确认处理'))

const targetLabel = (t: ReportTargetType) => (t === 'post' ? '帖子' : t === 'comment' ? '评论' : '用户')

function openHandle(r: Report) {
  current.value = r
  action.value = ''
  note.value = ''
  handleShow.value = true
}

async function submitHandle() {
  const r = current.value
  if (!r || !action.value) {
    showToast('请选择处理动作')
    return
  }
  const trimmed = note.value.trim()
  if (isBan.value && !trimmed) {
    showToast('请填写封禁原因')
    return
  }
  if (isBan.value) {
    try {
      await showConfirmDialog({
        title: '确认封禁该用户？',
        message: '封禁后该用户无法登录与执行写操作，解封需管理员操作。',
      })
    } catch {
      return // 取消
    }
  }
  submitting.value = true
  try {
    await api.handleReport(r.id, action.value, trimmed || undefined)
    showToast(isBan.value ? '已封禁' : '已处理')
    handleShow.value = false
    load()
  } catch (e) {
    showToast((e as Error).message || '处理失败')
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <div v-if="auth.isMod" class="page">
    <Empty v-if="!loading && !items.length" title="没有待处理的举报" />
    <div v-else class="rlist">
      <div v-for="r in items" :key="r.id" class="row">
        <div class="rtop">
          <span class="tag">{{ targetLabel(r.target_type) }}</span>
          <span class="rtitle">{{ r.target_title || `#${r.target_id}` }}</span>
          <span class="rtime">{{ formatTime(r.created_at) }}</span>
        </div>
        <div class="rreason">
          {{ r.reason }}<template v-if="r.reporter_note">：{{ r.reporter_note }}</template>
          <span class="rreporter">举报人：{{ r.reporter_username || `#${r.reporter_id}` }}</span>
        </div>
        <div class="ract">
          <button class="hbtn" @click="openHandle(r)">处理</button>
        </div>
      </div>
    </div>

    <van-popup v-model:show="handleShow" position="bottom" round>
      <div class="hpop">
        <div class="htitle">
          处理举报<template v-if="current">（{{ targetLabel(current.target_type) }} #{{ current.target_id }}）</template>
        </div>
        <div class="hreason">原因：{{ current?.reason }}</div>
        <div class="acts">
          <button
            v-for="a in allowedActions"
            :key="a.key"
            class="act"
            :class="{ on: action === a.key, danger: a.danger }"
            @click="action = a.key"
          >
            {{ a.label }}
          </button>
        </div>
        <textarea v-model="note" class="note" rows="2" maxlength="500" :placeholder="notePlaceholder" />
        <van-button type="primary" block round class="submit" :loading="submitting" @click="submitHandle">
          {{ submitLabel }}
        </van-button>
      </div>
    </van-popup>
  </div>
  <Empty v-else title="无权限访问" desc="仅管理员可处理举报" />
</template>

<style scoped>
.rlist {
  padding-bottom: 24px;
}
.row {
  background: var(--surface);
  border-bottom: 1px solid var(--line);
  padding: 12px 16px;
}
.rtop {
  display: flex;
  align-items: center;
  gap: 8px;
}
.tag {
  flex-shrink: 0;
  padding: 2px 8px;
  border-radius: 6px;
  font-size: 11px;
  font-weight: 600;
  color: var(--brand);
  background: var(--brand-soft);
}
.rtitle {
  flex: 1;
  min-width: 0;
  font-size: 14px;
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.rtime {
  flex-shrink: 0;
  font-size: 11px;
  color: var(--ink-3);
}
.rreason {
  margin-top: 6px;
  font-size: 12.5px;
  color: var(--ink-2);
  display: flex;
  align-items: center;
  gap: 8px;
}
.rreporter {
  margin-left: auto;
  flex-shrink: 0;
  font-size: 11.5px;
  color: var(--ink-3);
}
.ract {
  margin-top: 8px;
  text-align: right;
}
.hbtn {
  height: 30px;
  padding: 0 14px;
  border-radius: 15px;
  border: none;
  font-size: 12.5px;
  font-weight: 600;
  color: #fff;
  background: var(--brand);
}
.hpop {
  padding: 16px 20px 24px;
}
.htitle {
  font-size: 15px;
  font-weight: 700;
}
.hreason {
  margin-top: 6px;
  font-size: 12.5px;
  color: var(--ink-2);
}
.acts {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 12px;
}
.act {
  padding: 8px 14px;
  border-radius: 16px;
  border: 1px solid var(--line);
  background: transparent;
  font-size: 13px;
  color: var(--ink-2);
}
.act.on {
  border-color: var(--danger);
  color: var(--danger);
  background: color-mix(in srgb, var(--danger) 8%, transparent);
}
.act.danger.on {
  background: var(--danger);
  color: #fff;
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
