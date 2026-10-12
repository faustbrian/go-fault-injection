package faultinject

import "fmt"

// Schedule is a deterministic schedule description. New accepts only the
// constructors in this package, so selection never calls user code under lock.
type Schedule any

type everySchedule struct{ every uint64 }
type nthSchedule struct{ nth uint64 }
type sequenceSchedule struct {
	pattern []bool
	repeat  bool
}
type probabilitySchedule struct {
	seed        uint64
	numerator   uint64
	denominator uint64
}

// Every describes every nth eligible call, beginning with call n. New rejects
// n == 0; the constructor itself performs no validation or scheduling work.
func Every(n uint64) Schedule { return everySchedule{every: n} }

// Nth describes only the nth eligible call. New rejects n == 0. Reset restarts
// the eligible-call counter; this does not mean the nth global evaluation.
func Nth(n uint64) Schedule { return nthSchedule{nth: n} }

// Sequence selects calls from a copied finite pattern. When repeat is true the
// pattern cycles; otherwise calls after the pattern do not match. Copying occurs
// eagerly in this constructor; New then requires a length of 1–1024 and copies
// it again. Keep construction inputs bounded before calling Sequence.
func Sequence(pattern []bool, repeat bool) Schedule {
	return sequenceSchedule{pattern: append([]bool(nil), pattern...), repeat: repeat}
}

// Probability selects a deterministic seeded fraction of eligible calls.
// Reproduction requires the seed and the same eligible-call ordering. New
// requires denominator > 0 and numerator <= denominator; zero/full fractions
// never/always match. The counter-derived modular test is not cryptographic
// randomness or a promise of an exact frequency in a finite campaign.
func Probability(seed, numerator, denominator uint64) Schedule {
	return probabilitySchedule{seed: seed, numerator: numerator, denominator: denominator}
}

func cloneSchedule(schedule Schedule) (Schedule, error) {
	switch value := schedule.(type) {
	case everySchedule:
		if value.every == 0 {
			return nil, invalid("rule.schedule", "Every requires a positive interval")
		}
		return value, nil
	case nthSchedule:
		if value.nth == 0 {
			return nil, invalid("rule.schedule", "Nth requires a positive call")
		}
		return value, nil
	case sequenceSchedule:
		if len(value.pattern) == 0 || len(value.pattern) > 1024 {
			return nil, invalid("rule.schedule", "Sequence length must be between 1 and 1024")
		}
		value.pattern = append([]bool(nil), value.pattern...)
		return value, nil
	case probabilitySchedule:
		if value.denominator == 0 || value.numerator > value.denominator {
			return nil, invalid("rule.schedule", "Probability must be a valid fraction")
		}
		return value, nil
	case nil:
		return nil, invalid("rule.schedule", "must be declared")
	default:
		return nil, invalid("rule.schedule", fmt.Sprintf("unsupported type %T", schedule))
	}
}

func scheduleMatches(schedule Schedule, call uint64) bool {
	switch value := schedule.(type) {
	case everySchedule:
		return call%value.every == 0
	case nthSchedule:
		return call == value.nth
	case sequenceSchedule:
		index := call - 1
		if value.repeat {
			index = index % uint64(len(value.pattern))
		} else if index >= uint64(len(value.pattern)) {
			return false
		}
		return value.pattern[index]
	case probabilitySchedule:
		if value.numerator == 0 {
			return false
		}
		if value.numerator == value.denominator {
			return true
		}
		return splitMix64(value.seed+call*0x9e3779b97f4a7c15)%value.denominator < value.numerator
	default:
		panic("faultinject: validated schedule has unknown type")
	}
}

func scheduleSeed(schedule Schedule) uint64 {
	if value, ok := schedule.(probabilitySchedule); ok {
		return value.seed
	}
	return 0
}

// Fixed unsigned wraparound and constants make the counter-to-word mapping
// stable without mutable PRNG state. Changing this mapping changes recorded
// seeded campaigns (TestSeededProbabilityAndSequenceAreReproducible).
func splitMix64(value uint64) uint64 {
	value += 0x9e3779b97f4a7c15
	value = (value ^ (value >> 30)) * 0xbf58476d1ce4e5b9
	value = (value ^ (value >> 27)) * 0x94d049bb133111eb
	return value ^ (value >> 31)
}
