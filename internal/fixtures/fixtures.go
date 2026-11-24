package fixtures

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"paymentexplorer/internal/domain"
)

// ExpectedEdge describes an expected dependency edge in a fixture.
type ExpectedEdge struct {
	FromPos  int      `json:"from_pos"`
	ToPos    int      `json:"to_pos"`
	Accounts []string `json:"accounts"`
}

// ExpectedResults captures the authoritative ground-truth outcomes, balances,
// graph properties, and logical makespans for a demonstration fixture.
type ExpectedResults struct {
	Outcomes          map[string]domain.Outcome `json:"outcomes"`
	FinalBalances     map[string]int64          `json:"final_balances"`
	EdgeCount         int                       `json:"edge_count"`
	ExpectedEdges     []ExpectedEdge            `json:"expected_edges,omitempty"`
	CriticalPathLen   int                       `json:"critical_path_len"`
	CriticalPathIDs   []string                  `json:"critical_path_ids"`
	MakespanByWorkers map[int]int64             `json:"makespan_by_workers"`
}

// Fixture bundles a deterministic demonstration batch with its expected ground-truth results.
type Fixture struct {
	Key         string          `json:"key"`
	Filename    string          `json:"filename"`
	Title       string          `json:"title"`
	Description string          `json:"description"`
	Batch       domain.Batch    `json:"batch"`
	Expected    ExpectedResults `json:"expected"`
}

var fixedTimestamp = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

// All returns the complete list of authoritative demonstration and acceptance fixtures
// required by Section 5 and Section 23 of the specification.
func All() []Fixture {
	return []Fixture{
		EmptyBatchFixture(),
		SingleSuccessFixture(),
		SingleRejectionFixture(),
		WorkedExample6PaymentsFixture(),
		IndependentPaymentsFixture(),
		SharedAccountChainFixture(),
		EarlierCreditFundingFixture(),
		NoRetryAfterLaterCreditFixture(),
		RepeatedPairDedupFixture(),
	}
}

// ByKey looks up a built-in fixture by its key.
func ByKey(key string) (Fixture, bool) {
	for _, f := range All() {
		if f.Key == key {
			return f, true
		}
	}
	return Fixture{}, false
}

// WriteDatasets writes all valid fixtures as formatted versioned JSON files to dir.
func WriteDatasets(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir datasets dir: %w", err)
	}
	for _, f := range All() {
		data, err := json.MarshalIndent(f, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal fixture %s: %w", f.Key, err)
		}
		path := filepath.Join(dir, f.Filename)
		if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
			return fmt.Errorf("write fixture %s: %w", path, err)
		}
	}
	return nil
}

// 1. Empty batch (N = 0).
func EmptyBatchFixture() Fixture {
	b := domain.Batch{
		ID:            "fixture-01-empty-batch",
		SchemaVersion: domain.CurrentSchemaVersion,
		Name:          "01. Empty Batch (N=0)",
		Description:   "Edge case with 0 payments and 2 accounts. Makespan and critical path are 0; speedup and efficiency are not applicable.",
		Currency:      domain.DefaultCurrency,
		CreatedAt:     fixedTimestamp,
		Configuration: domain.ScenarioConfig{
			Scenario:       domain.ScenarioCustom,
			AccountCount:   2,
			PaymentCount:   0,
			OpeningBalance: 100,
		},
		Accounts: []domain.AccountInput{
			{ID: "A", OpeningBalance: 100},
			{ID: "B", OpeningBalance: 100},
		},
		Payments:          []domain.PaymentInput{},
		TotalOpeningFunds: 200,
	}
	return Fixture{
		Key:         "empty_batch",
		Filename:    "01_empty_batch.json",
		Title:       b.Name,
		Description: b.Description,
		Batch:       b,
		Expected: ExpectedResults{
			Outcomes:          map[string]domain.Outcome{},
			FinalBalances:     map[string]int64{"A": 100, "B": 100},
			EdgeCount:         0,
			CriticalPathLen:   0,
			CriticalPathIDs:   []string{},
			MakespanByWorkers: map[int]int64{1: 0, 2: 0, 4: 0},
		},
	}
}

