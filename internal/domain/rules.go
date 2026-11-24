package domain

import "fmt"

// ProcessTransfer executes the indivisible payment settlement operation on sender and receiver balances.
// Authoritative rules:
//   - Check sender funds.
//   - If senderBalance < amount, return OutcomeRejectedInsufficientFunds and leave both balances unchanged.
//   - Otherwise, debit sender and credit receiver atomically using checked int64 arithmetic and return OutcomeSucceeded.
//   - No random failures, fees, credit facilities, or retries are permitted.
func ProcessTransfer(senderBalance, receiverBalance, amount int64) (newSender, newReceiver int64, outcome Outcome, err error) {
	if amount <= 0 {
		return senderBalance, receiverBalance, "", fmt.Errorf("internal error: transfer amount must be positive, got %d", amount)
	}
	if senderBalance < 0 || receiverBalance < 0 {
		return senderBalance, receiverBalance, "", fmt.Errorf("internal error: negative account balance encountered (sender=%d, receiver=%d)", senderBalance, receiverBalance)
	}

	if senderBalance < amount {
		return senderBalance, receiverBalance, OutcomeRejectedInsufficientFunds, nil
	}

	credited, ok := CheckedAddInt64(receiverBalance, amount)
	if !ok {
		return senderBalance, receiverBalance, "", fmt.Errorf("internal error: int64 overflow crediting receiver (%d + %d)", receiverBalance, amount)
	}

	return senderBalance - amount, credited, OutcomeSucceeded, nil
}
