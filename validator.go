package main

type Validator struct {
	exitEpoch uint64
}

func NewValidator() Validator {
	return Validator{exitEpoch: FAR_FUTURE_EPOCH}
}

func (v *Validator) IsActiveValidator(epoch uint64) bool {
	return epoch < v.exitEpoch
}
