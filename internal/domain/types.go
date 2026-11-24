package domain

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

// Schema and application constants.
const (
	CurrentSchemaVersion = "ppse.v1"
	AppVersion           = "1.0.0"
	DefaultCurrency      = "SEK"

	MaxPayments    = 10000
	MinWorkers     = 1
	MaxWorkers     = 64
	MaxImportBytes = 16 * 1024 * 1024 // 16 MiB
)

// Int64String serializes an int64 as a JSON decimal string so JavaScript
// never loses integer precision above 2^53 - 1, while accepting either a JSON
// string or a JSON number on unmarshal.
type Int64String int64

func (v Int64String) MarshalJSON() ([]byte, error) {
	return json.Marshal(strconv.FormatInt(int64(v), 10))
}

func (v *Int64String) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		parsed, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid int64 string %q: %w", s, err)
		}
		*v = Int64String(parsed)
		return nil
	}
	var n int64
	if err := json.Unmarshal(data, &n); err != nil {
		return fmt.Errorf("expected int64 string or number: %w", err)
	}
	*v = Int64String(n)
	return nil
}

// Uint64String serializes a uint64 (such as a 64-bit PRNG seed) as a decimal
// string in JSON, while accepting either a JSON string or JSON number.
type Uint64String uint64

func (v Uint64String) MarshalJSON() ([]byte, error) {
	return json.Marshal(strconv.FormatUint(uint64(v), 10))
}

func (v *Uint64String) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		parsed, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid uint64 string %q: %w", s, err)
		}
		*v = Uint64String(parsed)
		return nil
	}
	var n uint64
	if err := json.Unmarshal(data, &n); err != nil {
		return fmt.Errorf("expected uint64 string or number: %w", err)
	}
	*v = Uint64String(n)
	return nil
}

// Outcome represents the authoritative settlement outcome of a single payment attempt.
type Outcome string

const (
	OutcomeSucceeded                 Outcome = "succeeded"
	OutcomeRejectedInsufficientFunds Outcome = "rejected_insufficient_funds"
)

// ScenarioType identifies the synthetic batch generation scenario.
type ScenarioType string

const (
	ScenarioIndependent   ScenarioType = "independent"
	ScenarioSharedAccount ScenarioType = "shared_account"
	ScenarioUniform       ScenarioType = "uniform"
	ScenarioCustom        ScenarioType = "custom"
)

// ExecutionMode identifies how a payment batch is executed.
type ExecutionMode string

const (
	ModeSequential ExecutionMode = "sequential"
	ModeLogical    ExecutionMode = "logical"
	ModeConcurrent ExecutionMode = "concurrent"
)

// SchedulingPolicy identifies the ready-queue tie-breaking / priority rule.
type SchedulingPolicy string

const (
	PolicyEarliestPosition  SchedulingPolicy = "earliest_position"
	PolicyCriticalPathFirst SchedulingPolicy = "critical_path_first"
)

// RunStatus represents the lifecycle state of an execution run.
type RunStatus string

const (
	RunStatusQueued    RunStatus = "queued"
	RunStatusRunning   RunStatus = "running"
	RunStatusCompleted RunStatus = "completed"
	RunStatusFailed    RunStatus = "failed"
	RunStatusCancelled RunStatus = "cancelled"
)

// AccountInput represents an account's synthetic identifier and opening balance in minor units.
type AccountInput struct {
	ID             string      `json:"id"`
	OpeningBalance Int64String `json:"opening_balance"`
}

// PaymentInput represents an ordered payment instruction in a batch.
// Position is determined by its 0-based index in Batch.Payments.
type PaymentInput struct {
	Position int         `json:"position"`
	ID       string      `json:"id"`
	Sender   string      `json:"sender"`
	Receiver string      `json:"receiver"`
	Amount   Int64String `json:"amount"`
}

// ScenarioConfig records the configuration used to create or generate a batch.
type ScenarioConfig struct {
	Scenario       ScenarioType `json:"scenario"`
	AccountCount   int          `json:"account_count"`
	PaymentCount   int          `json:"payment_count"`
	OpeningBalance Int64String  `json:"opening_balance"`
	MinAmount      Int64String  `json:"min_amount"`
	MaxAmount      Int64String  `json:"max_amount"`
	CommonAccount  string       `json:"common_account,omitempty"`
}

// SeedMetadata records the deterministic random number generator settings.
type SeedMetadata struct {
	Seed             Uint64String `json:"seed"`
	SecondSeed       Uint64String `json:"second_seed"`
	GeneratorAlgo    string       `json:"generator_algorithm"`
	GeneratorVersion string       `json:"generator_version"`
}

// Batch is an immutable payment batch with opening account balances and ordered payments.
type Batch struct {
	ID                string         `json:"id"`
	SchemaVersion     string         `json:"schema_version"`
	Name              string         `json:"name"`
	Description       string         `json:"description,omitempty"`
	Currency          string         `json:"currency"`
	CreatedAt         time.Time      `json:"created_at"`
	Configuration     ScenarioConfig `json:"configuration"`
	SeedMeta          *SeedMetadata  `json:"seed_metadata,omitempty"`
	Accounts          []AccountInput `json:"accounts"`
	Payments          []PaymentInput `json:"payments"`
	TotalOpeningFunds Int64String    `json:"total_opening_funds"`
	Fingerprint       string         `json:"fingerprint"`
}

// CloneAccounts returns a fresh deep copy of the batch's accounts slice.
func (b *Batch) CloneAccounts() []AccountInput {
	if b == nil || len(b.Accounts) == 0 {
		return nil
	}
	out := make([]AccountInput, len(b.Accounts))
	copy(out, b.Accounts)
	return out
}