// 2. Single successful payment.
func SingleSuccessFixture() Fixture {
	b := domain.Batch{
		ID:            "fixture-02-single-success",
		SchemaVersion: domain.CurrentSchemaVersion,
		Name:          "02. Single Successful Payment",
		Description:   "A single payment P1: A -> B of amount 25 with opening balances A=100, B=50.",
		Currency:      domain.DefaultCurrency,
		CreatedAt:     fixedTimestamp,
		Configuration: domain.ScenarioConfig{
			Scenario:       domain.ScenarioCustom,
			AccountCount:   2,
			PaymentCount:   1,
			OpeningBalance: 100,
			MinAmount:      25,
			MaxAmount:      25,
		},
		Accounts: []domain.AccountInput{
			{ID: "A", OpeningBalance: 100},
			{ID: "B", OpeningBalance: 50},
		},
		Payments: []domain.PaymentInput{
			{Position: 0, ID: "P1", Sender: "A", Receiver: "B", Amount: 25},
		},
		TotalOpeningFunds: 150,
	}
	return Fixture{
		Key:         "single_success",
		Filename:    "02_single_success.json",
		Title:       b.Name,
		Description: b.Description,
		Batch:       b,
		Expected: ExpectedResults{
			Outcomes:          map[string]domain.Outcome{"P1": domain.OutcomeSucceeded},
			FinalBalances:     map[string]int64{"A": 75, "B": 75},
			EdgeCount:         0,
			CriticalPathLen:   1,
			CriticalPathIDs:   []string{"P1"},
			MakespanByWorkers: map[int]int64{1: 1, 2: 1, 4: 1},
		},
	}
}

// 3. Single insufficient-funds rejection.
func SingleRejectionFixture() Fixture {
	b := domain.Batch{
		ID:            "fixture-03-single-rejection",
		SchemaVersion: domain.CurrentSchemaVersion,
		Name:          "03. Single Insufficient-Funds Rejection",
		Description:   "A single payment P1: A -> B of amount 50 when A only has 30. Rejected without changing either balance, consuming 1 logical unit.",
		Currency:      domain.DefaultCurrency,
		CreatedAt:     fixedTimestamp,
		Configuration: domain.ScenarioConfig{
			Scenario:       domain.ScenarioCustom,
			AccountCount:   2,
			PaymentCount:   1,
			OpeningBalance: 30,
			MinAmount:      50,
			MaxAmount:      50,
		},
		Accounts: []domain.AccountInput{
			{ID: "A", OpeningBalance: 30},
			{ID: "B", OpeningBalance: 10},
		},
		Payments: []domain.PaymentInput{
			{Position: 0, ID: "P1", Sender: "A", Receiver: "B", Amount: 50},
		},
		TotalOpeningFunds: 40,
	}
	return Fixture{
		Key:         "single_rejection",
		Filename:    "03_single_rejection.json",
		Title:       b.Name,
		Description: b.Description,
		Batch:       b,
		Expected: ExpectedResults{
			Outcomes:          map[string]domain.Outcome{"P1": domain.OutcomeRejectedInsufficientFunds},
			FinalBalances:     map[string]int64{"A": 30, "B": 10},
			EdgeCount:         0,
			CriticalPathLen:   1,
			CriticalPathIDs:   []string{"P1"},
			MakespanByWorkers: map[int]int64{1: 1, 2: 1},
		},
	}
}

