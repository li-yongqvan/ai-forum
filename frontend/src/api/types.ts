// 与后端 REST JSON 对应的类型（v2 §3.2 D6；viewer 字段见 v2 §4）
export type Role = 'guest' | 'user' | 'moderator' | 'admin'

export interface AuthUser {
  id: number
  username: string
  email: string
  role: Role
  avatar_url: string | null
}

export interface AuthResult {
  token: string
  user: AuthUser
}

export interface FollowViewer {
  following: boolean
}

export interface Board {
  id: number
  name: string
  description: string | null
  viewer?: FollowViewer
}

export interface Topic {
  id: number
  board_id: number
  name: string
  viewer?: FollowViewer
}

export interface PostViewer {
  liked: boolean
  favorited: boolean
  following_author: boolean
}

export interface Post {
  id: number
  board_id: number
  board_name: string
  topic_id: number | null
  topic_name: string | null
  author_id: number
  author_name: string
  author_avatar: string | null
  title: string
  content: string
  is_pinned: boolean
  is_featured: boolean
  view_count: number
  like_count: number
  comment_count: number
  favorite_count: number
  viewer?: PostViewer
  created_at: string
}

export interface CommentNode {
  id: number
  post_id: number
  author_id: number
  author_name: string
  parent_id: number | null
  floor: number | null
  content: string
  deleted: boolean
  created_at: string
  replies?: CommentNode[]
}

export interface CommentTree {
  post_id: number
  comments: CommentNode[]
}

export interface PostList {
  items: Post[]
  page: number
  page_size: number
}

export interface UserProfile {
  id: number
  username: string
  avatar_url: string | null
  bio: string | null
  joined_at: string
  post_count: number
  follower_count: number
  following_count: number
  viewer?: FollowViewer
}

export type FollowTargetType = 'user' | 'board' | 'topic'

export interface FollowTarget {
  target_type: FollowTargetType
  target_id: number
}
