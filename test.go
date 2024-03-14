package main

import "fmt"

// Results holds the outcome of a test.
type Results struct {
	OfflinePercent             int
	InactivityLeakStopDays     float64
	FractionTotalBalanceBurned float64
}

// RunTest simulates the blockchain state under specified conditions and returns the results.
func RunTest(offlinePercent int, validatorCount int) (Results, error) {
	state := NewState()

	const initialBalance = 32000000000
	startIdxNonParticipant := (100 - offlinePercent) * validatorCount / 100

	for i := 0; i < validatorCount; i++ {
		state.AddValidator(i < startIdxNonParticipant, initialBalance)
	}

	var (
		minBalances, maxBalances                                                     []uint64
		avgBalances                                                                  []float64
		minInactivityScores, maxInactivityScores                                     []uint64
		avgInactivityScores                                                          []float64
		inactivityLeakStopEpoch                                                      *uint64
		activateValidators, participation, activeBalance, activeParticipatingBalance []float64
	)

	for {
		state.ProcessEpochSinglePass()

		// Record metrics for balances
		min, max, avg := ComputeMinMaxAvg(state.balances[startIdxNonParticipant:])
		minBalances = append(minBalances, min)
		avgBalances = append(avgBalances, avg)
		maxBalances = append(maxBalances, max)

		// Record metrics for inactivity scores
		min, max, avg = ComputeMinMaxAvg(state.inactivityScores[startIdxNonParticipant:])
		minInactivityScores = append(minInactivityScores, min)
		avgInactivityScores = append(avgInactivityScores, avg)
		maxInactivityScores = append(maxInactivityScores, max)

		// Check for inactivity leak stop
		if !state.IsInInactivityLeak() && inactivityLeakStopEpoch == nil {
			epoch := state.epoch
			inactivityLeakStopEpoch = &epoch
		}

		// Record various metrics for analysis
		activateValidators = append(activateValidators, float64(state.activeCountPrevEpoch))
		if state.activeBalance > 0 { // Avoid division by zero
			participation = append(participation, float64(state.activeParticipatingBalance)/float64(state.activeBalance))
		} else {
			participation = append(participation, 0)
		}
		activeBalance = append(activeBalance, float64(state.activeBalance))
		activeParticipatingBalance = append(activeParticipatingBalance, float64(state.activeParticipatingBalance))

		if state.epoch%100 == 0 {
			fmt.Printf("epoch %v  balance %v   participating %v\n", state.epoch, float64(state.activeBalance), float64(state.activeParticipatingBalance))
		}

		// Break the loop if finality is recovered and no more penalties are applied to active validators
		if !state.IsInInactivityLeak() && state.maxActiveInactivityScore == 0 {
			break
		}
	}

	// Derive inactivity leak stop epoch and convert to days
	var inactivityLeakStopEpochUint uint64
	if inactivityLeakStopEpoch != nil {
		inactivityLeakStopEpochUint = *inactivityLeakStopEpoch
	}
	inactivityLeakStopDays := float64(inactivityLeakStopEpochUint*32*12) / (60 * 60 * 24)

	// Calculate total balance burned
	totalBalanceBurned := uint64(0)
	for _, balance := range state.balances {
		if initialBalance > balance {
			totalBalanceBurned += initialBalance - balance
		}
	}
	fractionTotalBalanceBurned := float64(totalBalanceBurned) / float64(state.totalInitialBalance)

	// Print summary
	fmt.Printf("\n")
	fmt.Printf("offline_percent:                %d\n", offlinePercent)
	fmt.Printf("inactivity_leak_stop:           %d epochs\n", inactivityLeakStopEpochUint)
	fmt.Printf("inactivity_leak_stop:           %f days\n", inactivityLeakStopDays)
	fmt.Printf("fraction_eth_burned:            %f\n", fractionTotalBalanceBurned)
	fmt.Printf("balances[end]:                  %d %f %d\n", minBalances[len(minBalances)-1], avgBalances[len(avgBalances)-1], maxBalances[len(maxBalances)-1])
	fmt.Printf("inactivity_scores[end]:         %d %f %d\n", minInactivityScores[len(minInactivityScores)-1], avgInactivityScores[len(avgInactivityScores)-1], maxInactivityScores[len(maxInactivityScores)-1])
	fmt.Printf("activate_validators[end]:       %f\n", activateValidators[len(activateValidators)-1])
	fmt.Printf("participation[end]:             %f\n", participation[len(participation)-1])
	fmt.Printf("state_end_epoch:                %d\n", state.epoch)
	fmt.Printf("exit_queue_epoch:               %d\n", state.exitQueueEpoch)

	// Prepare and return results
	results := Results{
		OfflinePercent:             offlinePercent,
		InactivityLeakStopDays:     inactivityLeakStopDays,
		FractionTotalBalanceBurned: fractionTotalBalanceBurned,
	}

	return results, nil
}