// 4. Section 5 Worked Example (6 payments, 7 accounts A..G).
func WorkedExample6PaymentsFixture() Fixture {
	b := domain.Batch{
		ID:            "fixture-04-worked-example-6",
		SchemaVersion: domain.CurrentSchemaVersion,
		Name:          "04. Teaching Worked Example (6 Payments)",
		Description:   "Authoritative 6-payment example from Section 5: P1(A->B), P2(C->D), P3(B->E), P4(F->G), P5(D->F), P6(E->G), amount 10, opening balance 100.",
		Currency:      domain.DefaultCurrency,
		CreatedAt:     fixedTimestamp,
		Configuration: domain.ScenarioConfig{
			Scenario:       domain.ScenarioCustom,
			AccountCount:   7,
			PaymentCount:   6,
			OpeningBalance: 100,
			MinAmount:      10,
			MaxAmount:      10,
		},
		Accounts: []domain.AccountInput{
			{ID: "A", OpeningBalance: 100},
			{ID: "B", OpeningBalance: 100},
			{ID: "C", OpeningBalance: 100},
			{ID: "D", OpeningBalance: 100},
			{ID: "E", OpeningBalance: 100},
			{ID: "F", OpeningBalance: 100},
			{ID: "G", OpeningBalance: 100},
		},
		Payments: []domain.PaymentInput{
			{Position: 0, ID: "P1", Sender: "A", Receiver: "B", Amount: 10},
			{Position: 1, ID: "P2", Sender: "C", Receiver: "D", Amount: 10},
			{Position: 2, ID: "P3", Sender: "B", Receiver: "E", Amount: 10},
			{Position: 3, ID: "P4", Sender: "F", Receiver: "G", Amount: 10},
			{Position: 4, ID: "P5", Sender: "D", Receiver: "F", Amount: 10},
			{Position: 5, ID: "P6", Sender: "E", Receiver: "G", Amount: 10},
		},
		TotalOpeningFunds: 700,
	}
	return Fixture{
		Key:         "worked_example_6",
		Filename:    "04_worked_example_6_payments.json",
		Title:       b.Name,
		Description: b.Description,
		Batch:       b,
		Expected: ExpectedResults{
			Outcomes: map[string]domain.Outcome{
				"P1": domain.OutcomeSucceeded,
				"P2": domain.OutcomeSucceeded,
				"P3": domain.OutcomeSucceeded,
				"P4": domain.OutcomeSucceeded,
				"P5": domain.OutcomeSucceeded,
				"P6": domain.OutcomeSucceeded,
			},
			FinalBalances: map[string]int64{
				"A": 90,
				"B": 100,
				"C": 90,
				"D": 100,
				"E": 100,
				"F": 100,
				"G": 120,
			},
			EdgeCount: 5,
			ExpectedEdges: []ExpectedEdge{
				{FromPos: 0, ToPos: 2, Accounts: []string{"B"}}, // P1 -> P3 via B
				{FromPos: 1, ToPos: 4, Accounts: []string{"D"}}, // P2 -> P5 via D
				{FromPos: 3, ToPos: 4, Accounts: []string{"F"}}, // P4 -> P5 via F
				{FromPos: 2, ToPos: 5, Accounts: []string{"E"}}, // P3 -> P6 via E
				{FromPos: 3, ToPos: 5, Accounts: []string{"G"}}, // P4 -> P6 via G
			},
			CriticalPathLen: 3,
			CriticalPathIDs: []string{"P1", "P3", "P6"},
			MakespanByWorkers: map[int]int64{
				1: 6,
				2: 3,
				3: 3,
				4: 3,
				6: 3,
			},
		},
	}
}

