#!/bin/bash
# 知识点传递链路测试脚本

echo "=== 知识点传递链路测试 ==="
echo ""

# 1. 检查活动的知识点配置
echo "1️⃣ 检查学习活动的知识点配置"
echo "   查询: SELECT id, title, kp_ids FROM learning_activities WHERE id = <activity_id>;"
echo ""

# 2. 检查会话的 CurrentKP
echo "2️⃣ 检查会话的 CurrentKP"
echo "   查询: SELECT id, student_id, activity_id, current_kp FROM student_sessions WHERE id = <session_id>;"
echo ""

# 3. 检查知识点标题
echo "3️⃣ 检查知识点标题"
echo "   查询: SELECT id, title FROM knowledge_points WHERE id = <kp_id>;"
echo ""

# 4. 查看后端日志
echo "4️⃣ 查看后端日志（关键信息）"
echo "   grep 'strategist analysis complete' <log_file>"
echo "   grep 'enhanced query with KP title' <log_file>"
echo "   grep 'system prompt includes target KP' <log_file>"
echo ""

echo "=== 预期日志输出 ==="
cat <<'EOF'
[Strategist] strategist analysis complete 
    session_id=12 
    target_kp_id=45 
    target_kp_title="循环结构" 
    current_mastery=0.10 
    skill_id="presentation_generator"

[Designer] enhanced query with KP title 
    session_id=12 
    kp_id=45 
    kp_title="循环结构" 
    user_input="你好" 
    enhanced_query="循环结构 你好"

[Designer] system prompt includes target KP 
    session_id=12 
    kp_id=45 
    kp_title="循环结构"
EOF

echo ""
echo "=== 常见问题排查 ==="
echo ""
echo "❌ 问题 1: target_kp_id 与活动指定的不一致"
echo "   原因: Strategist 按掌握度排序,可能选择了其他知识点"
echo "   解决: 检查 sortTargetsByMastery 逻辑"
echo ""
echo "❌ 问题 2: kp_title 查询失败"
echo "   原因: 知识点 ID 不存在或数据库连接问题"
echo "   解决: 检查 knowledge_points 表"
echo ""
echo "❌ 问题 3: enhanced_query 中没有知识点标题"
echo "   原因: prescription.TargetKPSequence 为空"
echo "   解决: 检查 Strategist.Analyze 返回值"
echo ""
echo "❌ 问题 4: system prompt 中没有目标知识点"
echo "   原因: buildSystemPrompt 未正确调用"
echo "   解决: 检查 Designer.Assemble 流程"
echo ""

echo "=== 手动测试步骤 ==="
echo ""
echo "1. 启动后端: go run cmd/server/main.go"
echo "2. 创建学习活动,指定知识点 ID (如 45)"
echo "3. 学生加入活动"
echo "4. 观察日志输出"
echo "5. 验证每个环节的 kp_id 和 kp_title 是否一致"
echo ""
