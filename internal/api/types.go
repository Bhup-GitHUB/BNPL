package api

type SubmitApplicationRequest struct {
	IdempotencyKey    string            `json:"idempotency_key"`
	FullName          string            `json:"full_name"`
	PAN               string            `json:"pan"`
	Mobile            string            `json:"mobile"`
	Email             string            `json:"email"`
	EmploymentType    string            `json:"employment_type"`
	MonthlyIncome     int64             `json:"monthly_income"`
	RequestedAmount   int64             `json:"requested_amount"`
	ConsentHandle     string            `json:"consent_handle"`
	BankAccountRef    string            `json:"bank_account_ref"`
	DeviceFingerprint string            `json:"device_fingerprint"`
	TypingCadenceMs   int64             `json:"typing_cadence_ms"`
	FormFillSeconds   int64             `json:"form_fill_seconds"`
	AddressPincode    string            `json:"address_pincode"`
	EmployerName      string            `json:"employer_name"`
	Metadata          map[string]string `json:"metadata"`
}

type SubmitApplicationResponse struct {
	ApplicationID string            `json:"application_id"`
	TraceID       string            `json:"trace_id"`
	Status        string            `json:"status"`
	ApprovedLimit int64             `json:"approved_limit"`
	Reasons       []string          `json:"reasons"`
	Score         int32             `json:"score"`
	LatencyMs     int64             `json:"latency_ms"`
	ProviderState map[string]string `json:"provider_state"`
	CreatedAt     string            `json:"created_at"`
	UpdatedAt     string            `json:"updated_at"`
}

type GetApplicationRequest struct {
	ApplicationID string `json:"application_id"`
}

type GetApplicationResponse struct {
	ApplicationID   string            `json:"application_id"`
	TraceID         string            `json:"trace_id"`
	Status          string            `json:"status"`
	ApprovedLimit   int64             `json:"approved_limit"`
	Reasons         []string          `json:"reasons"`
	Score           int32             `json:"score"`
	ProviderState   map[string]string `json:"provider_state"`
	ProviderLatency map[string]int64  `json:"provider_latency"`
	CreatedAt       string            `json:"created_at"`
	UpdatedAt       string            `json:"updated_at"`
}

type GetDecisionTraceRequest struct {
	ApplicationID string `json:"application_id"`
}

type GetDecisionTraceResponse struct {
	ApplicationID   string             `json:"application_id"`
	TraceID         string             `json:"trace_id"`
	Status          string             `json:"status"`
	Score           int32              `json:"score"`
	ScoreBreakdown  map[string]float64 `json:"score_breakdown"`
	RuleHits        []string           `json:"rule_hits"`
	Reasons         []string           `json:"reasons"`
	FraudHit        bool               `json:"fraud_hit"`
	DedupeHit       bool               `json:"dedupe_hit"`
	ProviderState   map[string]string  `json:"provider_state"`
	ProviderLatency map[string]int64   `json:"provider_latency"`
	FeatureSnapshot map[string]any     `json:"feature_snapshot"`
	ApprovedLimit   int64              `json:"approved_limit"`
	CreatedAt       string             `json:"created_at"`
	UpdatedAt       string             `json:"updated_at"`
}