// 5. Completely independent payments (4 payments, 8 disjoint accounts).
func IndependentPaymentsFixture() Fixture {
	b := domain.Batch{
		ID:            "fixture-05-independent-payments",
		SchemaVersion: domain.CurrentSchemaVersion,
		Name:          "05. Disjoint Independent Payments (4 Payments, 8 Accounts)",
		Description:   "Four payments using 8 completely disjoint accounts. Zero dependency edges, critical path length 1, ideal parallel speedup.",
		Currency:      domain.DefaultCurrency,
		CreatedAt:     fixedTimestamp,
		Configuration: domain.ScenarioConfig{
			Scenario:       domain.ScenarioIndependent,
			AccountCount:   8,
			PaymentCount:   4,
			OpeningBalance: 100,
			MinAmount:      20,
			MaxAmount:      20,
		},
		Accounts: []domain.AccountInput{
			{ID: "A1", OpeningBalance: 100},
			{ID: "A2", OpeningBalance: 100},
			{ID: "A3", OpeningBalance: 100},
			{ID: "A4", OpeningBalance: 100},
			{ID: "A5", OpeningBalance: 100},
			{ID: "A6", OpeningBalance: 100},
			{ID: "A7", OpeningBalance: 100},
			{ID: "A8", OpeningBalance: 100},
		},
		Payments: []domain.PaymentInput{
			{Position: 0, ID: "P1", Sender: "A1", Receiver: "A2", Amount: 20},
			{Position: 1, ID: "P2", Sender: "A3", Receiver: "A4", Amount: 20},
			{Position: 2, ID: "P3", Sender: "A5", Receiver: "A6", Amount: 20},
			{Position: 3, ID: "P4", Sender: "A7", Receiver: "A8", Amount: 20},
		},
		TotalOpeningFunds: 800,
	}
	return Fixture{
		Key:         "independent_payments",
		Filename:    "05_independent_payments.json",
		Title:       b.Name,
		Description: b.Description,
		Batch:       b,
		Expected: ExpectedResults{
			Outcomes: map[string]domain.Outcome{
				"P1": domain.OutcomeSucceeded,
				"P2": domain.OutcomeSucceeded,
				"P3": domain.OutcomeSucceeded,
				"P4": domain.OutcomeSucceeded,
			},
			FinalBalances: map[string]int64{
				"A1": 80, "A2": 120,
				"A3": 80, "A4": 120,
				"A5": 80, "A6": 120,
				"A7": 80, "A8": 120,
			},
			EdgeCount:       0,
			CriticalPathLen: 1,
			CriticalPathIDs: []string{"P1"},
			MakespanByWorkers: map[int]int64{
				1: 4,
				2: 2,
				4: 1,
			},
		},
	}
}

// 6. All payments sharing one common sender account.
func SharedAccountChainFixture() Fixture {
	b := domain.Batch{
		ID:            "fixture-06-shared-account-chain",
		SchemaVersion: domain.CurrentSchemaVersion,
		Name:          "06. Shared Account Serialization Chain (4 Payments)",
		Description:   "All four payments share common sender account A1 (with opening balance 50, each transfer 20). Forms a linear chain P1->P2->P3->P4 where P1 and P2 succeed and P3 and P4 are rejected for insufficient funds.",
		Currency:      domain.DefaultCurrency,
		CreatedAt:     fixedTimestamp,
		Configuration: domain.ScenarioConfig{
			Scenario:       domain.ScenarioSharedAccount,
			AccountCount:   5,
			PaymentCount:   4,
			OpeningBalance: 50,
			MinAmount:      20,
			MaxAmount:      20,
			CommonAccount:  "A1",
		},
		Accounts: []domain.AccountInput{
			{ID: "A1", OpeningBalance: 50},
			{ID: "A2", OpeningBalance: 50},
			{ID: "A3", OpeningBalance: 50},
			{ID: "A4", OpeningBalance: 50},
			{ID: "A5", OpeningBalance: 50},
		},
		Payments: []domain.PaymentInput{
			{Position: 0, ID: "P1", Sender: "A1", Receiver: "A2", Amount: 20},
			{Position: 1, ID: "P2", Sender: "A1", Receiver: "A3", Amount: 20},
			{Position: 2, ID: "P3", Sender: "A1", Receiver: "A4", Amount: 20},
			{Position: 3, ID: "P4", Sender: "A1", Receiver: "A5", Amount: 20},
		},
		TotalOpeningFunds: 250,
	}
	return Fixture{
		Key:         "shared_account_chain",
		Filename:    "06_shared_account_chain.json",
		Title:       b.Name,
		Description: b.Description,
		Batch:       b,
		Expected: ExpectedResults{
			Outcomes: map[string]domain.Outcome{
				"P1": domain.OutcomeSucceeded,
				"P2": domain.OutcomeSucceeded,
				"P3": domain.OutcomeRejectedInsufficientFunds,
				"P4": domain.OutcomeRejectedInsufficientFunds,
			},
			FinalBalances: map[string]int64{
				"A1": 10,
				"A2": 70,
				"A3": 70,
				"A4": 50,
				"A5": 50,
			},
			EdgeCount: 3,
			ExpectedEdges: []ExpectedEdge{
				{FromPos: 0, ToPos: 1, Accounts: []string{"A1"}},
				{FromPos: 1, ToPos: 2, Accounts: []string{"A1"}},
				{FromPos: 2, ToPos: 3, Accounts: []string{"A1"}},
			},
			CriticalPathLen: 4,
			CriticalPathIDs: []string{"P1", "P2", "P3", "P4"},
			MakespanByWorkers: map[int]int64{
				1: 4,
				2: 4,
				4: 4,
			},
		},
	}
}

