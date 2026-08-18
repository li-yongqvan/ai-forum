# Package upload — 本地上传深模块接口（Interface README）

> 依据：#12 图片上传（MVP 本地上传）、#7 深模块规范、#8 部署约定。
> **状态：已实现（2026-08-18）**。服务、HTTP 端点与测试就位。

## 范围

对外提供 `Service` 接口（`Store` / `MaxBytes`）。**无 DB、无依赖注入 seam**：图片是磁盘文件，post/comment 的 markdown 直接引用返回的 URL（#12 D5），不建表、不迁移。

调用方只有 httpapi handler；handler 负责 multipart 解析与「相对路径 → 绝对 URL」拼接，Service 负责校验 + 落盘。

## 不变量（Invariants）

- **类型双校验**（#12 D2）：扩展名白名单（jpg/png/gif/webp）+ `http.DetectContentType` MIME sniff 一致，防伪造扩展名；不满足返回 `ErrBadType`。
- **大小上限**：单文件默认 5MB（`DefaultMaxBytes`，可经 `UPLOAD_MAX_BYTES` 覆盖）；超限返回 `ErrTooLarge`。handler 层 `MaxBytesReader` 提前拦截超大请求体，Service 内二次校验兜底。
- **文件名服务端生成**（#12 D3）：`<随机 hex 32>.扩展名`，不信任用户文件名；用户文件名只取白名单扩展名，路径穿越字符不可能进入落盘路径。
- **懒创建目录**：`Dir` 不存在时 Store 自动 `MkdirAll`（生产 `/opt/ai-forum/uploads`、开发 `./data/uploads` 均由 config 默认）。

## 错误模式（Errors）

| 哨兵错误 | 语义 | 建议 HTTP |
|---|---|---|
| `ErrNoFile` | 未选择文件（handler 直接 400，Service 不感知） | 400 |
| `ErrBadType` | 扩展名不在白名单或 MIME sniff 不一致 | 400 |
| `ErrTooLarge` | 超过大小上限 | 413 |
| `ErrEmpty` | 文件内容为空 | 400 |

> 其余 error 为基础设施故障（读写失败），调用方按 500 处理。

## 必需配置（Required config）

- `Config.Dir`：存储目录。生产 `/opt/ai-forum/uploads`（nginx 托管 `location /uploads/`，#8），开发 `./data/uploads`（Go 自身托管）。
- `Config.MaxBytes`：单文件上限；`<=0` 落为默认 5MB。

## 性能与边界（#12 D5）

- 单文件读入内存后校验再一次性落盘；5MB 上限下内存占用可控。
- **不做**：缩略图、图片删除/管理页、配额/频控、额外鉴权、对象存储（B 方案留待规模升级）。
