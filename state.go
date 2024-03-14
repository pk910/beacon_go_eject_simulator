package main

import (
	"math/rand"
	"sync"
)

type State struct {
	epoch                      uint64
	validators                 []Validator
	balances                   []uint64
	inactivityScores           []uint64
	participatingCount         uint64
	participating              []bool
	totalInitialBalance        uint64
	exitQueueEpoch             uint64
	exitQueueChurn             uint64
	activeCountPrevEpoch       uint64
	activeBalance              uint64
	totalBalance               uint64
	activeParticipatingBalance uint64
	totalParticipatingBalance  uint64
	maxActiveInactivityScore   uint64
	inclusionProbability       float64
	capacityMisses             uint64
}

func NewState() *State {
	return &State{
		participating:    make([]bool, 0),
		validators:       make([]Validator, 0),
		balances:         make([]uint64, 0),
		inactivityScores: make([]uint64, 0),
	}
}

func (s *State) AddValidator(participating bool, initialBalance uint64) {
	s.validators = append(s.validators, NewValidator())
	s.balances = append(s.balances, initialBalance)
	s.inactivityScores = append(s.inactivityScores, 0)
	s.participating = append(s.participating, participating)
	s.totalInitialBalance += initialBalance
	s.activeBalance += initialBalance
	s.totalBalance += initialBalance
	s.activeCountPrevEpoch++
	if participating {
		s.participatingCount++
		s.activeParticipatingBalance += initialBalance
		s.totalParticipatingBalance += initialBalance
	}
}

func (s *State) IsParticipating(index int) bool {
	return s.participating[index]
}

func (s *State) IsInInactivityLeak() bool {
	return 3*s.activeParticipatingBalance < 2*s.activeBalance
}

func (s *State) GetValidatorChurnLimit() uint64 {
	return MaxUint64(MIN_PER_EPOCH_CHURN_LIMIT, s.activeCountPrevEpoch/CHURN_LIMIT_QUOTIENT)
}

func (s *State) InitiateValidatorExit(index int) {
	validator := &s.validators[index]
	if validator.exitEpoch != FAR_FUTURE_EPOCH {
		return
	}

	minExitEpoch := computeActivationExitEpoch(s.epoch)
	if s.exitQueueEpoch < minExitEpoch {
		s.exitQueueEpoch = minExitEpoch
		s.exitQueueChurn = 0
	}

	if s.exitQueueChurn >= s.GetValidatorChurnLimit() {
		s.exitQueueEpoch++
		s.exitQueueChurn = 0
	}

	s.exitQueueChurn++
	validator.exitEpoch = s.exitQueueEpoch
}

func (s *State) ProcessRegistryUpdatesSinglePass(index int) {
	// Process activation eligibility and ejections
	if s.validators[index].IsActiveValidator(s.epoch) && s.balances[index] <= EJECTION_BALANCE {
		s.InitiateValidatorExit(index)
	}
}

func (s *State) ProcessInactivityUpdatesSinglePass(index int, attIncluded bool) {
	// Increase the inactivity score of inactive validators
	if s.IsParticipating(index) && attIncluded {
		if s.inactivityScores[index] > 0 {
			s.inactivityScores[index] -= MinUint64(1, s.inactivityScores[index])
		}
	} else {
		s.inactivityScores[index] += INACTIVITY_SCORE_BIAS
	}
	// Decrease the inactivity score of all eligible validators during a leak-free epoch
	if !s.IsInInactivityLeak() {
		s.inactivityScores[index] -= MinUint64(INACTIVITY_SCORE_RECOVERY_RATE, s.inactivityScores[index])
	}
}

func (s *State) ProcessRewardsAndPenaltiesSinglePass(index int, attIncluded bool) {
	if !s.IsParticipating(index) || !attIncluded {
		penaltyNumerator := s.balances[index] * s.inactivityScores[index]
		penaltyDenominator := uint64(INACTIVITY_SCORE_BIAS * INACTIVITY_PENALTY_QUOTIENT)
		s.balances[index] -= penaltyNumerator / penaltyDenominator
	}
}

func (state *State) ProcessEffectiveBalanceUpdatesSinglePass(index int) {
	hysteresisIncrement := EFFECTIVE_BALANCE_INCREMENT / HYSTERESIS_QUOTIENT
	downwardThreshold := uint64(hysteresisIncrement * HYSTERESIS_DOWNWARD_MULTIPLIER)
	upwardThreshold := uint64(hysteresisIncrement * HYSTERESIS_UPWARD_MULTIPLIER)

	balance := state.balances[index]
	if balance+downwardThreshold < state.validators[index].EffectiveBalance ||
		state.validators[index].EffectiveBalance+upwardThreshold < balance {
		newEffectiveBalance := balance - (balance % EFFECTIVE_BALANCE_INCREMENT)
		if newEffectiveBalance > MAX_EFFECTIVE_BALANCE {
			state.validators[index].EffectiveBalance = MAX_EFFECTIVE_BALANCE
		} else {
			state.validators[index].EffectiveBalance = newEffectiveBalance
		}
	}
}

