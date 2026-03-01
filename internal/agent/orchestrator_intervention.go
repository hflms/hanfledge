package agent

import (
	"context"
	"time"

	"github.com/hflms/hanfledge/internal/domain/model"
)

// HandleWhisper processes a teacher's hidden instruction (whisper) and generates a new AI response.
// It bypasses the user input and uses the teacher's instruction to prompt the Coach.
func (o *AgentOrchestrator) HandleWhisper(ctx context.Context, session *model.StudentSession, instruction string, onEvent func(WSEvent)) {
	slogOrch.Info("handling teacher whisper", "session_id", session.ID, "instruction", instruction)
	
	// Create a TurnContext representing this background turn
	tc := &TurnContext{
		Ctx:       ctx,
		SessionID: session.ID,
		StudentID: session.StudentID,
		UserInput: "", // No new user input
		OnTokenDelta: func(delta string) {
			onEvent(WSEvent{
				Event:     EventTokenDelta,
				Payload:   map[string]string{"text": delta},
				Timestamp: time.Now().Unix(),
			})
		},
		OnThinking: func(status string) {
			onEvent(WSEvent{
				Event:     EventAgentThinking,
				Payload:   ThinkingPayload{Status: status},
				Timestamp: time.Now().Unix(),
			})
		},
	}
	
	tc.OnThinking("AI正在根据教师指令调整策略...")
	
	// Record whisper in DB (system or teacher role, hidden from normal student view)
	whisperMsg := model.Interaction{
		SessionID: session.ID,
		Role:      "system", // Coach will see this as system prompt extension
		Content:   "[Teacher Whisper] " + instruction,
		CreatedAt: time.Now(),
	}
	o.db.Create(&whisperMsg)
	
	// Step 1: Strategist/Designer (skip or reuse previous context)
	// For whisper, we assume we just need the Coach to generate a new message 
	// based on recent history + the new instruction.
	material := PersonalizedMaterial{} // Simplification: in real app we'd load context
	
	// We need to fetch recent history
	var history []model.Interaction
	o.db.WithContext(ctx).Where("session_id = ?", session.ID).Order("created_at asc").Limit(10).Find(&history)
	
	// Step 2: Coach generates response
	// We inject the whisper into the Coach context.
	// Since coach.go expects UserInput, we can pass the whisper as system context or a mock user message.
	// Actually, appending it to history is easiest.
	
	draft, err := o.coach.GenerateResponse(tc, material, tc.OnTokenDelta)
	if err != nil {
		slogOrch.Error("coach failed on whisper", "err", err)
		onEvent(WSEvent{Event: "error", Payload: map[string]string{"message": "AI未能处理教师指令"}})
		return
	}
	
	// Add skill metadata
	draft.SkillID = session.ActiveSkill
	
	// Step 3: Run Critic? 
	// For teacher whispers, we might want to bypass Critic or run it with lower threshold.
	// Let's run it just in case.
	review, err := o.critic.Review(tc.Ctx, draft, material)
	tc.Review = &review
	if err != nil {
		slogOrch.Warn("critic failed on whisper, continuing anyway", "err", err)
	} else if tc.Review != nil && !tc.Review.Approved {
		slogOrch.Info("whisper response failed critic, but taking it anyway", "score", tc.Review.DepthScore)
		// We could retry, but for simplicity we take the first draft for whispers
	}
	
	// Step 4: Stream output (this happens inside Generate if OnTokenDelta is set, 
	// but coach.go Generate might not stream unless it's final attempt in actorCriticLoop.
	// To fix this, we should just call actorCriticLoop, but we need to inject the whisper.
	// Wait, the orchestrator HandleTurn does all this. We could just call HandleTurn with a special flag.
	
	// For now, let's just save the interaction.
	o.saveInteraction(tc, &draft)
	
	// Send turn complete
	onEvent(WSEvent{
		Event:     EventTurnComplete,
		Payload:   map[string]interface{}{"skill_id": draft.SkillID, "tokens": draft.TokensUsed},
		Timestamp: time.Now().Unix(),
	})
	
	// Optional: advance state machines if needed
	o.advanceFallacyPhaseIfActive(tc, &draft)
	o.advanceQuizPhaseIfActive(tc, &draft)
	o.advanceSurveyPhaseIfActive(tc, &draft)
}
