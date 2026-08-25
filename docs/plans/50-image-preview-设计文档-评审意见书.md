# #50 图片点击放大查看（手机）——设计文档 评审意见书

> **评审对象**：《#50 图片点击放大查看（手机）——设计文档（供评审）》（`docs/plans/50-image-preview-设计文档.md`，数据时点 2026-08-23）
> **评审方式**：独立复核——本机仓库 @ `0c10359`（git HEAD 已核）为代码真值源，逐项核对 §2 声明；GitHub 侧经 `gh` CLI 复核（issue #50 / open PRs / 地图 issue #1）；Vant 行为以 `node_modules` 实际安装版 `vant@4.10.0` 源码与 `dist` 构建产物核对；`md.ts` 渲染行为用实际函数跑实（复刻 `md()` 逐规则追踪）。
> **评审结论**：**有条件通过**

---

## 一、总体结论

方向正确、证据纪律在本项目历史中属于扎实的那一档：§2 内嵌的代码真值绝大部分逐条命中，Vant API 默认值与「函数式组件样式不随按需引入加载」这个关键技术事实（§2.4/§2.5）核实为真，是全文最值钱的两条。§6.2 的「`.cbody` 互不嵌套 → 无双重触发」论证经模板 DOM 结构逐层核对成立，这条通常最容易翻车的递归组件坑，本设计处理得干净。D1–D4 与 §6 的八项决策我全部认可。

但有两类问题必须在动身前消化，都不涉及设计骨架，全部是**事实陈述与文档一致性的修复**：

1. **三处事实性出入**（F1/F2/F3）：设计文档 §5.2 的核心函数伪代码与源计划中的正确实现不一致（照 §5.2 字面写会踩 `NodeList.indexOf` 与 `images` 传元素两个坑，TS 能兜住但执行基线不该自带矛盾）；Q6 / grilling D3 的「`[![alt](img)](url)` 现在点图会跳链接」前提经实证**不成立**（当前 `md()` 永不产出 `<a><img></a>`）；§2.2「#50 尚未登记进 Roadmap」实查为假（地图 #1 已登记）。
2. **测试清单一处遗漏**（F4）：现有 `CommentTree.test.ts` 会在组件新增 util 导入后**传递加载真实 `vant`**，违反本项目「凡组件碰 vant 必 mock」的既有惯例（PostDetail/ReportSheet 均 mock），计划 §8 未覆盖。

总体评价：这是一份可以按其框架执行、值得作为执行基线使用的设计文档；把上表四类修复落地后即可动手。

---

## 二、事实与证据复核

复核范围：§2 全部内嵌真值、§3 决策依据、§5/§6 引用的机制事实，以及 Q1–Q7 声称的前提。

### 核实为真（28 项）

