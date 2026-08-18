<script setup lang="ts">
// 发帖（IA §5.3：板块必选/话题可选、草稿 localStorage 防抖 300ms、7 天过期、恢复提示）
import { nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { showToast } from 'vant'
import { useAuthStore } from '../stores/auth'
import * as api from '../api/content'
import type { Board, Topic } from '../api/types'
import { insertAtCursor } from '../utils/editor'
import ImagePicker from '../components/ImagePicker.vue'

const router = useRouter()
const auth = useAuthStore()

const boards = ref<Board[]>([])
const topics = ref<Topic[]>([])
const title = ref('')
const content = ref('')
const boardId = ref<number | null>(null)
const topicId = ref<number | null>(null)
const showBoardSheet = ref(false)
const showTopicSheet = ref(false)
const submitting = ref(false)
const contentEl = ref<HTMLTextAreaElement | null>(null)

const draftKey = () => `af_draft_${auth.user?.id ?? 0}`
let draftTimer: ReturnType<typeof setTimeout> | null = null

function saveDraft() {
  if (draftTimer) clearTimeout(draftTimer)
  draftTimer = setTimeout(() => {
    localStorage.setItem(
      draftKey(),
      JSON.stringify({
        title: title.value,
        content: content.value,
        boardId: boardId.value,
        topicId: topicId.value,
        savedAt: Date.now(),
      }),
    )
  }, 300)
}
function restoreDraft() {
  try {
    const raw = localStorage.getItem(draftKey())
    if (!raw) return
    const d = JSON.parse(raw)
    if (Date.now() - d.savedAt > 7 * 86_400_000) {
      localStorage.removeItem(draftKey())
      return
    }
    title.value = d.title ?? ''
    content.value = d.content ?? ''
    boardId.value = d.boardId ?? null
    topicId.value = d.topicId ?? null
    if (title.value || content.value) showToast('已恢复草稿')
  } catch {
    /* 草稿损坏则忽略 */
  }
}
function clearDraft() {
  localStorage.removeItem(draftKey())
}

onMounted(async () => {
  try {
    const [b, t] = await Promise.all([api.listBoards(), api.listTopics()])
    boards.value = b.items
    topics.value = t.items
  } catch (e) {
    showToast((e as Error).message || '加载失败')
  }
  restoreDraft()
})
onUnmounted(() => {
  if (draftTimer) clearTimeout(draftTimer)
  saveDraft()
})
watch([title, content, boardId, topicId], saveDraft)

const boardName = () => boards.value.find((b) => b.id === boardId.value)?.name ?? '选择板块'
const filteredTopics = () => topics.value.filter((t) => t.board_id === boardId.value)

function selectBoard(b: Board) {
  boardId.value = b.id
  topicId.value = null
  showBoardSheet.value = false
}
function selectTopic(t: Topic) {
  topicId.value = t.id
  showTopicSheet.value = false
}

// 图片上传成功：把 URL 插入正文光标处（#12 D4：图片独占一行，光标移到 URL 后）
function onImage(url: string) {
  const el = contentEl.value
  const start = el?.selectionStart ?? content.value.length
  const end = el?.selectionEnd ?? start
  const { value, cursor } = insertAtCursor(content.value, start, end, url, { block: true })
  content.value = value
  nextTick(() => {
    el?.focus()
    el?.setSelectionRange(cursor, cursor)
  })
}

async function submit() {
  if (!boardId.value) {
    showToast('请选择板块')
    return
  }
  if (!title.value.trim() || !content.value.trim()) {
    showToast('标题与内容不能为空')
    return
  }
  submitting.value = true
  try {
    const p = await api.createPost({
      board_id: boardId.value,
      topic_id: topicId.value,
      title: title.value.trim(),
      content: content.value,
    })
    clearDraft()
    showToast('发布成功')
    router.replace(`/post/${p.id}`)
  } catch (e) {
    showToast((e as Error).message || '发布失败')
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <div class="page write">
    <van-cell-group inset>
      <van-field
        v-model="title"
        label="标题"
        placeholder="起个清晰的标题"
        maxlength="255"
      />
      <van-field
        :model-value="boardName()"
        is-link
        label="板块"
        readonly
        placeholder="选择板块（必选）"
        @click="showBoardSheet = true"
      />
      <van-field
        :model-value="topicId ? '#' + (topics.find((t) => t.id === topicId)?.name ?? '') : ''"
        is-link
        label="话题"
        readonly
        placeholder="选择话题（可选）"
        :disabled="!boardId"
        @click="boardId && (showTopicSheet = true)"
      />
    </van-cell-group>

    <div class="editor">
      <div class="toolbar">
        <ImagePicker @uploaded="onImage" />
        <span class="toolbar-hint">图片上传后自动插入光标处</span>
      </div>
      <textarea
        ref="contentEl"
        v-model="content"
        class="content-input"
        placeholder="正文（支持 Markdown：代码块、行内代码、**粗体**、链接、图片）"
        rows="10"
      ></textarea>
    </div>

    <div class="submit">
      <van-button round block type="primary" :loading="submitting" @click="submit">发布</van-button>
    </div>

    <van-popup v-model:show="showBoardSheet" position="bottom" round>
      <div class="sheet">
        <div class="stitle">选择板块</div>
        <button v-for="b in boards" :key="b.id" class="sitem" @click="selectBoard(b)">
          {{ b.name }}
        </button>
      </div>
    </van-popup>

    <van-popup v-model:show="showTopicSheet" position="bottom" round>
      <div class="sheet">
        <div class="stitle">选择话题</div>
        <button
          v-for="t in filteredTopics()"
          :key="t.id"
          class="sitem"
          @click="selectTopic(t)"
        >
          # {{ t.name }}
        </button>
      </div>
    </van-popup>
  </div>
</template>

<style scoped>
.write {
  padding-bottom: 40px;
}
.editor {
  padding: 12px 16px;
}
.toolbar {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 10px;
}
.toolbar-hint {
  font-size: 11.5px;
  color: var(--ink-3);
}
.content-input {
  width: 100%;
  min-height: 200px;
  background: var(--surface);
  border: 1px solid var(--line);
  border-radius: 14px;
  padding: 14px;
  font-size: 14px;
  line-height: 1.7;
  resize: vertical;
  outline: none;
  color: var(--ink);
}
.content-input::placeholder {
  color: var(--ink-3);
}
.submit {
  padding: 0 16px;
}
.sheet {
  padding: 8px 0 calc(16px + env(safe-area-inset-bottom));
  max-height: 60vh;
  overflow-y: auto;
}
.stitle {
  font-size: 15px;
  font-weight: 700;
  text-align: center;
  padding: 10px 20px;
}
.sitem {
  display: flex;
  align-items: center;
  width: 100%;
  min-height: 48px;
  padding: 12px 24px;
  font-size: 14.5px;
  text-align: left;
}
.sitem:active {
  background: var(--surface-2);
}
</style>
