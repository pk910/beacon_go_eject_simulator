package main

import (
	"fmt"
	"math"
)

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
		lastParticipation                                                            float64
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
			participation = append(participation, 0.4)
		}
		activeBalance = append(activeBalance, float64(state.activeBalance))
		activeParticipatingBalance = append(activeParticipatingBalance, float64(state.activeParticipatingBalance))

		participation := float64(state.activeParticipatingBalance) * 100 / float64(state.activeBalance)
		if state.epoch%100 == 0 || math.Abs(participation-lastParticipation) >= 1 {
			lastParticipation = participation
			fmt.Printf(
				"epoch %v   active %v / %v ETH   online: %v ETH   participation: %.2f%%    att-misses: %v (prob.: %.2f%%)\n",
				state.epoch,
				state.activeCountPrevEpoch,
				state.activeBalance/1000000000,
				state.activeParticipatingBalance/1000000000,
				lastParticipation,
				state.capacityMisses,
				state.inclusionProbability*100,
			)
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

	// get balances from online / offline validators
	balancesOnline := []uint64{}
	balancesOffline := []uint64{}

	for i := 0; i < validatorCount; i++ {
		if state.IsParticipating(i) {
			balancesOnline = append(balancesOnline, state.balances[i])
		} else {
			balancesOffline = append(balancesOffline, state.balances[i])
		}
	}

	minBalanceOnline, maxBalanceOnline, avgBalanceOnline := ComputeMinMaxAvg(balancesOnline)
	minBalanceOffline, maxBalanceOffline, avgBalanceOffline := ComputeMinMaxAvg(balancesOffline)

	// Print summary
	fmt.Printf("\n")
	fmt.Printf("offline_percent:                %d\n", offlinePercent)
	fmt.Printf("inactivity_leak_stop:           %d epochs\n", inactivityLeakStopEpochUint)
	fmt.Printf("inactivity_leak_stop:           %f days\n", inactivityLeakStopDays)
	fmt.Printf("fraction_eth_burned:            %.2f%%\n", fractionTotalBalanceBurned*100)
	fmt.Printf("balances[end]:                  %.2f ETH, %.2f ETH, %.2f ETH\n", float64(minBalances[len(minBalances)-1])/1000000000.0, avgBalances[len(avgBalances)-1]/1000000000.0, float64(maxBalances[len(maxBalances)-1])/1000000000.0)
	fmt.Printf("balances_online[end]:           %.2f ETH, %.2f ETH, %.2f ETH\n", float64(minBalanceOnline)/1000000000.0, avgBalanceOnline/1000000000.0, float64(maxBalanceOnline)/1000000000.0)
	fmt.Printf("balances_offline[end]:          %.2f ETH, %.2f ETH, %.2f ETH\n", float64(minBalanceOffline)/1000000000.0, avgBalanceOffline/1000000000.0, float64(maxBalanceOffline)/1000000000.0)
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
