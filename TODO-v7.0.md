# TODO v7.0 - 前端性能与架构优化

基于 Vercel React Best Practices 的代码审查，本项目存在以下 5 个需要改进的地方：

## 1. 数据获取瀑布流 (Data Fetching Waterfalls) - `CRITICAL` (已完成)
- **相关规则：** `async-waterfall` / `client-swr-dedup`
- **发现位置：** `TeacherDashboardPage` 和 `KnowledgeMapPage`
- **问题描述：** 页面组件中存在串行等待的数据获取模式（先获取课程列表，再根据选中的课程获取雷达图、活动等数据），导致首屏渲染经历多次 Loading 闪烁，极大增加 LCP 时间。
- **改进方案：** 引入 `SWR`（或 React Query），消除客户端瀑布流请求，并利用其缓存机制优化体验。

## 2. 关键的 React Ref 与生命周期 Bug - `CRITICAL` (已完成)
- **相关规则：** `advanced-event-handler-refs`
- **发现位置：** `VoiceInput` (以及 `Avatar3D` 存在的相同隐患)
- **问题描述：** `useEffect` 中直接依赖了 `wsRef.current` 进行 `addEventListener` 绑定。由于 React 不会追踪 Ref 的突变，如果在挂载时 WebSocket 尚未连接，Effect 将直接返回且永远不会重试，导致组件无法接收消息。
- **改进方案：** 改为通过向下传递 `agentChannel` 对象（封装好的稳定接口）进行事件订阅，并在卸载时正确移除监听器。

## 3. 组件级对象的内联创建 (Hoisting JSX Objects) - `MEDIUM` (已完成)
- **相关规则：** `rendering-hoist-jsx` / `rerender-memo`
- **发现位置：** `MarkdownRenderer`
- **问题描述：** `ReactMarkdown` 的 `components` 和 `remarkPlugins` 属性是直接在渲染函数内创建的字面量对象和数组。在流式输出触发高频更新时，会导致内部组件每次都被迫重新评估和构建。
- **改进方案：** 将不依赖于内部状态的配置对象提取（Hoist）到组件外部作为全局常量。

## 4. 长列表流式渲染性能 - `MEDIUM` (已完成)
- **相关规则：** `rerender-memo` / `rendering-content-visibility`
- **发现位置：** `MessageList`
- **问题描述：** 在 AI 流式输出期间，`streamingContent` 频繁更新会触发整个 `MessageList` 的重新渲染，所有历史消息的沉重 Markdown 视图也会随之重新渲染，导致长对话卡顿。
- **改进方案：** 抽取独立的 `MessageBubble` 组件并使用 `React.memo` 包裹；结合 `content-visibility: auto` 优化视口外元素的渲染计算。

## 5. API Client 缺乏防抖与缓存机制 - `MEDIUM-HIGH` (已完成)
- **相关规则：** `client-swr-dedup`
- **发现位置：** `frontend/src/lib/api.ts`
- **问题描述：** 当前单纯使用 `fetch` 结合 `useEffect` 容易导致同一接口的重复请求，缺乏针对 GET 请求的高级缓存和重新验证机制。
- **改进方案：** 在项目中安装并整合 `swr`，针对读操作提供数据缓存和去重。