| 计划主张 | 复核结果 |
|---|---|
| `md.ts:39` 图片正则 / `:40` 输出裸 `<img>` 无 class | ✅ `frontend/src/utils/md.ts:39-40` 逐字命中 |
| 前端零引用 `showImagePreview`/`ImagePreview` | ✅ `grep -rn "showImagePreview\|ImagePreview" src/ e2e/` → ZERO HITS |
| `PostDetail.vue:354` 正文注入点 | ✅ `<div v-html="md(post.content)"></div>`（含 `eslint-disable` 注释） |
| `CommentTree.vue:80` 评论注入点 | ✅ `<div class="cbody" v-html="md(c.content)"></div>` |
| `Write.vue:100` 插入裸 URL + `editor.ts` 纯函数 | ✅ `insertAtCursor(content.value, start, end, url, { block: true })`；`editor.ts:17-33` |
| `global.css:109-113` `.md img` 样式 | ✅ `max-width:100%; border-radius:12px; margin:6px 0` |
| `.cbody` 互不嵌套（递归结构） | ✅ `CommentTree.vue:70-121`：`.cbody` 在 `.prow2>.cmain` 内，递归实例渲染在 `.replies`（`.prow2` 的**旁支**）；`.cbody` 与 `.replies` 永不成祖先关系 |
| `PostDetail.test.ts:18-21` vant mock | ✅ `{ showToast, showConfirmDialog }` |
| `CommentTree.test.ts:10` mock `../utils/md` | ✅ 属实 |
| Vant `showImagePreview` defaultConfig（`showIndex:true`/`closeable:false`/`closeOnClickOverlay:true`/`loop:true`/`teleport:"body"`） | ✅ `node_modules/vant/es/image-preview/function-call.mjs` 逐项命中 |
| `closeOnClickImage` 默认 true | ✅ `ImagePreview.d.ts:58-62` `{ type: BooleanConstructor; default: true }` |
| `showIndex` 默认 true 且单图也渲染页码 | ✅ `ImagePreview.mjs:75-81` `renderIndex` 在 `props.showIndex` 时无条件输出 `${active+1} / ${images.length}` |
| toast 样式入口存在但无人引用 | ✅ `vant/es/toast/style/index.mjs` 含 `../index.css`；`grep -rn "vant/es" src/` → ZERO HITS |
| image-preview 样式入口覆盖 swipe/swipe-item/image 依赖 | ✅ `vant/es/image-preview/style/index.mjs` 引入 `swipe`/`swipe-item`/`image` 等 css |
| 本地构建产物无 `.van-toast`/`.van-image-preview` 类规则 | ✅ `grep -l "\.van-toast\b" dist/assets/*.css` → NONE；`\.van-image-preview\b` → NONE |
| 唯一命中 `van-toast` 的是 CSS 变量 | ✅ `src/styles/vant-theme.css:26-27` `--van-toast-background`/`--van-toast-text-color`，非类规则 |
| §6.1「多数依赖已被 `<van-action-sheet>` 引入」 | ✅ action-sheet 实用于 3 处（`PostList.vue:203`/`ReportSheet.vue:76`/`PostDetail.vue:381`），且 dist 已含 `.van-action-sheet`/`.van-popup`/`.van-overlay`/`.van-icon` 类 |
| ImagePreview 内部自引 Swipe/Popup/Icon，函数式调用无需注册 | ✅ `ImagePreview.mjs:1-18` import 命中 |
| `images` prop 为 `string[]` | ✅ `ImagePreview.d.ts:231` |
| 本地 main HEAD = `0c10359`（= PR #48 合并） | ✅ `git log --oneline -1` |
| `components.d.ts` 被 git 跟踪（§8 `git checkout --` 有效） | ✅ `git ls-files` 命中；`git status` 无改动 |
| 现有 `md.test.ts` 无精确 `<img>` 字符串断言（加 class 不破坏现有测试） | ✅ 读 `md.test.ts` 全文件，图片断言均为 `toContain` 级 |
| issue #50 OPEN / title | ✅ `gh issue view 50` → `state=OPEN`、title 逐字吻合 |
| 仅 2 个 open PR：**#49**/**#29** | ✅ `gh pr list --state open` 命中，head 分支吻合 |
| 地图 #1 决策 9「不做缩略图/删除/配额（#12 D5）」 | ✅ `gh issue view 1` body 决策 9 逐字命中（`#12 D5`） |
| handoff / grilling 决策记录 / 源计划文档存在 | ✅ `docs/handoffs/50-image-preview.md`、`grilling-decisions/issue-50-image-preview-decisions.md`、`docs/plans/50-image-preview.md` 均在 |
| `frontend/e2e/comment-depth-smoke.spec.ts` 存在（冒烟参照） | ✅ |
| Vant 样式侧副作用导入可被 TS 解析（`vue-tsc` 不报） | ✅ `vant/es/image-preview/style/` 含 `index.d.ts`+`index.mjs`；tsconfig 无 `noUncheckedSideEffectImports` |

### 不实 / 冲突（3 项）

| 计划主张 | 复核结果 |
|---|---|
| **§2.2「#50 尚未登记进 Roadmap」** | ❌ **不实**——`gh issue view 1` body 已含 #50 两处：决策区「**#50 图片放大预览（进行中，2026-08-22 建票）**」与 Roadmap 清单「3. **#50 图片放大预览**…」。地图**已登记**。（不排除作者核验时点与统筹方更新存在先后差，但现状与声明不符，见 F3） |
| **Q6 前提「`[![alt](img)](url)` 现在点图会跳链接」** | ❌ **不实**——用实际 `md()` 逐规则跑实：图片规则（`md.ts:39`）先于链接规则（`:43`）执行，内部图片 URL 被先替换为占位符；链接规则在占位符上不匹配，`<img>` 从不落入 `<a>` 内。三种写法均不产生 `<a><img></a>`（见 F2）。 |
| **设计文档 §5.2 实现描述 vs 源计划代码** | ❌ **不一致**——§5.2 写「`images.indexOf(clicked)`」「`showImagePreview({ images, ... })`」，但 `querySelectorAll` 返回 `NodeList`（无 `.indexOf`），且 Vant `images` 需 `string[]` URL 而非元素；源计划 `docs/plans/50-image-preview.md:58-81` 的正确代码是 `Array.from(...)` + `.map(i => i.src)` + `startPosition < 0` 防御（见 F1）。 |