// 7. Specific earlier-funding example from Section 23.
// Opening balances: A=0, B=0, C=10. P1: C->A (10), P2: A->B (10). Both succeed.
func EarlierCreditFundingFixture() Fixture {
	b := domain.Batch{
		ID:            "fixture-07-earlier-credit-funding",
		SchemaVersion: domain.CurrentSchemaVersion,
		Name:          "07. Earlier Credit Funds Later Payment",
		Description:   "Section 23 fixture: A=0, B=0, C=10. P1(C->A, 10) credits A before P2(A->B, 10) executes. Because P1->P2 dependency on A is respected, both succeed.",
		Currency:      domain.DefaultCurrency,
		CreatedAt:     fixedTimestamp,
		Configuration: domain.ScenarioConfig{
			Scenario:     domain.ScenarioCustom,
			AccountCount: 3,
			PaymentCount: 2,
			MinAmount:    10,
			MaxAmount:    10,
		},
		Accounts: []domain.AccountInput{
			{ID: "A", OpeningBalance: 0},
			{ID: "B", OpeningBalance: 0},
			{ID: "C", OpeningBalance: 10},
		},
		Payments: []domain.PaymentInput{
			{Position: 0, ID: "P1", Sender: "C", Receiver: "A", Amount: 10},
			{Position: 1, ID: "P2", Sender: "A", Receiver: "B", Amount: 10},
		},
		TotalOpeningFunds: 10,
	}
	return Fixture{
		Key:         "earlier_credit_funding",
		Filename:    "07_earlier_credit_funding.json",
		Title:       b.Name,
		Description: b.Description,
		Batch:       b,
		Expected: ExpectedResults{
			Outcomes: map[string]domain.Outcome{
				"P1": domain.OutcomeSucceeded,
				"P2": domain.OutcomeSucceeded,
			},
			FinalBalances: map[string]int64{
				"A": 0,
				"B": 10,
				"C": 0,
			},
			EdgeCount: 1,
			ExpectedEdges: []ExpectedEdge{
				{FromPos: 0, ToPos: 1, Accounts: []string{"A"}},
			},
			CriticalPathLen:   2,
			CriticalPathIDs:   []string{"P1", "P2"},
			MakespanByWorkers: map[int]int64{1: 2, 2: 2},
		},
	}
}