type ProcessEpochResult struct {
	activeCountPrevEpoch       uint64
	activeBalance              uint64
	totalBalance               uint64
	activeParticipatingBalance uint64
	totalParticipatingBalance  uint64
	maxActiveInactivityScore   uint64
	capacityMisses             uint64
	inclusionProbability       float64
}

func (s *State) ProcessEpochValidatorRangesSinglePass(startIndex int, count int, previousEpoch uint64) ProcessEpochResult {
	result := ProcessEpochResult{}
	endIndex := startIndex + count

	blockCount := 0
	for slotIdx := 0; slotIdx < SLOTS_PER_EPOCH; slotIdx++ {
		if float64(s.activeBalance)*rand.Float64() <= float64(s.activeParticipatingBalance) {
			blockCount++
		}
	}
	blockAttAggregateCapacity := uint64(blockCount * MAX_ATTESTATIONS)

	activeValidatorCount := s.activeCountPrevEpoch
	committeesPerSlot := activeValidatorCount / SLOTS_PER_EPOCH / TARGET_COMMITTEE_SIZE
	if committeesPerSlot < 1 {
		committeesPerSlot = 1
	}
	if committeesPerSlot > MAX_COMMITTEES_PER_SLOT {
		committeesPerSlot = MAX_COMMITTEES_PER_SLOT
	}
	totalCommittees := committeesPerSlot * SLOTS_PER_EPOCH

	totalCommittees += uint64(float64(totalCommittees) * 0)

	inclusionProbability := float64(1)
	if totalCommittees > blockAttAggregateCapacity {
		inclusionProbability = float64(blockAttAggregateCapacity) / float64(totalCommittees)
	}

	result.inclusionProbability = inclusionProbability

	for index := startIndex; index < endIndex; index++ {
		isActivePrevEpoch := s.validators[index].IsActiveValidator(previousEpoch)
		if isActivePrevEpoch {
			attIncluded := true
			if inclusionProbability < 1 && inclusionProbability < rand.Float64() {
				attIncluded = false
				result.capacityMisses++
			}

			s.ProcessInactivityUpdatesSinglePass(index, attIncluded)
			s.ProcessRewardsAndPenaltiesSinglePass(index, attIncluded)
		}
		s.ProcessRegistryUpdatesSinglePass(index)
		s.ProcessEffectiveBalanceUpdatesSinglePass(index)

		if isActivePrevEpoch {

			result.activeCountPrevEpoch++
			result.totalBalance += s.balances[index]
			result.activeBalance += s.validators[index].EffectiveBalance
			if s.IsParticipating(index) {
				result.activeParticipatingBalance += s.validators[index].EffectiveBalance
				result.totalParticipatingBalance += s.balances[index]
			}
			// Track for stopping condition
			if s.inactivityScores[index] > result.maxActiveInactivityScore {
				result.maxActiveInactivityScore = s.inactivityScores[index]
			}
		}
	}

	return result
}

func (s *State) ProcessEpochSinglePass() {
	var previousEpoch uint64 = 0

	if s.epoch > 0 {
		previousEpoch = s.epoch - 1
	}

	totalIndexes := len(s.validators)
	lastIndex := 0
	results := []ProcessEpochResult{}
	resultMutex := sync.Mutex{}
	resultWg := sync.WaitGroup{}

	for lastIndex < totalIndexes {
		startIndex := lastIndex
		indexCount := 100000
		if startIndex+indexCount > totalIndexes {
			indexCount = totalIndexes - startIndex
		}

		lastIndex = startIndex + indexCount

		resultWg.Add(1)

		go func(startIndex int, count int) {
			defer resultWg.Done()

			result := s.ProcessEpochValidatorRangesSinglePass(startIndex, count, previousEpoch)

			resultMutex.Lock()
			defer resultMutex.Unlock()
			results = append(results, result)
		}(startIndex, indexCount)
	}

	resultWg.Wait()
	s.epoch++

	var activeCountPrevEpoch uint64 = 0
	var activeBalance uint64 = 0
	var totalBalance uint64 = 0
	var activeParticipatingBalance uint64 = 0
	var totalParticipatingBalance uint64 = 0
	var maxActiveInactivityScore uint64 = 0
	var inclusionProbability float64 = 0
	var capacityMisses uint64 = 0

	for _, result := range results {
		activeCountPrevEpoch += result.activeCountPrevEpoch
		activeBalance += result.activeBalance
		totalBalance += result.totalBalance
		activeParticipatingBalance += result.activeParticipatingBalance
		totalParticipatingBalance += result.totalParticipatingBalance
		if result.maxActiveInactivityScore > maxActiveInactivityScore {
			maxActiveInactivityScore = result.maxActiveInactivityScore
		}
		inclusionProbability += result.inclusionProbability
		capacityMisses += result.capacityMisses
	}

	s.activeCountPrevEpoch = activeCountPrevEpoch
	s.activeBalance = activeBalance
	s.totalBalance = totalBalance
	s.activeParticipatingBalance = activeParticipatingBalance
	s.totalParticipatingBalance = totalParticipatingBalance
	s.maxActiveInactivityScore = maxActiveInactivityScore
	s.inclusionProbability = inclusionProbability / float64(len(results))
	s.capacityMisses = capacityMisses
}
