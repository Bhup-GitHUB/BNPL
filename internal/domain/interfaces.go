package domain

import "context"

type BureauClient interface {
	Fetch(ctx context.Context, application Application) (BureauReport, error)
}

type AccountAggregatorClient interface {
	Fetch(ctx context.Context, application Application) (AccountAggregatorReport, error)
}

type FraudClient interface {
	Fetch(ctx context.Context, application Application) (FraudReport, error)
}

type FeatureBuilder interface {
	Extract(ctx context.Context, application Application) (BehaviorFeatures, error)
	Build(application Application, bureau BureauReport, aa AccountAggregatorReport, fraud FraudReport, behavior BehaviorFeatures) FeatureVector
}

type ScoringEngine interface {
	Score(ctx context.Context, features FeatureVector) (ScoreResult, error)
}

type RulesEngine interface {
	Evaluate(ctx context.Context, application Application, features FeatureVector, score ScoreResult, criticalTimeout bool) (RuleResult, error)
}

type ApplicationRepository interface {
	CreateApplication(ctx context.Context, application Application) (Application, error)
	GetByIdempotencyKey(ctx context.Context, key string) (ApplicationRecord, bool, error)
	SaveDecision(ctx context.Context, decision Decision, snapshots []ProviderSnapshot) error
	GetApplication(ctx context.Context, applicationID string) (ApplicationRecord, error)
	GetDecisionTrace(ctx context.Context, applicationID string) (DecisionTrace, error)
	Ping(ctx context.Context) error
}

type AuditRepository interface {
	AppendAuditEvent(ctx context.Context, event AuditEvent) error
}