// ClonePayments returns a fresh deep copy of the batch's payments slice with normalized positions.
func (b *Batch) ClonePayments() []PaymentInput {
	if b == nil || len(b.Payments) == 0 {
		return nil
	}
	out := make([]PaymentInput, len(b.Payments))
	copy(out, b.Payments)
	for i := range out {
		out[i].Position = i
	}
	return out
}

// AccountBalance represents a final or snapshot balance for a single account.
type AccountBalance struct {
	Position       int         `json:"position"`
	ID             string      `json:"id"`
	OpeningBalance Int64String `json:"opening_balance"`
	FinalBalance   Int64String `json:"final_balance"`
	NetChange      Int64String `json:"net_change"`
}

// PaymentResult records the execution trace and outcome of a single payment attempt.
// Notice that LogicalStart/LogicalFinish (abstract integer time units) are strictly separated
// from StartOffsetNs/FinishOffsetNs (monotonic elapsed nanoseconds from the run origin).
type PaymentResult struct {
	Position int         `json:"position"`
	ID       string      `json:"id"`
	Sender   string      `json:"sender"`
	Receiver string      `json:"receiver"`
	Amount   Int64String `json:"amount"`
	Outcome  Outcome     `json:"outcome"`
	WorkerID int         `json:"worker_id"`

	// Logical simulation timing domain (abstract integer time units; 1 unit per attempt).
	LogicalStart  int64 `json:"logical_start"`
	LogicalFinish int64 `json:"logical_finish"`

	// Actual execution timing domain (monotonic nanoseconds relative to run execution start).
	StartOffsetNs  Int64String `json:"start_offset_ns"`
	FinishOffsetNs Int64String `json:"finish_offset_ns"`
	DurationNs     Int64String `json:"duration_ns"`
}

// RunConfig specifies the execution parameters for a single run.
type RunConfig struct {
	Mode             ExecutionMode    `json:"mode"`
	RequestedWorkers int              `json:"requested_workers"`
	Policy           SchedulingPolicy `json:"policy"`
}

// LogicalMetrics holds abstract unit-duration model metrics.
type LogicalMetrics struct {
	TotalPayments           int     `json:"total_payments"`
	SucceededCount          int     `json:"succeeded_count"`
	RejectedCount           int     `json:"rejected_count"`
	RequestedWorkers        int     `json:"requested_workers"`
	EffectiveWorkers        int     `json:"effective_workers"`
	SequentialBaselineUnits int64   `json:"sequential_baseline_units"`
	MakespanUnits           int64   `json:"makespan_units"`
	CriticalPathLength      int64   `json:"critical_path_length"`
	LowerBoundUnits         int64   `json:"lower_bound_units"`
	Applicable              bool    `json:"applicable"`
	Speedup                 float64 `json:"speedup"`
	EfficiencyRequested     float64 `json:"efficiency_requested"`
	EfficiencyEffective     float64 `json:"efficiency_effective"`
	UtilisationRequested    float64 `json:"utilisation_requested"`
	UtilisationEffective    float64 `json:"utilisation_effective"`
}

// MeasuredMetrics holds actual wall-clock / monotonic elapsed execution measurements
// clearly separated from application overhead (analysis, setup, verification, persistence).
type MeasuredMetrics struct {
	TotalPayments                 int         `json:"total_payments"`
	SucceededCount                int         `json:"succeeded_count"`
	RejectedCount                 int         `json:"rejected_count"`
	RequestedWorkers              int         `json:"requested_workers"`
	EffectiveWorkers              int         `json:"effective_workers"`
	AnalysisDurationNs            Int64String `json:"analysis_duration_ns"`
	SetupDurationNs               Int64String `json:"setup_duration_ns"`
	ExecutionDurationNs           Int64String `json:"execution_duration_ns"`
	VerificationDurationNs        Int64String `json:"verification_duration_ns"`
	PersistenceDurationNs         Int64String `json:"persistence_duration_ns"`
	TotalEndToEndDurationNs       Int64String `json:"total_end_to_end_duration_ns"`
	DirectSequentialBaselineNs    Int64String `json:"direct_sequential_baseline_ns,omitempty"`
	MeasuredExecutionSpeedup      float64     `json:"measured_execution_speedup,omitempty"`
	CompletedAttemptThroughputPS  float64     `json:"completed_attempt_throughput_per_sec"`
	SuccessfulPaymentThroughputPS float64     `json:"successful_payment_throughput_per_sec"`
}

// InvariantCheck records the pass/fail result of a single verification invariant.
type InvariantCheck struct {
	Name    string `json:"name"`
	Passed  bool   `json:"passed"`
	Details string `json:"details"`
}

// VerificationReport holds the complete independent correctness check report for a run.
type VerificationReport struct {
	Passed            bool             `json:"passed"`
	Summary           string           `json:"summary"`
	TotalOpeningFunds Int64String      `json:"total_opening_funds"`
	TotalFinalFunds   Int64String      `json:"total_final_funds"`
	Checks            []InvariantCheck `json:"checks"`
}

// RunResult aggregates the outputs of a completed execution run.
type RunResult struct {
	TimingDomain   string             `json:"timing_domain"` // "logical_units" or "measured_nanoseconds"
	PaymentResults []PaymentResult    `json:"payment_results"`
	FinalBalances  []AccountBalance   `json:"final_balances"`
	Logical        *LogicalMetrics    `json:"logical_metrics,omitempty"`
	Measured       *MeasuredMetrics   `json:"measured_metrics,omitempty"`
	Verification   VerificationReport `json:"verification"`
}
