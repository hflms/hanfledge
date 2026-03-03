package agent

import "context"

// GetInitialPrescription 暴露给外部以预检初始学习处方
func (o *AgentOrchestrator) GetInitialPrescription(ctx context.Context, sessionID, studentID, activityID uint) (*LearningPrescription, error) {
	p, err := o.strategist.Analyze(ctx, sessionID, studentID, activityID)
	if err != nil {
		return nil, err
	}
	return &p, nil
}
