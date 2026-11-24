package domain_test

import (
	"encoding/json"
	"math"
	"strings"
	"testing"

	"paymentexplorer/internal/domain"
)

func TestInt64AndUint64StringJSON(t *testing.T) {
	type payload struct {
		Amount domain.Int64String  `json:"amount"`
		Seed   domain.Uint64String `json:"seed"`
	}

	orig := payload{
		Amount: domain.Int64String(9007199254740993), // > 2^53 - 1
		Seed:   domain.Uint64String(18446744073709551615),
	}

	raw, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	jsonStr := string(raw)
	if !strings.Contains(jsonStr, `"9007199254740993"`) {
		t.Fatalf("expected amount serialized as decimal string, got %s", jsonStr)
	}
	if !strings.Contains(jsonStr, `"18446744073709551615"`) {
		t.Fatalf("expected seed serialized as decimal string, got %s", jsonStr)
	}

	var decoded payload
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if decoded.Amount != orig.Amount || decoded.Seed != orig.Seed {
		t.Fatalf("round-trip mismatch: got %+v, want %+v", decoded, orig)
	}

	// Also verify unmarshaling from plain JSON numbers works for convenience.
	var fromNum payload
	if err := json.Unmarshal([]byte(`{"amount": 12345, "seed": 42}`), &fromNum); err != nil {
		t.Fatalf("Unmarshal from numbers failed: %v", err)
	}
	if fromNum.Amount != 12345 || fromNum.Seed != 42 {
		t.Fatalf("unexpected values from numeric JSON: %+v", fromNum)
	}
}

func TestProcessTransfer(t *testing.T) {
	// Sufficient funds
	s, r, out, err := domain.ProcessTransfer(100, 50, 40)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != domain.OutcomeSucceeded || s != 60 || r != 90 {
		t.Fatalf("got (s=%d, r=%d, out=%s), want (60, 90, succeeded)", s, r, out)
	}

	// Exact balance match (sender ends at 0)
	s, r, out, err = domain.ProcessTransfer(40, 0, 40)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != domain.OutcomeSucceeded || s != 0 || r != 40 {
		t.Fatalf("got (s=%d, r=%d, out=%s), want (0, 40, succeeded)", s, r, out)
	}

	// Insufficient funds leaves both accounts untouched
	s, r, out, err = domain.ProcessTransfer(39, 50, 40)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != domain.OutcomeRejectedInsufficientFunds || s != 39 || r != 50 {
		t.Fatalf("got (s=%d, r=%d, out=%s), want (39, 50, rejected_insufficient_funds)", s, r, out)
	}
}

func TestValidateBatchInvalidCases(t *testing.T) {
	cases := []struct {
		name     string
		batch    *domain.Batch
		wantCode string
	}{
		{
			name: "duplicate account ID",
			batch: &domain.Batch{
				Accounts: []domain.AccountInput{
					{ID: "A1", OpeningBalance: 100},
					{ID: "A1", OpeningBalance: 200},
				},
			},
			wantCode: "duplicate_account_id",
		},
		{
			name: "negative opening balance",
			batch: &domain.Batch{
				Accounts: []domain.AccountInput{
					{ID: "A1", OpeningBalance: -1},
					{ID: "A2", OpeningBalance: 100},
				},
			},
			wantCode: "negative_opening_balance",
		},
		{
			name: "opening balance int64 overflow",
			batch: &domain.Batch{
				Accounts: []domain.AccountInput{
					{ID: "A1", OpeningBalance: domain.Int64String(math.MaxInt64)},
					{ID: "A2", OpeningBalance: 1},
				},
			},
			wantCode: "total_funds_overflow",
		},
		{
			name: "duplicate payment ID",
			batch: &domain.Batch{
				Accounts: []domain.AccountInput{
					{ID: "A1", OpeningBalance: 100},
					{ID: "A2", OpeningBalance: 100},
				},
				Payments: []domain.PaymentInput{
					{ID: "P1", Sender: "A1", Receiver: "A2", Amount: 10},
					{ID: "P1", Sender: "A2", Receiver: "A1", Amount: 10},
				},
			},
			wantCode: "duplicate_payment_id",
		},
		{
			name: "unknown sender account",
			batch: &domain.Batch{
				Accounts: []domain.AccountInput{
					{ID: "A1", OpeningBalance: 100},
					{ID: "A2", OpeningBalance: 100},
				},
				Payments: []domain.PaymentInput{
					{ID: "P1", Sender: "UNKNOWN", Receiver: "A2", Amount: 10},
				},
			},
			wantCode: "unknown_sender_account",
		},
		{
			name: "unknown receiver account",
			batch: &domain.Batch{
				Accounts: []domain.AccountInput{
					{ID: "A1", OpeningBalance: 100},
					{ID: "A2", OpeningBalance: 100},
				},
				Payments: []domain.PaymentInput{
					{ID: "P1", Sender: "A1", Receiver: "UNKNOWN", Amount: 10},
				},
			},
			wantCode: "unknown_receiver_account",
		},
		{
			name: "self transfer forbidden",
			batch: &domain.Batch{
				Accounts: []domain.AccountInput{
					{ID: "A1", OpeningBalance: 100},
					{ID: "A2", OpeningBalance: 100},
				},
				Payments: []domain.PaymentInput{
					{ID: "P1", Sender: "A1", Receiver: "A1", Amount: 10},
				},
			},
			wantCode: "self_transfer_forbidden",
		},
		{
			name: "zero payment amount",
			batch: &domain.Batch{
				Accounts: []domain.AccountInput{
					{ID: "A1", OpeningBalance: 100},
					{ID: "A2", OpeningBalance: 100},
				},
				Payments: []domain.PaymentInput{
					{ID: "P1", Sender: "A1", Receiver: "A2", Amount: 0},
				},
			},
			wantCode: "non_positive_amount",
		},
		{
			name: "negative payment amount",
			batch: &domain.Batch{
				Accounts: []domain.AccountInput{
					{ID: "A1", OpeningBalance: 100},
					{ID: "A2", OpeningBalance: 100},
				},
				Payments: []domain.PaymentInput{
					{ID: "P1", Sender: "A1", Receiver: "A2", Amount: -50},
				},
			},
			wantCode: "non_positive_amount",
		},
		{
			name: "unsupported schema version",
			batch: &domain.Batch{
				SchemaVersion: "unknown.v99",
				Accounts: []domain.AccountInput{
					{ID: "A1", OpeningBalance: 100},
					{ID: "A2", OpeningBalance: 100},
				},
			},
			wantCode: "unsupported_schema_version",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := domain.ValidateBatch(tc.batch)
			if err == nil {
				t.Fatalf("expected validation error with code %q, got nil", tc.wantCode)
			}
			ve, ok := err.(*domain.ValidationError)
			if !ok {
				t.Fatalf("expected *domain.ValidationError, got %T", err)
			}
			found := false
			for _, fe := range ve.Errors {
				if fe.Code == tc.wantCode {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("expected error code %q in %+v", tc.wantCode, ve.Errors)
			}
		})
	}
}