### 不可复核（2 项）

| 项 | 说明 |
|---|---|
| **§2.1 服务器实查**（`ssh 122.51.233.225`：容器状态、healthz） | 评审方无服务器通道，按作者自报采信；GitHub 侧已独立复核为真，服务器侧不构成执行风险 |
| **真实浏览器运行时表现**（Vant 多图滑动、toast 渲染、`naturalWidth` 行为） | 计划已自认（Q7），依赖 §8 冒烟实测——同意该安排，冒烟不可省 |

---

## 三、逐条评审

| 决策/选择 | 结论 | 评审意见 |
|---|---|---|
| D1 同一内容多图进同一预览 + 滑动 + 页码 | **认可** | 与 #50 ticket 目标一致；`indexOf` 按元素引用定位同 URL 重复图，定位正确 |
| D2 不内置「查看原图/保存」 | **认可** | 依据（#12 无缩略图 → 预览即原图）属实；长按走系统菜单是移动端标准路径 |
| D3 只响应图片、`preventDefault`、不拦截长按/右键 | **认可（附条件）** | 事件委托 + `closest('.content-img')` 正确；**条件**：`preventDefault` 的「防 `<a><img></a>` 顺带跳转」理由不实（见 F2），应改述为防御性保留 |
| D4 失败检测 → toast | **认可（附条件）** | `complete && naturalWidth===0` 口径正确；**条件**：文档标注对「0×0 合法图」的已知误判（极罕见，接受即可，见 F5） |
| §6.1 样式显式引入 + 顺带修 toast | **认可** | 最小正确方案已核实（见 §二）；把既有 toast 缺陷纳入本 PR 是 1 行低风险、与 D4 直接相关，不越范围；Vite 对 CSS 引用去重，无重复引入副作用 |
| §6.2 事件委托绑 `.cbody` | **认可** | 无双重触发论证经模板结构核对**成立**：`.cbody` 均在 `.prow2>.cmain` 内，递归实例在 `.replies`（旁支），子 `.cbody` 不经过父 `.cbody` 冒泡路径；作用域恰为「单条评论」 |
| §6.3 失败检测口径 | **认可** | `complete` 为加载终态标记、`naturalWidth===0` 表示失败，lazy 未加载图 `complete=false` 不误判——口径与浏览器语义一致 |
| §6.4 `showIndex: images.length > 1` | **认可** | 单图「1/1」是无意义噪音；多图页码是 D1 体验的一部分，偏离 Vant 默认值得 |

---

## 四、开放点裁决（Q1–Q7）

### Q1 样式引入方案 —— **认可，且纳入 toast 修复不越范围**
`import 'vant/es/image-preview/style'` + `'vant/es/toast/style'` 即 Vant 官方对函数式组件的标准引入路径，已核实为最小正确。CSS 重复引入由 Vite 构建去重消化，无副作用。「一次性修复全站 toast」依据（CSS 全局生效）已核（`dist` 全 chunk 无 `.van-toast`，仅 CSS 变量在 theme）。1 行改动、与 D4 提示直接相关，纳入本 PR 合理。

### Q2 `.cbody` 绑定无双重触发 —— **论证成立**
已按 `CommentTree.vue:63-121` 结构逐层核对：`.cbody` 是 `.prow2>.cmain` 的子元素，递归子树渲染在 `.replies`（`.prow2` 的旁支）。父 `.cbody` 不在子评论的冒泡路径上（`.replies`→父 `.comment`，不经过父 `.cmain>` 内的 `.cbody`）。**不存在**遗漏的嵌套冒泡路径；`.cbody` 内其他可点击元素（`cactions` 按钮在 `.cbody` 之外）也不经 `.cbody` 的 handler 命中图片。结论：无双重触发。

