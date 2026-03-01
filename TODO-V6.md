# Hanfledge V6.0 — Teacher Takeover (Human-in-the-loop)

## 目标
实现“教师接管”机制：教师在 Dashboard 查看学生会话时，能够实时介入。如果学生卡住，教师可以：
1. **Takeover (接管/直接回复)**：教师直接发消息给学生，显示为“教师”角色，暂停或跳过该轮的 AI 回复。
2. **Whisper (悄悄话/指令)**：教师给 AI 下达隐藏指令（例如“换一种更简单的比喻”），AI 收到后根据指令生成下一条发给学生的消息。

## Backend Tasks
- [ ] `internal/delivery/http/handler/session.go`: 
  - 增加 `ActiveSessions` map，记录 `sessionID -> *wsConn`
  - 新增 API: `POST /api/v1/sessions/:id/intervention` (body: `{ type: "takeover" | "whisper", content: "..." }`)。
- [ ] `internal/domain/model/session.go` & `internal/agent/types.go`: 
  - 增加对 `role: "teacher"` 的支持
- [ ] `internal/agent/orchestrator.go` & `coach.go`: 
  - 能够处理教师注入的指令。如果类型是 whisper，将指令注入上下文；如果是 takeover，直接将消息存入 Interaction 并下发给学生。

## Frontend Tasks
- [ ] `frontend/src/app/teacher/dashboard/session/[id]/page.tsx`:
  - 增加一个“干预面板” (Intervention Panel)，只有在会话为 `active` 状态时显示。
  - 输入框 + "接管回复" (Takeover) 和 "私语AI" (Whisper) 按钮。
- [ ] `frontend/src/app/student/session/[id]/components/MessageList.tsx`:
  - 支持渲染 `role: 'teacher'` 的消息，使用不同的样式（如蓝色高亮、教师头像）。
- [ ] `frontend/src/lib/api.ts`: 
  - 添加 `sendIntervention(sessionId, type, content)`。

## Batching & AI-Recommended Skill Mounting
- [x] Backend: Add `LLMProvider` to `SkillHandler` deps via `RouterDeps`
- [x] Backend: Add `POST /api/v1/courses/:id/skills/recommend` handler using LLM.
- [x] Backend: Add `POST /api/v1/courses/:id/skills/batch-mount` handler.
- [x] Frontend: Add UI for AI Recommendation Modal in Outline Page.
- [x] Frontend: Handle state for selected mounts and batch applying them.
