package main

import (
	"math"
)

const (
	OUT_DIR                        = "./out"
	EJECTION_BALANCE               = 16000000000
	FAR_FUTURE_EPOCH               = math.MaxUint64
	CHURN_LIMIT_QUOTIENT           = 65536
	MIN_PER_EPOCH_CHURN_LIMIT      = 4
	MAX_SEED_LOOKAHEAD             = 4
	INACTIVITY_SCORE_BIAS          = 4
	INACTIVITY_SCORE_RECOVERY_RATE = 16
	INACTIVITY_PENALTY_QUOTIENT    = 16777216
	MAX_ATTESTATIONS               = 128
	MAX_COMMITTEES_PER_SLOT        = 64
	TARGET_COMMITTEE_SIZE          = 128
)

func main() {
	RunTest(80, 1000000)
}
