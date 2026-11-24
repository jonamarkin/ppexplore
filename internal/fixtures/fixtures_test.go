package fixtures_test

import (
	"os"
	"path/filepath"
	"testing"

	"paymentexplorer/internal/engine"
	"paymentexplorer/internal/fixtures"
)

func TestAllFixturesSequentialReference(t *testing.T) {
	all := fixtures.All()
	if len(all) != 9 {
		t.Fatalf("expected 9 valid acceptance fixtures, got %d", len(all))
	}

	for _, f := range all {
		t.Run(f.Key, func(t *testing.T) {
			batchCopy := f.Batch
			res, err := engine.ExecuteSequential(&batchCopy)
			if err != nil {
				t.Fatalf("ExecuteSequential failed for fixture %s: %v", f.Key, err)
			}

			if !res.Verification.Passed {
				t.Fatalf("Verification failed for fixture %s: %+v", f.Key, res.Verification)
			}

			if len(res.PaymentResults) != len(f.Batch.Payments) {
				t.Fatalf("expected %d payment results, got %d", len(f.Batch.Payments), len(res.PaymentResults))
			}

			// Verify every payment outcome matches expected ground truth.
			for _, pr := range res.PaymentResults {
				wantOutcome, ok := f.Expected.Outcomes[pr.ID]
				if !ok {
					t.Fatalf("unexpected payment ID %q in results", pr.ID)
				}
				if pr.Outcome != wantOutcome {
					t.Errorf("payment %s outcome = %s, want %s", pr.ID, pr.Outcome, wantOutcome)
				}
			}

			// Verify every final account balance matches expected ground truth.
			for _, ab := range res.FinalBalances {
				wantBal, ok := f.Expected.FinalBalances[ab.ID]
				if !ok {
					t.Fatalf("unexpected account ID %q in final balances", ab.ID)
				}
				if int64(ab.FinalBalance) != wantBal {
					t.Errorf("account %s final balance = %d, want %d", ab.ID, ab.FinalBalance, wantBal)
				}
			}

			// Verify sequential makespan equals N.
			wantSeqMakespan := f.Expected.MakespanByWorkers[1]
			if res.Logical.MakespanUnits != wantSeqMakespan {
				t.Errorf("sequential logical makespan = %d, want %d", res.Logical.MakespanUnits, wantSeqMakespan)
			}
		})
	}
}

func TestWriteDatasets(t *testing.T) {
	dir := filepath.Join("..", "..", "datasets")
	if err := fixtures.WriteDatasets(dir); err != nil {
		t.Fatalf("WriteDatasets failed: %v", err)
	}
	for _, f := range fixtures.All() {
		p := filepath.Join(dir, f.Filename)
		info, err := os.Stat(p)
		if err != nil {
			t.Fatalf("expected dataset file %s to exist: %v", p, err)
		}
		if info.Size() == 0 {
			t.Fatalf("dataset file %s is empty", p)
		}
	}
}
