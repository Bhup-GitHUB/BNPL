package domain

import "time"

type DecisionStatus string

const (
	StatusPending      DecisionStatus = "pending"
	StatusApproved     DecisionStatus = "approved"
	StatusManualReview DecisionStatus = "manual_review"
	StatusRejected     DecisionStatus = "rejected"
)

type Application struct {
	ID                string
	TraceID           string
	IdempotencyKey    string
	FullName          string
	PAN               string
	Mobile            string
	Email             string
	EmploymentType    string
	MonthlyIncome     int64
	RequestedAmount   int64
	ConsentHandle     string
	BankAccountRef    string
	DeviceFingerprint string
	TypingCadenceMs   int64
	FormFillSeconds   int64
	AddressPincode    string
	EmployerName      string
	Metadata          map[string]string
	Status            DecisionStatus
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type ApplicationRecord struct {
	Application Application
	Decision    Decision
	Snapshots   []ProviderSnapshot
}

type BureauReport struct {
	ScoreBand string
	ThinFile  bool
	Provider  string
	Raw       map[string]any
}

type AccountAggregatorReport struct {
	IncomeStability float64
	BalanceTrend    string
	BounceCount     int
	ConsentRef      string
	Raw             map[string]any
}

type FraudReport struct {
	FraudHit         bool
	DedupeHit        bool
	DeviceReuseCount int
	Raw              map[string]any
}

type BehaviorFeatures struct {
	TypingCadenceAnomaly bool
	FormCompletionSpeed  string
	DeviceConfidence     float64
	Raw                  map[string]any
}

type FeatureVector struct {
	IncomeStability      float64
	BalanceTrend         string
	BounceCount          int
	BureauScoreBand      string
	ThinFile             bool
	TypingCadenceAnomaly bool
	FormCompletionSpeed  string
	DeviceReuseCount     int
	DedupeHit            bool
	FraudHit             bool
	MonthlyIncome        int64
	RequestedAmount      int64
	FeatureSnapshot      map[string]any
}

type ScoreResult struct {
	Score     int
	Breakdown map[string]float64
}

type RuleResult struct {
	Status        DecisionStatus
	Reasons       []string
	RuleHits      []string
	ApprovedLimit int64
}

type Decision struct {
	ApplicationID  string
	Status         DecisionStatus
	Score          int
	ApprovedLimit  int64
	Reasons        []string
	RuleHits       []string
	ScoreBreakdown map[string]float64
	FeatureSummary map[string]any
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type ProviderSnapshot struct {
	ApplicationID string
	ProviderName  string
	Status        string
	LatencyMs     int64
	Payload       map[string]any
	CreatedAt     time.Time
}

type AuditEvent struct {
	ApplicationID string
	EventType     string
	Payload       map[string]any
	CreatedAt     time.Time
}

type DecisionTrace struct {
	ApplicationID   string
	TraceID         string
	Status          DecisionStatus
	Score           int
	ApprovedLimit   int64
	Reasons         []string
	RuleHits        []string
	ScoreBreakdown  map[string]float64
	FeatureSnapshot map[string]any
	FraudHit        bool
	DedupeHit       bool
	ProviderState   map[string]string
	ProviderLatency map[string]int64
	CreatedAt       time.Time
	UpdatedAt       time.Time
}
