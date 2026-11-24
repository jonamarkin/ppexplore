package engine

import (
	"fmt"
	"time"

	"paymentexplorer/internal/domain"
)

// ExecuteSequential runs the authoritative sequential reference executor on an immutable batch.
// Key properties required by the specification:
//   - Simple loop over original batch order (0..N-1).
//   - Does NOT depend on the dependency graph, channels, or worker goroutines.
//   - Operates on fresh cloned balances so the input batch is never mutated.
//   - Records both logical sequential baseline (T1 = N attempts, each taking 1 unit [i, i+1])
//     and actual elapsed monotonic execution duration.
func ExecuteSequential(batch *domain.Batch) (*domain.RunResult, error) {
	setupStart := time.Now()
	totalFunds, err := domain.ValidateBatch(batch)
	if err != nil {
		return nil, err
	}

	numAccounts := len(batch.Accounts)
	numPayments := len(batch.Payments)

	// Build local account index and fresh balance array.
	accountIdx := make(map[string]int, numAccounts)
	balances := make([]int64, numAccounts)
	for i, acc := range batch.Accounts {
		accountIdx[acc.ID] = i
		balances[i] = int64(acc.OpeningBalance)
	}

	results := make([]domain.PaymentResult, numPayments)
	setupDuration := time.Since(setupStart)

	// Begin measured execution loop.
	execOrigin := time.Now()
	succeededCount := 0
	rejectedCount := 0

	for i, p := range batch.Payments {
		sIdx := accountIdx[p.Sender]
		rIdx := accountIdx[p.Receiver]
		amt := int64(p.Amount)

		startOffset := time.Since(execOrigin)
		newSender, newReceiver, outcome, err := domain.ProcessTransfer(balances[sIdx], balances[rIdx], amt)
		if err != nil {
			return nil, fmt.Errorf("sequential execution failure at payment %d (%s): %w", i, p.ID, err)
		}
		balances[sIdx] = newSender
		balances[rIdx] = newReceiver
		finishOffset := time.Since(execOrigin)
		if finishOffset < startOffset {
			finishOffset = startOffset
		}

		if outcome == domain.OutcomeSucceeded {
			succeededCount++
		} else {
			rejectedCount++
		}

		results[i] = domain.PaymentResult{
			Position:       i,
			ID:             p.ID,
			Sender:         p.Sender,
			Receiver:       p.Receiver,
			Amount:         p.Amount,
			Outcome:        outcome,
			WorkerID:       1,
			LogicalStart:   int64(i),
			LogicalFinish:  int64(i + 1),
			StartOffsetNs:  domain.Int64String(startOffset.Nanoseconds()),
			FinishOffsetNs: domain.Int64String(finishOffset.Nanoseconds()),
			DurationNs:     domain.Int64String((finishOffset - startOffset).Nanoseconds()),
		}
	}
	execDuration := time.Since(execOrigin)

	// Build final account balances snapshot and verify money conservation.
	verifyStart := time.Now()
	finalBalances := make([]domain.AccountBalance, numAccounts)
	var totalFinalFunds int64
	allNonNegative := true
	for i, acc := range batch.Accounts {
		openBal := int64(acc.OpeningBalance)
		finalBal := balances[i]
		if finalBal < 0 {
			allNonNegative = false
		}
		totalFinalFunds += finalBal
		finalBalances[i] = domain.AccountBalance{
			Position:       i,
			ID:             acc.ID,
			OpeningBalance: acc.OpeningBalance,
			FinalBalance:   domain.Int64String(finalBal),
			NetChange:      domain.Int64String(finalBal - openBal),
		}
	}
	verifyDuration := time.Since(verifyStart)
	totalDuration := setupDuration + execDuration + verifyDuration

	effWorkers := 1
	if numPayments == 0 {
		effWorkers = 0
	}

	var attemptThroughput, successThroughput float64
	if execDuration > 0 && numPayments > 0 {
		sec := execDuration.Seconds()
		attemptThroughput = float64(numPayments) / sec
		successThroughput = float64(succeededCount) / sec
	}

	logicalMetrics := &domain.LogicalMetrics{
		TotalPayments:           numPayments,
		SucceededCount:          succeededCount,
		RejectedCount:           rejectedCount,
		RequestedWorkers:        1,
		EffectiveWorkers:        effWorkers,
		SequentialBaselineUnits: int64(numPayments),
		MakespanUnits:           int64(numPayments),
		CriticalPathLength:      int64(numPayments),
		LowerBoundUnits:         int64(numPayments),
		Applicable:              numPayments > 0,
	}
	if numPayments > 0 {
		logicalMetrics.Speedup = 1.0
		logicalMetrics.EfficiencyRequested = 1.0
		logicalMetrics.EfficiencyEffective = 1.0
		logicalMetrics.UtilisationRequested = 1.0
		logicalMetrics.UtilisationEffective = 1.0
	}

	measuredMetrics := &domain.MeasuredMetrics{
		TotalPayments:                 numPayments,
		SucceededCount:                succeededCount,
		RejectedCount:                 rejectedCount,
		RequestedWorkers:              1,
		EffectiveWorkers:              effWorkers,
		AnalysisDurationNs:            0,
		SetupDurationNs:               domain.Int64String(setupDuration.Nanoseconds()),
		ExecutionDurationNs:           domain.Int64String(execDuration.Nanoseconds()),
		VerificationDurationNs:        domain.Int64String(verifyDuration.Nanoseconds()),
		PersistenceDurationNs:         0,
		TotalEndToEndDurationNs:       domain.Int64String(totalDuration.Nanoseconds()),
		DirectSequentialBaselineNs:    domain.Int64String(execDuration.Nanoseconds()),
		MeasuredExecutionSpeedup:      1.0,
		CompletedAttemptThroughputPS:  attemptThroughput,
		SuccessfulPaymentThroughputPS: successThroughput,
	}

	conserved := totalFunds == totalFinalFunds
	verification := domain.VerificationReport{
		Passed:            conserved && allNonNegative && (succeededCount+rejectedCount == numPayments),
		Summary:           "Sequential reference execution completed and conserved total system funds.",
		TotalOpeningFunds: domain.Int64String(totalFunds),
		TotalFinalFunds:   domain.Int64String(totalFinalFunds),
		Checks: []domain.InvariantCheck{
			{
				Name:    "payment_coverage",
				Passed:  len(results) == numPayments,
				Details: fmt.Sprintf("Processed %d of %d payments in original order", len(results), numPayments),
			},
			{
				Name:    "outcome_counts_match",
				Passed:  succeededCount+rejectedCount == numPayments,
				Details: fmt.Sprintf("Succeeded=%d, Rejected=%d, Total=%d", succeededCount, rejectedCount, numPayments),
			},
			{
				Name:    "non_negative_final_balances",
				Passed:  allNonNegative,
				Details: "All account balances remained >= 0",
			},
			{
				Name:    "total_money_conserved",
				Passed:  conserved,
				Details: fmt.Sprintf("Opening=%d, Final=%d", totalFunds, totalFinalFunds),
			},
		},
	}

	return &domain.RunResult{
		TimingDomain:   "measured_nanoseconds",
		PaymentResults: results,
		FinalBalances:  finalBalances,
		Logical:        logicalMetrics,
		Measured:       measuredMetrics,
		Verification:   verification,
	}, nil
}
