import { request } from './client'
import type {
  Board,
  CommentTree,
  FollowTarget,
  Post,
  PostList,
  Topic,
  UserProfile,
} from './types'

// ---- 板块 / 话题 ----

export function listBoards() {
  return request<{ items: Board[] }>('GET', '/boards')
}

export function listTopics(boardId?: number) {
  const q = boardId !== undefined ? `?board_id=${boardId}` : ''
  return request<{ items: Topic[] }>('GET', `/topics${q}`)
}

// ---- 帖子 ----

export interface ListPostsParams {
  tab?: 'all' | 'follow'
  boardId?: number
  topicId?: number
  authorId?: number
  page?: number
  pageSize?: number
}

export function listPosts(p: ListPostsParams = {}) {
  const qs = new URLSearchParams()
  if (p.tab) qs.set('tab', p.tab)
  if (p.boardId !== undefined) qs.set('board_id', String(p.boardId))
  if (p.topicId !== undefined) qs.set('topic_id', String(p.topicId))
  if (p.authorId !== undefined) qs.set('author_id', String(p.authorId))
  qs.set('page', String(p.page ?? 1))
  qs.set('page_size', String(p.pageSize ?? 20))
  return request<PostList>('GET', `/posts?${qs.toString()}`)
}

export function getPost(id: number) {
  return request<Post>('GET', `/posts/${id}`)
}

export function getComments(postId: number) {
  return request<CommentTree>('GET', `/posts/${postId}/comments`)
}

export function createPost(payload: { board_id: number; topic_id?: number | null; title: string; content: string }) {
  return request<Post>('POST', '/posts', payload, { requireAuth: true })
}

export function deletePost(id: number) {
  return request('DELETE', `/posts/${id}`, undefined, { requireAuth: true })
}

export function togglePin(id: number) {
  return request('POST', `/posts/${id}/pin`, undefined, { requireAuth: true })
}

export function toggleFeature(id: number) {
  return request('POST', `/posts/${id}/feature`, undefined, { requireAuth: true })
}

// ---- 评论 ----

export function createComment(payload: { post_id: number; parent_id?: number | null; content: string }) {
  return request('POST', '/comments', payload, { requireAuth: true })
}

export function deleteComment(id: number) {
  return request('DELETE', `/comments/${id}`, undefined, { requireAuth: true })
}

// ---- 点赞 / 收藏 ----

export function like(targetType: 'post' | 'comment', targetId: number) {
  return request('POST', '/likes', { target_type: targetType, target_id: targetId }, { requireAuth: true })
}

export function unlike(targetType: 'post' | 'comment', targetId: number) {
  return request('DELETE', `/likes?target_type=${targetType}&target_id=${targetId}`, undefined, {
    requireAuth: true,
  })
}

export function favorite(postId: number) {
  return request('POST', '/favorites', { post_id: postId }, { requireAuth: true })
}

export function unfavorite(postId: number) {
  return request('DELETE', `/favorites?post_id=${postId}`, undefined, { requireAuth: true })
}

// ---- 关注 ----

export function follow(t: FollowTarget) {
  return request('POST', '/follows', t, { requireAuth: true })
}

export function unfollow(t: FollowTarget) {
  return request('DELETE', `/follows?target_type=${t.target_type}&target_id=${t.target_id}`, undefined, {
    requireAuth: true,
  })
}

// ---- 用户 ----

export function getUserProfile(id: number) {
  return request<UserProfile>('GET', `/users/${id}`)
}