// 8. Specific no-retry example from Section 23.
// Opening balances: A=0, B=0, C=10. P1: A->B (10), P2: C->A (10).
// Expected: P1 rejected, P2 succeeded, final A=10, B=0, C=0, critical path 2, makespan 2.
func NoRetryAfterLaterCreditFixture() Fixture {
	b := domain.Batch{
		ID:            "fixture-08-no-retry-after-credit",
		SchemaVersion: domain.CurrentSchemaVersion,
		Name:          "08. Rejected Payment Followed by Later Credit (No Retry)",
		Description:   "Section 23 fixture: A=0, B=0, C=10. P1(A->B, 10) is attempted first and rejected for insufficient funds (consuming 1 unit). It releases P2(C->A, 10) which succeeds. P1 is never retried.",
		Currency:      domain.DefaultCurrency,
		CreatedAt:     fixedTimestamp,
		Configuration: domain.ScenarioConfig{
			Scenario:     domain.ScenarioCustom,
			AccountCount: 3,
			PaymentCount: 2,
			MinAmount:    10,
			MaxAmount:    10,
		},
		Accounts: []domain.AccountInput{
			{ID: "A", OpeningBalance: 0},
			{ID: "B", OpeningBalance: 0},
			{ID: "C", OpeningBalance: 10},
		},
		Payments: []domain.PaymentInput{
			{Position: 0, ID: "P1", Sender: "A", Receiver: "B", Amount: 10},
			{Position: 1, ID: "P2", Sender: "C", Receiver: "A", Amount: 10},
		},
		TotalOpeningFunds: 10,
	}
	return Fixture{
		Key:         "no_retry_after_credit",
		Filename:    "08_no_retry_after_later_credit.json",
		Title:       b.Name,
		Description: b.Description,
		Batch:       b,
		Expected: ExpectedResults{
			Outcomes: map[string]domain.Outcome{
				"P1": domain.OutcomeRejectedInsufficientFunds,
				"P2": domain.OutcomeSucceeded,
			},
			FinalBalances: map[string]int64{
				"A": 10,
				"B": 0,
				"C": 0,
			},
			EdgeCount: 1,
			ExpectedEdges: []ExpectedEdge{
				{FromPos: 0, ToPos: 1, Accounts: []string{"A"}},
			},
			CriticalPathLen:   2,
			CriticalPathIDs:   []string{"P1", "P2"},
			MakespanByWorkers: map[int]int64{1: 2, 2: 2},
		},
	}
}

// 9. Repeated account pair testing predecessor-edge deduplication.
func RepeatedPairDedupFixture() Fixture {
	b := domain.Batch{
		ID:            "fixture-09-repeated-pair-dedup",
		SchemaVersion: domain.CurrentSchemaVersion,
		Name:          "09. Repeated Account Pair (Edge Deduplication)",
		Description:   "P1(A->B, 10), P2(B->A, 10), P3(A->B, 10). Both sender and receiver of P2 were last touched by P1, and both accounts of P3 were last touched by P2. Tests that only 1 deduplicated edge is created per step with both account labels [A, B].",
		Currency:      domain.DefaultCurrency,
		CreatedAt:     fixedTimestamp,
		Configuration: domain.ScenarioConfig{
			Scenario:       domain.ScenarioCustom,
			AccountCount:   2,
			PaymentCount:   3,
			OpeningBalance: 50,
			MinAmount:      10,
			MaxAmount:      10,
		},
		Accounts: []domain.AccountInput{
			{ID: "A", OpeningBalance: 50},
			{ID: "B", OpeningBalance: 50},
		},
		Payments: []domain.PaymentInput{
			{Position: 0, ID: "P1", Sender: "A", Receiver: "B", Amount: 10},
			{Position: 1, ID: "P2", Sender: "B", Receiver: "A", Amount: 10},
			{Position: 2, ID: "P3", Sender: "A", Receiver: "B", Amount: 10},
		},
		TotalOpeningFunds: 100,
	}
	return Fixture{
		Key:         "repeated_pair_dedup",
		Filename:    "09_repeated_pair_dedup.json",
		Title:       b.Name,
		Description: b.Description,
		Batch:       b,
		Expected: ExpectedResults{
			Outcomes: map[string]domain.Outcome{
				"P1": domain.OutcomeSucceeded,
				"P2": domain.OutcomeSucceeded,
				"P3": domain.OutcomeSucceeded,
			},
			FinalBalances: map[string]int64{
				"A": 40,
				"B": 60,
			},
			EdgeCount: 2,
			ExpectedEdges: []ExpectedEdge{
				{FromPos: 0, ToPos: 1, Accounts: []string{"A", "B"}},
				{FromPos: 1, ToPos: 2, Accounts: []string{"A", "B"}},
			},
			CriticalPathLen:   3,
			CriticalPathIDs:   []string{"P1", "P2", "P3"},
			MakespanByWorkers: map[int]int64{1: 3, 2: 3},
		},
	}
}
