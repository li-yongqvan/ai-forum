# Grilling #32 通知中心 — 决策记录

- **Ticket**: [#32 通知中心：补齐通知 API 与页面](https://github.com/li-yongqvan/ai-forum/issues/32)
- **类型**: grilling / HITL
- **日期**: 2026-08-20
- **参与**: Claude + @li-yongqvan
- **产出**: 本票实现范围定稿（收窄为**仅关注通知**），随后按此实现

---

## 目标

完成 #32 第 0 步 grilling，锁定通知触发范围与交互语义，再据此实现（SOP §3）。

---

## 决策清单

### D1 通知触发范围（大幅收窄）

- **问题**: 本票实现哪些触发点？schema 5 类（follow/like/comment/reply/report_result）均已建。
- **选项**:
  - A 全量：like→帖主、comment→楼主、reply→被回复者、follow(用户)→被关注者
  - B **仅关注**：follow(用户)→被关注者；like/comment/reply **不生成通知**
- **决定**: **B**
- **理由**: 点赞/评论/回复，帖子作者**上线自己看即可**，无需打扰；只有关注无法被被关注者自然感知，值得主动通知。like/comment/reply 触发点留待后续按需补（schema 已留余地）。report_result 明确留给「举报闭环 #33」。

### D2 去重策略

- **决定**: **不做去重**。同人反复触发各写一条，MVP 从简（接口最简单，先跑通闭环）。

### D3 私信 tab

- **决定**: **占位**。「通知/私信」分段页只接「通知」段，私信 tab 显示「即将上线」空态；messages 能力后续排票。

### D4 自触发

- **决定**: **不通知自己**。关注场景由 user 服务层拒自关（`ErrSelfFollow`）天然覆盖，无需 handler 额外判断。

### D5 已读语义 / 跳转锚点

- 已由脚手架接口（is_read 布尔 + unread_count + 单条/全量两个接口）+ IA v2 §4 定死，照做：follow→用户主页、like/comment/reply→帖子详情评论锚点、report_result→结果提示。

---

## 后续行动

- [x] 实现 notify 域（gorm_repo + service 实现）、通知 API 路由、关注触发点（handler 层跨包聚合）、前端通知页接线 + Tab badge
- [x] 测试：notify service 单测 + handler 集成（Docker）+ 前端 `Notifications.test.ts`
- [ ] PR → 用户授权合并 → CI 部署 → 服务器验证 → 统筹方更新地图 #1
