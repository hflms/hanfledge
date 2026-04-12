package agent

import (
	"context"
	"time"

	"github.com/hflms/hanfledge/internal/domain/model"
	"github.com/hflms/hanfledge/internal/infrastructure/cache"
)

// HandleWhisper processes a teacher's hidden instruction (whisper) and generates a new AI response.
// It bypasses the user input and uses the teacher's instruction to prompt the Coach.
func (o *AgentOrchestrator) HandleWhisper(ctx context.Context, session *model.StudentSession, instruction string, onEvent func(WSEvent)) {
	slogOrch.Info("handling teacher whisper", "session_id", session.ID, "instruction", instruction)

	// Create a TurnContext representing this background turn
	tc := &TurnContext{
		Ctx:            ctx,
		SessionID:      session.ID,
		StudentID:      session.StudentID,
		UserInput:      "", // No new user input
		TeacherWhisper: instruction,
		Scaffold:       session.Scaffold,
		IsSandbox:      session.IsSandbox,
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
		Role:      "teacher", // Use "teacher" role so dashboard shows it nicely
		Content:   "[Whisper] " + instruction,
		CreatedAt: time.Now(),
	}
	o.db.Create(&whisperMsg)

	// Inject into cache so future turns see it. We pretend it's a "user" message
	// for the LLM's sake so it doesn't break chat template constraints.
	if o.cache != nil {
		_ = o.cache.AppendSessionHistory(ctx, session.ID, cache.CachedMessage{
			Role:    "user",
			Content: "[来自教师的隐藏干预指令] " + instruction,
		})
	}

	// Step 1: Strategist/Designer (skip or reuse previous context)
	// For whisper, we assume we just need the Coach to generate a new message
	// based on recent history + the new instruction.
	material := PersonalizedMaterial{
		Prescription: LearningPrescription{
			RecommendedSkill: session.ActiveSkill,
		},
	}

	// Step 2 & 3 & 4: Call actorCriticLoop
	// We inject the whisper into the Coach context via TurnContext.
	// actorCriticLoop handles streaming on the final attempt.
	finalResponse, err := o.actorCriticLoop(tc, material)
	if err != nil {
		slogOrch.Error("actorCriticLoop failed on whisper", "err", err)
		onEvent(WSEvent{Event: "error", Payload: map[string]string{"message": "AI未能处理教师指令"}})
		return
	}

	// Ensure skill metadata is set
	if finalResponse != nil && finalResponse.SkillID == "" {
		finalResponse.SkillID = session.ActiveSkill
	}

	// Save the interaction.
	o.saveInteraction(tc, finalResponse)

	// Send turn complete
	onEvent(WSEvent{
		Event:     EventTurnComplete,
		Payload:   map[string]interface{}{"skill_id": finalResponse.SkillID, "tokens": finalResponse.TokensUsed},
		Timestamp: time.Now().Unix(),
	})

	// Optional: advance state machines if needed
	o.skillState.AdvanceFallacyIfActive(tc, finalResponse)
	o.skillState.AdvanceQuizIfActive(tc, finalResponse)
	o.skillState.AdvanceSurveyIfActive(tc, finalResponse)
}