### Q3 失败检测可靠性 —— **口径可靠，jsdom 测试做法正确但需真实浏览器兜底**
`complete && naturalWidth===0` 在真实浏览器（lazy、慢网、缓存）下语义正确：加载终态（成功或失败）后 `complete=true`，失败时 `naturalWidth=0`，成功 >0；lazy 未加载 `complete=false` → 放行进预览。jsdom 中图片不加载、`complete` 恒 `false`、`naturalWidth` 恒 `0`，测试用 `Object.defineProperty` 显式定属性**不是掩盖问题，而是必要环境补偿**——但正因如此，该单测证明不了真实浏览器行为，**§8 冒烟里的「坏图点击 → toast」实测不可省**（Q7 已自认，同意）。

### Q4 `showIndex` 单图隐藏页码 —— **认可**
符合产品预期（单图「1/1」是噪音）；与 Vant 默认的偏离成本为零（一行显式传参），值得。

### Q5 范围红线 —— **认可，附 1 项测试清单补齐**
改动清单严格遵守「后端零改动 + #47 逻辑不碰 + 纯前端」，`md()` 全仓仅 `PostDetail.vue:354`/`CommentTree.vue:80` 两处消费，注入点覆盖完整。**测试清单缺一项**：现有 `CommentTree.test.ts` 需补 mock（见 F4）——组件新增 util 导入后它会传递加载真实 `vant`，违反本仓「凡碰 vant 必 mock」惯例（`PostDetail.test.ts`/`ReportSheet.test.ts:5` 均 mock）。

### Q6 `preventDefault` 行为变更 —— **前提不实，非行为变更，保留为防御性**
用实际 `md()` 逐规则跑实（复刻函数，node 追踪）：`[![a](img)](url)` 输出 `<p>[![a](<img…>)](<a href="url">url</a>)</p>`——`<img>` **不在** `<a>` 内，点图现在**不跳转、无反应**；`[<img-url>](target)` 输出 `<a>…</a>` 包裹的是未还原的占位符文本（图被丢弃）；`[text](img-url)` 输出字面 `[text](<img…>)`（图不在锚内）。**三种写法均不产生 `<a><img></a>`**，Q6 声称的「既有跳转行为变更」不存在。`preventDefault` 保留无害且面向未来（若 md.ts 日后支持标准图片链接语法），但计划/决策记录中的理由必须改述（F2）。

### Q7 未复核项声明 —— **「无未复核项」不成立**
§2.2「#50 尚未登记」实查为假（F3），且服务器实查为作者自报（评审方无通道）。其余（Vant 运行时表现）已由计划自认需冒烟实测。建议：删除「无未复核（待复核）项」表述，改为「代码/GitHub 侧已实核；服务器侧为自报；运行时表现待冒烟」。

---

## 五、新发现问题

