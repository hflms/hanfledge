# 学习活动自动开始功能

## 功能描述

学生进入学习活动时,系统自动根据 skill 安排开始活动,无需等待学生手动发送第一条消息。

## 实现方案

### 触发条件

1. **新会话检测**
   - 会话状态为 `active`
   - 无历史交互记录 (`interactions.length === 0`)

2. **WebSocket 就绪**
   - WebSocket 连接状态为 `connected`
   - 不在发送中状态 (`!sending`)

### 执行流程

```
1. 学生加入活动
   └─> 创建新会话 (JoinActivity API)

2. 前端加载会话
   └─> getSession() 获取会话数据
       └─> 检测: 无历史消息 + 状态 active
           └─> 标记: autoStartTriggeredRef = true

3. WebSocket 连接建立
   └─> wsStatus = 'connected'
       └─> 检测: autoStartTriggeredRef = true
           └─> 自动发送: { event: 'user_message', payload: { text: '开始学习' } }
           └─> 重置标记: autoStartTriggeredRef = false
```

## 技术实现

### 前端实现

**文件:** `frontend/src/app/student/session/[id]/page.tsx`

#### 1. 添加自动开始标记

```typescript
const autoStartTriggeredRef = useRef(false);
```

#### 2. 会话加载时检测

```typescript
useEffect(() => {
    const loadSession = async () => {
        const data = await getSession(sessionId);
        setSession(data.session);
        setMessages(existingMessages);

        // 检测新会话
        if (existingMessages.length === 0 && 
            data.session.status === 'active' && 
            !autoStartTriggeredRef.current) {
            autoStartTriggeredRef.current = true;
            console.log('[SESSION] 新会话,准备自动开始学习活动');
        }
    };
    loadSession();
}, [sessionId, router, toast]);
```

#### 3. WebSocket 就绪时自动发送

```typescript
useEffect(() => {
    if (autoStartTriggeredRef.current && 
        wsStatus === 'connected' && 
        !sending) {
        console.log('[SESSION] WebSocket 已连接,自动发送开始消息');
        
        agentChannel.send(JSON.stringify({
            event: 'user_message',
            payload: { text: '开始学习' },
            timestamp: Math.floor(Date.now() / 1000),
        }));
        
        setSending(true);
        autoStartTriggeredRef.current = false; // 防止重复触发
    }
}, [wsStatus, agentChannel, sending]);
```

## 用户体验

### 修改前

```
1. 学生点击"加入活动"
2. 进入会话页面
3. 看到空白对话框
4. 需要手动输入"开始"或其他消息
5. AI 才开始引导
```

### 修改后

```
1. 学生点击"加入活动"
2. 进入会话页面
3. 自动显示"AI 正在思考..."
4. AI 立即开始引导学习
5. 学生直接看到第一条引导消息
```

## 技术细节

### 防止重复触发

使用 `useRef` 存储触发状态:

```typescript
const autoStartTriggeredRef = useRef(false);

// 触发后立即重置
autoStartTriggeredRef.current = false;
```

**优点:**
- 不会因为组件重渲染而重复触发
- 不会因为 WebSocket 重连而重复触发
- 不会因为依赖变化而重复触发

### WebSocket 就绪检测

```typescript
if (wsStatus === 'connected' && !sending)
```

**确保:**
- WebSocket 已完全连接
- 没有其他消息正在发送
- 避免消息丢失或冲突

### 消息内容

```typescript
payload: { text: '开始学习' }
```

**说明:**
- 简短明确的触发消息
- 后端 Strategist 会根据活动配置生成实际引导
- 不影响用户体验 (消息不会显示在界面上)

## 兼容性

### 现有会话

- ✅ 有历史消息的会话不会触发自动开始
- ✅ 用户可以继续之前的学习进度
- ✅ 不影响现有功能

### 插件渲染器

- ✅ 自动开始逻辑在插件渲染器激活前执行
- ✅ 插件可以正常接管 WebSocket 事件
- ✅ 不影响特殊技能的初始化

### 教师预览

- ✅ 教师预览模式 (sandbox) 同样支持自动开始
- ✅ 预览体验与学生一致

## 测试验证

### 测试场景 1: 新会话

1. 创建学习活动并发布
2. 学生点击"加入活动"
3. 观察会话页面

**预期结果:**
- ✅ 自动显示"AI 正在思考..."
- ✅ 几秒后显示 AI 的第一条引导消息
- ✅ 无需学生手动输入

### 测试场景 2: 继续会话

1. 学生已有进行中的会话
2. 刷新页面或重新进入
3. 观察会话页面

**预期结果:**
- ✅ 显示历史消息
- ✅ 不会自动发送新消息
- ✅ 等待学生继续输入

### 测试场景 3: WebSocket 重连

1. 学生进入新会话
2. 断开网络连接
3. 恢复网络连接

**预期结果:**
- ✅ WebSocket 重连后不会重复发送
- ✅ 只在首次连接时触发一次

## 日志输出

### 正常流程

```
[SESSION] 新会话,准备自动开始学习活动
[SESSION] WebSocket 已连接,自动发送开始消息
[WS DEBUG] 发送消息: {"event":"user_message","payload":{"text":"开始学习"},...}
```

### 已有消息

```
(无日志输出,直接加载历史消息)
```

## 性能影响

- **网络请求:** 无额外请求
- **渲染性能:** 无影响
- **内存占用:** 增加 1 个 ref (可忽略)

## 未来优化

### 可配置触发消息

```typescript
// 从活动配置读取自定义触发消息
const triggerMessage = activity.auto_start_message || '开始学习';
```

### 延迟触发

```typescript
// 添加短暂延迟,让用户看到页面加载完成
setTimeout(() => {
    agentChannel.send(...);
}, 300);
```

### 进度提示

```typescript
// 显示"正在准备学习材料..."等提示
setThinkingStatus('正在准备学习材料...');
```

## 总结

**功能:** 学生进入学习活动时自动开始  
**实现:** 检测新会话 + WebSocket 就绪 → 自动发送触发消息  
**效果:** 提升用户体验,减少操作步骤  
**兼容:** 不影响现有功能和会话

---

**实施日期:** 2026-03-07  
**影响文件:** `frontend/src/app/student/session/[id]/page.tsx`  
**测试状态:** 编译通过,待功能测试
