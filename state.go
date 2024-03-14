package main

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
	activeParticipatingBalance uint64
	maxActiveInactivityScore   uint64
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
	s.activeCountPrevEpoch++
	if participating {
		s.participatingCount++
		s.activeParticipatingBalance += initialBalance
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

func (s *State) ProcessInactivityUpdatesSinglePass(index int) {
	// Increase the inactivity score of inactive validators
	if s.IsParticipating(index) {
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

func (s *State) ProcessRewardsAndPenaltiesSinglePass(index int) {
	if !s.IsParticipating(index) {
		penaltyNumerator := s.balances[index] * s.inactivityScores[index]
		penaltyDenominator := uint64(INACTIVITY_SCORE_BIAS * INACTIVITY_PENALTY_QUOTIENT)
		s.balances[index] -= penaltyNumerator / penaltyDenominator
	}
}

func (s *State) ProcessEpochSinglePass() {
	var activeCountPrevEpoch uint64 = 0
	var activeBalance uint64 = 0
	var activeParticipatingBalance uint64 = 0
	var maxActiveInactivityScore uint64 = 0
	var previousEpoch uint64 = 0

	if s.epoch > 0 {
		previousEpoch = s.epoch - 1
	}

	for index := range s.validators {
		isActivePrevEpoch := s.validators[index].IsActiveValidator(previousEpoch)
		if isActivePrevEpoch {
			s.ProcessInactivityUpdatesSinglePass(index)
			s.ProcessRewardsAndPenaltiesSinglePass(index)
		}
		s.ProcessRegistryUpdatesSinglePass(index)

		if isActivePrevEpoch {
			activeCountPrevEpoch++
			activeBalance += s.balances[index]
			if s.IsParticipating(index) {
				activeParticipatingBalance += s.balances[index]
			}
			// Track for stopping condition
			if s.inactivityScores[index] > maxActiveInactivityScore {
				maxActiveInactivityScore = s.inactivityScores[index]
			}
		}
	}

	s.epoch++
	s.activeCountPrevEpoch = activeCountPrevEpoch
	s.activeBalance = activeBalance
	s.activeParticipatingBalance = activeParticipatingBalance
	s.maxActiveInactivityScore = maxActiveInactivityScore
}
