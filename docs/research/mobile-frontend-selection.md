# 移动端前端技术方案调研（Issue #3）

> 本文档是 [ai-forum issue #3「[research] 移动端前端技术方案调研」](https://github.com/li-yongqvan/ai-forum/issues/3) 的研究结论。
> 完成本调研后**不关闭 issue**，供 #1 Wayfinder 地图与 #6 原型 ticket 引用。
> 所有结论均引用官方文档、GitHub repo、官方案例等第一手来源，出处见文末[参考来源](#8-参考来源)。

---

## 0. TL;DR（结论摘要）

| 问题 | 结论 |
|---|---|
| 方案 | **PWA（可安装的移动端 Web SPA）**，技术栈 **Vue 3 + Vite + Vant + vite-plugin-pwa** |
| 为什么不选跨端框架 | Flutter / React Native 需要应用商店或侧载分发，对「社团内部」是多余摩擦；Flutter 官方明确不推荐用于文本流式内容（论坛正是此类） |
| 为什么不选 uni-app / Taro | 价值在「一套代码发微信小程序等多端」，而本需求明确只要 **PWA / 移动端 Web**；引入 DCloud / JD 编译层抽象得不偿失 |
| 是否需要 SSR/SSG | **不需要**。登录墙 + 内部使用 + 无 SEO 需求；Vue 官方明确「只有需要 SSR 才用框架，否则直接用 Vite」 |
| 是否需要离线缓存 | **需要轻量级**：用 Workbox 预缓存 app shell（JS/CSS/HTML），提升二次加载与弱网体验；**不需要完整离线读写** |
| 与 Go 后端通信 | **RESTful JSON API（OpenAPI 3.1 规范）**。gRPC 只用于 Go 服务间内部调用，浏览器前端不用 gRPC-Web；GraphQL 复杂度大于收益 |
| 明确推荐组合 | **Vue 3 (TS) + Vite + Vue Router + Pinia + Vant + vite-plugin-pwa**；前端 REST → Go 网关/服务；构建产物为纯静态文件，nginx 或 Go 内嵌托管，Docker Compose 部署 |

---

## 1. 背景与约束（来自第一手上下文）

- **Wayfinder 地图 [#1](https://github.com/li-yongqvan/ai-forum/issues/1)** 明确：终端「**手机端优先，前端为 PWA 或移动端 Web**」；部署「用户自有服务器，需要 Docker Compose 或类似方案」；技术学习导向「用 Go + 微服务架构实践」。
- **[forum-wayfinder-input.md](../../docs/forum-wayfinder-input.md)** 明确非功能约束「移动端优先」，目标规模「万级注册、千级日活、十万级帖子」，且**明确不做**实时音视频、IM 主场景、直播、支付。
- **MVP 功能集**（#1）：发帖/看帖/评论、关注用户/板块/话题、私信/通知、用户中心；治理极简（举报 + 管理员删帖/封号）。
- 关键推论：这是一个**登录墙 + CRUD 为主 + 内部成员使用**的内容社区。它不是公开 SEO 站点，不需要原生设备能力（相机/GPS 等），也不需要上架应用商店。

---

## 2. 方案对比：PWA vs 移动端 Web vs 跨端框架

### 2.1 决策前提：为什么先排除「原生/跨端框架」一类

**分发成本是决定性因素。** 跨端框架（Flutter、React Native/Expo）产出的原生 App 主要经应用商店分发：

- [Expo 官方分发文档](https://docs.expo.dev/distribution/introduction/)：主要路径是「提交到 Google Play / Apple App Store」；内部测试走 AdHoc/TestFlight（[internal-distribution](https://docs.expo.dev/build/internal-distribution/)）。没有「给链接即用」的零摩擦方式。
- 对「社团内部使用」而言，让成员去装 TestFlight/侧载 APK、或等应用商店审核，是**纯摩擦**；而 Web 一个 URL 打开即用，天然契合。

**Flutter 官方对「论坛类内容」的明确态度**（[Flutter web 文档](https://docs.flutter.dev/platform-integration/web)）：

> "Not every HTML scenario is ideally suited for Flutter at this time. For example, **text-rich, flow-based, static content such as blog articles** benefit from the document-centric model that the web is built around, rather than the app-centric services that a UI framework like Flutter can deliver."

论坛（帖子、评论、树状回复）正是「text-rich, flow-based」内容——Flutter 官方自己指出这类内容应交给 Web 的 document 模型，而非 UI 框架。这是排除 Flutter 的最直接依据。

**学习曲线 / 维护成本**：Flutter 需要 Dart 语言 + 原生构建工具链；React Native 需要 React + 原生工程（Gradle/Xcode）。对「开发维护人员有限」的社团，每引入一种语言/工具链都是长期维护负担。后端已经是学习重点（Go 微服务），前端应选择**最「无聊」、最稳**的方案，而不是第二个学习项目。

### 2.2 PWA 与移动端 Web 的关系

PWA 不是另一种技术，而是「移动端 Web + 三个增强」：[web.dev Learn PWA](https://web.dev/learn/pwa) 定义其核心为 **Web App Manifest**（可安装、主屏图标、独立窗口）+ **Service Worker**（离线、加载加速、推送）+ **HTTPS**。

| 维度 | 普通移动端 Web | PWA（本推荐） |
|---|---|---|
| 访问方式 | 浏览器输入 URL | 同上 + 可「添加到主屏」成独立应用 |
| 离线/弱网 | 无 | Service Worker 缓存 app shell，二次打开接近秒开 |
| 推送 | 无 | Android Chrome 全量支持；iOS Safari 16.4+ 支持（需添加到主屏，见 §5） |
| 开发成本增量 | — | 极小：vite-plugin-pwa 零配置即可注入 manifest + 生成 Service Worker |

对移动端优先的论坛，**PWA 是移动端 Web 的免费增强**——多付出的只是一份 manifest 和一个 service worker 配置文件。

### 2.3 六种候选横向对比

| 方案 | 学习曲线 | 开发成本 | 维护成本 | 移动端体验 | 分发 | 适合本场景？ |
|---|---|---|---|---|---|---|
| **Vue 3 + Vite（PWA）** | 低（HTML/CSS/JS 即可上手，中文资料最全） | 低 | 低（单一代码库，纯静态部署） | 接近原生（移动 Web） | URL + 添加到主屏 | ✅ **推荐** |
| React + Vite（PWA） | 中（JSX/hooks） | 低-中 | 低-中 | 接近原生 | URL + 主屏 | 🟡 可行备选 |
| uni-app（Vue 语法） | 低 | 中（引入 DCloud 编译层） | 中 | H5 一般（主要优化小程序） | 小程序/H5/App 多端 | 🟡 仅当要发微信小程序 |
| Taro（React/Vue） | 中 | 中（JD 编译层） | 中 | H5 一般 | 小程序/H5/RN 多端 | 🟡 同上 |
| Flutter | 高（Dart） | 高 | 高 | 原生级 | 应用商店/侧载 | ❌ 官方不推荐文本流式内容 |
| React Native / Expo | 中-高 | 高 | 高 | 原生级 | 应用商店/TestFlight | ❌ 分发摩擦大 |

### 2.4 为什么 uni-app / Taro 也不推荐（MVP 阶段）

- 两者的核心价值是「**一套代码发布多端**」，尤其微信小程序（[uni-app 官方](https://uniapp.dcloud.net.cn/)「一套代码可发布到 iOS、Android、Web、以及各种小程序」；[Taro 官方](https://docs.taro.zone/docs/) 支持 H5/RN/任意小程序平台）。
- 而本需求（#1）已明确**只要 PWA / 移动端 Web，无小程序要求**。为「未来可能的微信小程序」现在付出 DCloud/JD 编译层抽象、H5 输出降级、PWA/Service Worker 支持弱、组件模型与标准 Web 不同等代价，属于过度设计。
- 若日后确需微信小程序，**uni-app 是届时的一个候选**，但那应是一个独立决策，不应让 MVP 前端被多端编译层绑架。

---

## 3. 最小成本交付可用的移动端体验：为什么是 Vue 3 + Vite + Vant 的 PWA

### 3.1 技术栈组成与依据

| 组件 | 选型 | 第一手依据 |
|---|---|---|
| 框架 | **Vue 3（Composition API + TypeScript）** | [Vue 官方](https://vuejs.org/guide/introduction)定位「渐进式框架，灵活、可增量采纳」，学习曲线平缓（官方称只需 HTML/CSS/JS 基础即可跟随教程） |
| 构建工具 | **Vite** | [Vue 官方 quick-start](https://vuejs.org/guide/quick-start)：`npm create vue@latest` 默认即 Vite；官方明确「**The general recommendation is to use a framework only if you need SSR. If you don't need SSR, you can simply use Vite**」 |
| 路由/状态 | Vue Router + Pinia | [create-vue](https://vuejs.org/guide/quick-start) 脚手架可选集成 |
| 移动端 UI 库 | **Vant** | [Vant（Youzan 维护，GitHub 24.4k stars，MIT）](https://github.com/youzan/vant)：「a lightweight, customizable Vue UI library for **mobile web apps**」，80+ 组件、平均单组件约 1KB（min+gzip）、零第三方依赖、TS 编写、支持 Vue 3 |
| PWA 增强 | **vite-plugin-pwa** | [官方](https://vite-pwa-org.netlify.app/)：零配置、框架无关，基于 Workbox 生成 Service Worker + 自动注入 Web App Manifest + 离线支持 |
| 部署 | 纯静态产物 → nginx 或 Go 服务内嵌/托管静态文件 | 与 #1 的 Docker Compose 自托管约束吻合；Vite 构建输出静态文件 |

### 3.2 逐条对照约束

- **移动端优先**：Vant 是专为移动 Web 设计的组件库（vs 通用 PC 组件库），单组件体积小、触控交互开箱即用；配合 viewport meta 与响应式布局即手机优先。
- **开发/维护人员有限**：Vue 3 学习曲线最低，中文文档与社区体量在候选人里最大；单一代码库、无原生工程、无多端编译层。
- **最小交付成本**：`create-vue` 一步脚手架；`vite-plugin-pwa` 一步加 PWA；静态文件部署零服务端渲染逻辑。后端学习已是重点，前端保持最简。
- **兜底备选**：若团队实际更熟 React，等价推荐 **React + Vite + Ant Design Mobile + vite-plugin-pwa**——结论与通信方式不变，仅组件层语言不同。

---

## 4. 是否需要 SSR/SSG？是否需要离线缓存？

### 4.1 SSR/SSG：不需要

- **场景判断**：登录墙（开放注册，内容主要面向注册用户）、社团内部使用、无公网 SEO / 爬虫收录需求。SSR 的价值（首屏 SEO、社交分享卡片）在此场景基本为零。
- **官方佐证**：Vue 官方明确「只有需要 SSR 才用框架，否则直接用 Vite」（§3.1）；[React 官方「开始新项目」](https://react.dev/learn/start-a-new-react-project)也把从零建 SPA（Vite）定位为「有特殊约束、想自己搭框架、或只想学 React 基础」时选择，且 React 框架（Next.js）的价值同样集中在 SSR/Server Components 这类后端能力。
- **结论**：MVP 用 **SPA + 纯静态托管** 最简单、最便宜。若将来需要一个对外公开的落地页/介绍页，单独做一个轻量静态页（如 VitePress）即可，与应用解耦，不必为此引入 SSG 框架。

### 4.2 离线缓存：需要轻量级（app shell 预缓存）

- **做**：用 Workbox 预缓存 app shell（HTML/JS/CSS 与图标）。收益有官方案例支撑——[Twitter Lite PWA（web.dev 官方案例）](https://web.dev/case-studies/twitter)：Service Worker 缓存静态资源与应用壳后，**回头客 <3 秒启动**、数据用量约 600KB（原生 Android 版为 23.5MB）、页面/会话 +65%、推文 +75%、跳出率 −20%。对社团成员在校园弱网/流量场景下体验提升明显。
- **不做**：完整离线读写。论坛内容服务器驱动，离线无法看新内容/发帖，离线缓存做到「app shell 可用 + 已读静态资源可回放」即可。
- **注意**：私信/通知的「实时性」不应依赖 PWA 推送（见 §5 推送限制），MVP 用应用内通知中心 + 轮询/WebSocket 即可，离线缓存不承担实时职责。

---

## 5. 与 Go 后端交互：RESTful API vs gRPC-Web vs GraphQL

### 5.1 对比表

| 方案 | 实现复杂度 | 浏览器直连 | 与 Go 后端匹配 | 适合 MVP？ |
|---|---|---|---|---|
| **RESTful JSON** | 低 | 原生 | `net/http` / Gin 直接 JSON 序列化 | ✅ **推荐** |
| gRPC-Web | 高 | **需代理**（默认 Envoy）+ protobuf 代码生成；TS 支持实验性；无双向流 | grpc-go 不原生支持 Web，必须加代理 | ❌ 前端不用 |
| GraphQL | 中-高 | 原生（HTTP/JSON） | 需 gqlgen 等 schema-first 代码生成 + 客户端缓存层 | ❌ 复杂度 > 收益 |

### 5.2 RESTful JSON API：推荐理由

- **浏览器原生、零胶水**：Go 侧 `net/http` 或 Gin 直接 `json.Marshal`/`json.NewEncoder` 出 JSON，前端 `fetch`/axios 直用。调试用 curl / 浏览器 DevTools 即可，对有限开发人员最友好。
- **与「Go 微服务学习」架构自洽**：[#1](https://github.com/li-yongqvan/ai-forum/issues/1) 要求「各业务模块数据隔离，服务间只走内部 API」——**服务间内部调用可用 gRPC（学习价值高、性能好），浏览器前端走 REST 到 Go 网关/API**。这是微服务架构的标准形态：对外 REST、对内 RPC。
- **可演进**：REST 端点用 **OpenAPI 3.1** 描述，可自动生成前端类型定义与文档；将来如需再叠加 GraphQL 网关也不破坏 REST 基线。

### 5.3 gRPC-Web：为什么前端不用

- [官方 gRPC-Web repo](https://github.com/grpc/grpc-web) 明言：浏览器无法直接讲原生 gRPC（缺 HTTP/2 API），**必须经特殊代理，默认 Envoy**；`grpc-web` 客户端的 **TypeScript 支持是实验性的**；**客户端流与双向流不支持**（仅 unary + 服务端流，且服务端流仅限 `grpcwebtext` 模式）。
- Go 生态的社区 gRPC-Web 代理 [improbable-eng/grpc-web](https://github.com/improbable-eng/grpc-web) 已进入 **maintenance mode**，README 建议迁移到官方 grpc-web 客户端——进一步说明浏览器端 gRPC 生态仍不稳定。
- 结论：gRPC-Web 的额外成本（protobuf、代码生成、Envoy 代理、实验性 TS）远大于对论坛 CRUD 的收益，**gRPC 留给 Go 服务间内部调用即可**。

### 5.4 GraphQL：为什么不推荐

- [gqlgen](https://gqlgen.com/)（99designs 维护）虽把 Go 侧开发做得顺滑（schema-first + 代码生成、强类型），但整套方案要求：学习 GraphQL 查询语言、定义 Schema、写 Resolver、前端引入 Apollo/urql 客户端与缓存层。
- 论坛 MVP 是**简单 CRUD**（帖子/评论/点赞/关注/私信），每个资源一个 REST 端点即覆盖，几乎不存在 REST 的 over/under-fetching 痛点（那才是 GraphQL 的主场）。为一个不需要弹性查询的场景引入查询语言 + 双层缓存，是纯复杂度。

### 5.5 推送通知的现实限制（影响「私信/通知」设计）

- **Android**：Chrome 全量支持 Web Push，无需安装到主屏。
- **iOS**：[Apple 官方文档](https://developer.apple.com/documentation/UserNotifications/sending-web-push-notifications-in-web-apps-and-browsers) 明确：iOS/iPadOS **16.4+** 起 Safari 支持 Web Push，但**仅限「添加到主屏」的 Web App**，普通 Safari 标签页不触发；设备低电量模式/关机时投递不可靠。
- 推论：若社团成员 iPhone 用户居多且未「添加到主屏」，推送到达率无法保证。因此 **MVP 的私信/通知以「应用内通知中心 + 轮询/WebSocket」为主**，Web Push 作为可选增强，不阻塞 MVP。

---

## 6. 明确推荐组合

```
┌────────────────────────────── 前端（PWA / 移动端 Web） ──────────────────────────────┐
│  Vue 3 (Composition API + TypeScript)                                                │
│  + Vite（构建）  + Vue Router（路由）  + Pinia（状态）                                 │
│  + Vant（移动端组件库）  + vite-plugin-pwa（Manifest + Workbox Service Worker）       │
│  产物：纯静态文件（HTML/JS/CSS）                                                       │
└───────────────────────────────────────┬──────────────────────────────────────────────┘
                                        │ RESTful JSON API（OpenAPI 3.1）
                                        ▼
┌────────────────────────────── 后端（Go，来自 #2/#4）─────────────────────────────────┐
│  Go API 网关/服务：net/http 或 Gin，JSON 序列化                                      │
│  服务间内部调用：gRPC（学习微服务用，浏览器不接触）                                    │
└─────────────────────────────── Docker Compose 自托管部署 ─────────────────────────────┘
```

- **前端框架**：Vue 3 + Vite + Vant，以 **PWA** 形式交付（vite-plugin-pwa）。备选：React + Vite + Ant Design Mobile（结论等价）。
- **与后端通信**：**RESTful JSON API**；gRPC 仅用于 Go 服务间内部调用；不引入 gRPC-Web / GraphQL。
- **SSR/SSG**：不引入；公开落地页未来可单独做轻量静态页。
- **离线**：Workbox 预缓存 app shell；不做完整离线。
- **部署**：前端静态文件 → nginx 或 Go 内嵌静态托管，Docker Compose 一键启动。

### 理由一句话

「**社团内部 + 移动端优先 + 人手有限 + 后端已是 Go 学习重点**」这四个约束叠加，指向**最小摩擦**的路径：无需应用商店的 PWA、无需 SSR 的纯静态 SPA、无需代理与代码生成的 REST，把团队的注意力集中在业务与 Go 微服务学习上。

---

## 7. 风险与开放问题

- **iOS 推送到达率**：取决于成员是否「添加到主屏」，见 §5.5。MVP 应把通知中心做扎实，不依赖推送。
- **团队技术倾向**：若团队 React 经验远强于 Vue，换 React + Vite + Ant Design Mobile 不改变本调研其余结论。
- **未来微信小程序**：如社团后续要求小程序分发，可再评估 uni-app（届时它才体现价值），属独立决策。
- **#4/#5 尚未定论**：本调研的 REST 推荐对后端框架（Gin/Echo/标准库）与服务拆分结论不敏感；实际网关形态待 #4 产出后对齐。

---

## 8. 参考来源

**项目上下文（第一手）**
- [ai-forum #1 Wayfinder 地图](https://github.com/li-yongqvan/ai-forum/issues/1)
- [ai-forum #3 本调研 ticket](https://github.com/li-yongqvan/ai-forum/issues/3)
- `docs/forum-wayfinder-input.md`（本仓库）

**PWA / 案例**
- [web.dev — Learn PWA（Manifest / Service Worker / 可安装性）](https://web.dev/learn/pwa)
- [web.dev 官方案例 — Twitter Lite PWA（数据用量 600KB vs 原生 23.5MB、回头客 <3s、+65% 页面/会话、+75% 推文、−20% 跳出率）](https://web.dev/case-studies/twitter)
- [Apple Developer — Sending web push notifications in web apps and browsers（iOS/iPadOS 16.4+，仅主屏 Web App）](https://developer.apple.com/documentation/UserNotifications/sending-web-push-notifications-in-web-apps-and-browsers)

**框架 / 构建 / 组件**
- [Vue.js — 官方介绍（渐进式框架）](https://vuejs.org/guide/introduction)
- [Vue.js — Quick Start（create-vue / Vite / 「仅需要 SSR 才用框架」）](https://vuejs.org/guide/quick-start)
- [Vite — 官方文档（SPA / SSR / 构建）](https://vite.dev/guide/)
- [Vant — 官方 README（Vue 移动端组件库，Youzan，80+ 组件，平均 1KB）](https://github.com/youzan/vant)
- [vite-plugin-pwa — 官方（零配置、Workbox、Manifest 注入）](https://vite-pwa-org.netlify.app/)
- [React — Start a New React Project（框架 vs Vite 从零建 SPA）](https://react.dev/learn/start-a-new-react-project)
- [Flutter — Web 支持（官方对文本流式内容的适用性说明）](https://docs.flutter.dev/platform-integration/web)
- [Expo — Distribution Introduction（应用商店为分发主路径）](https://docs.expo.dev/distribution/introduction/)
- [Expo — Internal Distribution（AdHoc/TestFlight 内部测试）](https://docs.expo.dev/build/internal-distribution/)
- [uni-app — 官方（DCloud，一套代码多端）](https://uniapp.dcloud.net.cn/)
- [Taro — 官方（京东凹凸实验室，多端）](https://docs.taro.zone/docs/)

**前后端通信**
- [gRPC-Web — 官方 repo（浏览器需 Envoy 代理、TS 实验性、无双向流）](https://github.com/grpc/grpc-web)
- [improbable-eng/grpc-web — Go gRPC-Web 代理（maintenance mode，建议迁移官方客户端）](https://github.com/improbable-eng/grpc-web)
- [gqlgen — 官方（Go GraphQL 服务器，schema-first 代码生成）](https://gqlgen.com/)
- [Go — net/http 包文档（HTTP 服务端/客户端）](https://pkg.go.dev/net/http)
