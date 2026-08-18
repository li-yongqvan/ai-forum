-- 0005 content 种子数据
-- MVP 无管理 UI，板块/话题由 DB 直管（同邀请码模式，auth-flow §7）。数据沿用 #6 原型 AI 社团语境。
INSERT INTO content.boards (id, name, description, parent_id, sort_order) VALUES
  (1, '学习讨论', '课程、教程与学习方法', NULL, 1),
  (2, '项目实战', '代码、框架与工程实践', NULL, 2),
  (3, '论文速递', '论文共读与解读', NULL, 3),
  (4, '竞赛组队', '数模、Kaggle、Hackathon', NULL, 4),
  (5, '活动公告', '工作坊、讲座与招新', NULL, 5),
  (6, '闲聊灌水', '面经、日常与树洞', NULL, 6);

INSERT INTO content.topics (id, board_id, name) VALUES
  (1, 1, 'LLM'),
  (2, 1, 'RAG'),
  (3, 2, 'Agent'),
  (4, 2, 'PyTorch'),
  (5, 4, '数模竞赛'),
  (6, 3, 'Transformer'),
  (7, 1, '微调'),
  (8, 1, '提示工程'),
  (9, 5, '工作坊'),
  (10, 6, '求职面经');
