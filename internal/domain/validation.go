package domain

import (
	"fmt"
	"math"
	"strings"
)

// FieldError describes a single validation error tied to a specific input field and optional index.
type FieldError struct {
	Field   string `json:"field"`
	Index   int    `json:"index,omitempty"` // -1 if not index-specific
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e FieldError) Error() string {
	if e.Index >= 0 {
		return fmt.Sprintf("%s[%d] (%s): %s", e.Field, e.Index, e.Code, e.Message)
	}
	return fmt.Sprintf("%s (%s): %s", e.Field, e.Code, e.Message)
}

// ValidationError aggregates one or more field-level validation failures.
type ValidationError struct {
	Errors []FieldError `json:"errors"`
}

func (v *ValidationError) Error() string {
	if v == nil || len(v.Errors) == 0 {
		return "validation failed"
	}
	parts := make([]string, len(v.Errors))
	for i, err := range v.Errors {
		parts[i] = err.Error()
	}
	return "validation failed: " + strings.Join(parts, "; ")
}

func (v *ValidationError) Add(field string, index int, code, msg string) {
	v.Errors = append(v.Errors, FieldError{
		Field:   field,
		Index:   index,
		Code:    code,
		Message: msg,
	})
}

func (v *ValidationError) HasErrors() bool {
	return len(v.Errors) > 0
}

// CheckedAddInt64 adds two nonnegative int64 values and reports whether overflow occurred.
func CheckedAddInt64(a, b int64) (int64, bool) {
	if a < 0 || b < 0 {
		return 0, false
	}
	if a > math.MaxInt64-b {
		return 0, false
	}
	return a + b, true
}

// ValidateBatch validates all authoritative domain rules on a Batch and returns
// the total opening funds (in minor units) if valid.
// It normalizes payment Position fields to match their array index 0..N-1.
func ValidateBatch(b *Batch) (int64, error) {
	ve := &ValidationError{}
	if b == nil {
		ve.Add("batch", -1, "nil_batch", "batch cannot be nil")
		return 0, ve
	}

	if b.SchemaVersion != "" && b.SchemaVersion != CurrentSchemaVersion {
		ve.Add("schema_version", -1, "unsupported_schema_version",
			fmt.Sprintf("unsupported schema version %q (expected %q)", b.SchemaVersion, CurrentSchemaVersion))
	}

	if len(b.Payments) > MaxPayments {
		ve.Add("payments", -1, "max_payments_exceeded",
			fmt.Sprintf("payment count %d exceeds maximum supported limit of %d", len(b.Payments), MaxPayments))
	}

	if len(b.Payments) > 0 && len(b.Accounts) < 2 {
		ve.Add("accounts", -1, "insufficient_accounts",
			"at least 2 accounts are required when a batch contains payments")
	}

	accountSet := make(map[string]int, len(b.Accounts))
	var totalFunds int64
	overflowed := false

	for i, acc := range b.Accounts {
		id := strings.TrimSpace(acc.ID)
		if id == "" {
			ve.Add("accounts.id", i, "empty_account_id", "account ID must not be empty")
		} else if prevIdx, exists := accountSet[id]; exists {
			ve.Add("accounts.id", i, "duplicate_account_id",
				fmt.Sprintf("duplicate account ID %q (first seen at index %d)", id, prevIdx))
		} else {
			accountSet[id] = i
		}

		bal := int64(acc.OpeningBalance)
		if bal < 0 {
			ve.Add("accounts.opening_balance", i, "negative_opening_balance",
				fmt.Sprintf("account %q has negative opening balance %d", id, bal))
		} else if !overflowed {
			sum, ok := CheckedAddInt64(totalFunds, bal)
			if !ok {
				overflowed = true
				ve.Add("accounts.opening_balance", i, "total_funds_overflow",
					"sum of opening balances exceeds int64 maximum (math.MaxInt64)")
			} else {
				totalFunds = sum
			}
		}
	}

	paymentSet := make(map[string]int, len(b.Payments))
	for i := range b.Payments {
		b.Payments[i].Position = i
		p := b.Payments[i]

		id := strings.TrimSpace(p.ID)
		if id == "" {
			ve.Add("payments.id", i, "empty_payment_id", "payment ID must not be empty")
		} else if prevIdx, exists := paymentSet[id]; exists {
			ve.Add("payments.id", i, "duplicate_payment_id",
				fmt.Sprintf("duplicate payment ID %q (first seen at position %d)", id, prevIdx))
		} else {
			paymentSet[id] = i
		}

		sender := strings.TrimSpace(p.Sender)
		receiver := strings.TrimSpace(p.Receiver)

		if sender == "" {
			ve.Add("payments.sender", i, "empty_sender", "sender account ID must not be empty")
		} else if _, exists := accountSet[sender]; !exists {
			ve.Add("payments.sender", i, "unknown_sender_account",
				fmt.Sprintf("sender account %q does not exist in batch accounts", sender))
		}

		if receiver == "" {
			ve.Add("payments.receiver", i, "empty_receiver", "receiver account ID must not be empty")
		} else if _, exists := accountSet[receiver]; !exists {
			ve.Add("payments.receiver", i, "unknown_receiver_account",
				fmt.Sprintf("receiver account %q does not exist in batch accounts", receiver))
		}

		if sender != "" && sender == receiver {
			ve.Add("payments.receiver", i, "self_transfer_forbidden",
				fmt.Sprintf("self-transfer on account %q is invalid input", sender))
		}

		amt := int64(p.Amount)
		if amt <= 0 {
			ve.Add("payments.amount", i, "non_positive_amount",
				fmt.Sprintf("payment amount must be a positive integer, got %d", amt))
		}
	}

	if ve.HasErrors() {
		return 0, ve
	}
	return totalFunds, nil
}

