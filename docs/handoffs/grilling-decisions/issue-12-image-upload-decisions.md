# Grilling #12 图片上传 — 决策记录

- **Ticket**: [#12 图片上传（MVP 本地上传）](https://github.com/li-yongqvan/ai-forum/issues/12)
- **类型**: grilling / HITL
- **日期**: 2026-08-18
- **参与**: Claude + @li-yongqvan
- **产出**: issue #12 + Wayfinder 地图 #1 frontier 条目（本次会话只落地 ticket 定义，不实现功能）

---

## 目标

把「图片上传」作为新 ticket 挂上 Wayfinder 地图（issue #1），范围经决策确认，后续会话可据此实现。

---

## 决策清单

### D1 上传范围

- **问题**: MVP 是否落地本地图片上传？（当前 IA §7 定案「仅外链图」，手机用户无法上传照片）
- **选项**:
  - A 后端 `POST /api/v1/uploads`（multipart + 登录态）+ 本地磁盘 `/opt/ai-forum/uploads`（nginx 托管）+ 类型/大小白名单 + 前端图片选择器插入 markdown。不引第三方存储。
  - B 引入对象存储（OSS/S3）。
  - C 维持仅外链，不开 ticket。
- **决定**: **A 本地上传**
- **理由**: 与 #4/#8 单服务器、少依赖一致；最小闭环。50 人规模引对象存储过度。

### D2 落地形态

- **问题**: ticket 怎么挂到地图 #1？
- **选项**: A 新建独立 GitHub issue + 地图条目 / B 仅地图 frontier 条目不开 issue
- **决定**: **A 独立 issue**（#12）
- **理由**: 沿用 wayfinder「每 ticket 一 issue」约定，可独立跟踪/决议/关闭。

### D3 地图更新方式

- **问题**: 如何更新 Wayfinder 地图（issue #1）？
- **选项**: 编辑 issue #1 body / 新增 comment
- **决定**: **编辑 body**
- **理由**: 地图自洽，单源一致；避免决策漂移。

---

## 关键讨论点

### 1. 升级 IA §7 的一致性

- 原定案「仅外链渲染」由 md 渲染器支持外链图；本地上传是超集（本地上传 + 外链），不推翻 md 渲染器，只在 `docs/ux/info-architecture.md` §7 同步升级说明。
- 前端 md 渲染器 `frontend/src/utils/md.ts` 已支持 `https://…png|jpg|gif|webp`，无需改动即可渲染本地上传后的 URL。

### 2. 依赖关系

- 无硬阻塞；实现复用 #8 部署约定（nginx 托管静态 + `/api` 反代）与 #9 IA（发帖/评论表单结构）。
- 后端基线 PR #11（`feat/backend-scaffold`）。

---

## 后续行动

- [x] 新建 issue #12（含范围/依赖/里程碑）
- [x] Wayfinder 地图 issue #1 加 frontier 条目 + blocking 图节点
- [x] 同步 `docs/handoffs/INDEX.md` 活跃表
- [x] 删除 image-upload-ticket.md 工作底稿（handoff 非持久化约定）
- [ ] 实现 `POST /api/v1/uploads`（由后续会话按 #12 执行）
- [ ] 前端发帖/评论图片选择器 + 上传态反馈
- [ ] 同步升级 `docs/ux/info-architecture.md` §7
