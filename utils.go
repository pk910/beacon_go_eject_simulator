package main

func MaxUint64(x, y uint64) uint64 {
	if x > y {
		return x
	}
	return y
}

func MinUint64(x, y uint64) uint64 {
	if x < y {
		return x
	}
	return y
}

// ComputeMinMaxAvg calculates the minimum, maximum, and average of a slice of uint64.
// It panics if the slice is empty.
func ComputeMinMaxAvg(numbers []uint64) (min uint64, max uint64, avg float64) {
	if len(numbers) == 0 {
		panic("slice is empty")
	}

	min, max = numbers[0], numbers[0]
	sum := uint64(0)
	for _, number := range numbers {
		if number < min {
			min = number
		}
		if number > max {
			max = number
		}
		sum += number
	}
	avg = float64(sum) / float64(len(numbers))
	return
}

func computeActivationExitEpoch(epoch uint64) uint64 {
	return epoch + 1 + MAX_SEED_LOOKAHEAD
}