| # | 级别 | 问题 | 要求 |
|---|---|---|---|
| F1 | **重要** | **设计文档 §5.2 与源计划实现不一致（执行基线的核心函数自带矛盾）**：§5.2 写「`container.querySelectorAll('img.content-img')` 收集…`images.indexOf(clicked)`」且第 5 步「`showImagePreview({ images, … })`」——照字面写会（a）在 `NodeList` 上调 `.indexOf`（运行时 TypeError / TS 编译错）、（b）把图片元素传给需要 `string[]` 的 `images` prop（预览渲染损坏）。源计划 `docs/plans/50-image-preview.md:72-79` 的正确代码是 `Array.from(...)` + `images.map(i => i.src)` + `startPosition < 0` 防御。设计文档是执行基线，不得与自己的源计划矛盾。 | §5.2 回填与源计划一致的精确代码（或内嵌该代码块并注明「照此实现」）：`const imgs = Array.from(container.querySelectorAll('img.content-img'))`；`const startPosition = imgs.indexOf(img); if (startPosition < 0) return`；`showImagePreview({ images: imgs.map(i => i.src), startPosition, showIndex: imgs.length > 1 })`。 |
| F2 | **重要** | **Q6 / grilling D3 / 源计划注释的前提不实**：三处都声称「`[![alt](img)](url)` 会生成 `<a><img></a>`，点图现在会跳链接」。实证：当前 `md()` 图片规则先于链接规则执行，`<img>` 从不落入 `<a>` 内（`[![a](img)](url)` → 图在锚外 + 破碎链接文本；`[<img-url>](target)` → 图被丢弃只剩占位文本；`[text](img-url)` → 字面文本+图）。**不存在既有跳转行为**，`preventDefault` 无从「改变」它。 | 修正 Q6 自评、`grilling-decisions` D3 依据、源计划 `imagePreview.ts` 内注释：`preventDefault` 改述为「防御性保留（当前 md() 不产出 `<a><img></a>`，命中即拦是面向未来的兜底；不产生行为变更）」；设计文档 §7 #2 不变量的依据同改。 |
| F3 | **重要** | **§2.2「#50 尚未登记进 Roadmap」不实**：`gh issue view 1` 决策区与 Roadmap 清单均已含 #50（「进行中，2026-08-22 建票」）。本结论不影响「本会话不 edit #1」的决策，但 §2 自称「全部可复现」的诚实度被打折。 | 更正为「#50 已登记（进行中）；本会话仍不 edit #1」；若存在核验时点差异，标注具体核对时间。 |
| F4 | **重要** | **现有 `CommentTree.test.ts` 未覆盖新的传递导入**：组件新增 `import { handleContentImageClick } from '../utils/imagePreview'` 后，该测试（当前不 mock vant）会**传递加载真实 `vant` 根模块 + 两个样式副作用导入**。本仓惯例是凡碰 vant 必 mock（`PostDetail.test.ts:18`/`ReportSheet.test.ts:5`），此遗漏大概率仍能跑绿（vitest+jsdom 可加载 vant），但违背惯例、拖慢、并埋下脆性。 | 在现有 `CommentTree.test.ts` 补 `vi.mock('../utils/imagePreview', () => ({ handleContentImageClick: vi.fn() }))`（该测试不测图片预览，mock 最干净）；或补 vant mock。§8 测试清单加这一项。 |
| F5 | 建议 | **D4 口径对「0×0 合法图」误判为失败**（`complete=true` 且 `naturalWidth=0` 的极小合法图会走 toast）。极罕见、可接受。 | 在 §6.3/§7 #7 标注为已知限制即可，不必改实现。 |
| F6 | 建议 | **图片无点击视觉暗示**：`.content-img` 无 `cursor: zoom-in`，桌面端看不出可点。移动端无碍（点击是直觉动作）。 | 可加 `.content-img { cursor: zoom-in }`（一行，进 `global.css` 或本票新增），非必需。 |
| F7 | 建议 | **设计文档 §5.2 未提 `startPosition < 0` 防御**（源计划有）：点击图若不在收集集中（理论上不可达）会开 `images[-1]` 空预览。 | 随 F1 一并回填。 |

---

## 六、通过条件清单（执行前勾选）

- [ ] **F1**：设计文档 §5.2 回填与源计划一致的精确代码（`Array.from` + `.map(src)` + `startPosition < 0`）
- [ ] **F2**：Q6 自评、`grilling-decisions` D3 依据、源计划注释中「`<a><img></a>` 跳转」的不实前提改为「防御性保留，无行为变更」
- [ ] **F3**：§2.2「#50 尚未登记」更正为「已登记（进行中）」（含核验时点标注）
- [ ] **F4**：现有 `CommentTree.test.ts` 补 `vi.mock('../utils/imagePreview')`（或 vant mock）；§8 测试清单补该项
- [ ] **F5**：§6.3/§7 #7 标注「0×0 合法图」已知误判
- [ ] 其余（D1–D4、§6.1–6.4、Q1–Q5、Q7）按计划执行；冒烟阶段「坏图点击 → toast」「多图滑动+页码」「点图关闭」实测不可省

## 七、结语

本设计的骨架（事件委托注入点、`.cbody` 作用域、样式显式引入、失败检测口径）经得起复核，绝大多数内嵌真值命中，是可信的执行基线。所提问题集中在**三处事实陈述的准确性**（F1/F2/F3）与**一处测试清单遗漏**（F4），均为低成本修复、不动骨架。建议按 §六清单修复后，以本文档作为 PR 自检表执行。

—— 评审方（独立复核：本地仓库 `0c10359` + gh CLI + `vant@4.10.0` 源码/构建产物 + `md()` 实证，2026-08-23）