// ValidateScenarioConfig validates synthetic batch generation parameters before generation.
func ValidateScenarioConfig(cfg ScenarioConfig) error {
	ve := &ValidationError{}

	if cfg.PaymentCount < 0 || cfg.PaymentCount > MaxPayments {
		ve.Add("payment_count", -1, "invalid_payment_count",
			fmt.Sprintf("payment_count must be between 0 and %d, got %d", MaxPayments, cfg.PaymentCount))
	}

	var derivedAccounts int
	switch cfg.Scenario {
	case ScenarioIndependent:
		derivedAccounts = 2 * cfg.PaymentCount
	case ScenarioSharedAccount:
		if cfg.PaymentCount == 0 {
			derivedAccounts = 0
		} else {
			derivedAccounts = cfg.PaymentCount + 1
		}
	case ScenarioUniform:
		if cfg.PaymentCount > 0 && cfg.AccountCount < 2 {
			ve.Add("account_count", -1, "insufficient_accounts",
				"uniform scenario requires at least 2 accounts when payment_count > 0")
		}
		if cfg.AccountCount < 0 || cfg.AccountCount > 2*MaxPayments {
			ve.Add("account_count", -1, "invalid_account_count",
				fmt.Sprintf("account_count must be between 0 and %d, got %d", 2*MaxPayments, cfg.AccountCount))
		}
		derivedAccounts = cfg.AccountCount
	default:
		ve.Add("scenario", -1, "unsupported_scenario",
			fmt.Sprintf("scenario %q is not supported (expected independent, shared_account, or uniform)", cfg.Scenario))
	}

	openBal := int64(cfg.OpeningBalance)
	if openBal < 0 {
		ve.Add("opening_balance", -1, "negative_opening_balance",
			fmt.Sprintf("opening_balance must be nonnegative, got %d", openBal))
	} else if derivedAccounts > 0 && openBal > 0 {
		if openBal > math.MaxInt64/int64(derivedAccounts) {
			ve.Add("opening_balance", -1, "total_funds_overflow",
				"total opening funds (account_count * opening_balance) exceeds int64 maximum")
		}
	}

	minAmt := int64(cfg.MinAmount)
	maxAmt := int64(cfg.MaxAmount)
	if cfg.PaymentCount > 0 {
		if minAmt <= 0 {
			ve.Add("min_amount", -1, "non_positive_min_amount",
				fmt.Sprintf("min_amount must be positive, got %d", minAmt))
		}
		if maxAmt <= 0 {
			ve.Add("max_amount", -1, "non_positive_max_amount",
				fmt.Sprintf("max_amount must be positive, got %d", maxAmt))
		}
		if minAmt > 0 && maxAmt > 0 && minAmt > maxAmt {
			ve.Add("max_amount", -1, "min_exceeds_max_amount",
				fmt.Sprintf("min_amount (%d) cannot exceed max_amount (%d)", minAmt, maxAmt))
		}
	}

	if ve.HasErrors() {
		return ve
	}
	return nil
}

// ValidateRunConfig checks execution mode, worker count bounds (1..64), and scheduling policy.
func ValidateRunConfig(cfg RunConfig) error {
	ve := &ValidationError{}

	switch cfg.Mode {
	case ModeSequential, ModeLogical, ModeConcurrent:
	default:
		ve.Add("mode", -1, "invalid_execution_mode",
			fmt.Sprintf("unsupported execution mode %q (expected sequential, logical, or concurrent)", cfg.Mode))
	}

	if cfg.RequestedWorkers < MinWorkers || cfg.RequestedWorkers > MaxWorkers {
		ve.Add("requested_workers", -1, "invalid_worker_count",
			fmt.Sprintf("requested_workers must be between %d and %d, got %d", MinWorkers, MaxWorkers, cfg.RequestedWorkers))
	}

	switch cfg.Policy {
	case "", PolicyEarliestPosition, PolicyCriticalPathFirst:
	default:
		ve.Add("policy", -1, "invalid_scheduling_policy",
			fmt.Sprintf("unsupported scheduling policy %q (expected earliest_position or critical_path_first)", cfg.Policy))
	}

	if ve.HasErrors() {
		return ve
	}
	return nil
}
