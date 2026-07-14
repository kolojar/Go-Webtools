package helpertools

import (
	"time"
)

// Value from https://datatracker.ietf.org/doc/html/rfc6298#section-2 - section 2.3
const rttCalculatorNestedAverageAlpha = float64(0.125)

// Value from https://datatracker.ietf.org/doc/html/rfc6298#section-2 - section 2.3
const rttCalculatorNestedAverageBeta = float64(0.25)

// RTTCalculator is struct for calculating RTT
type RTTCalculator struct {
	rtt         time.Duration
	rttJitter   time.Duration //Also called rttVariance
	isFirst     bool
	fallbackRTO time.Duration
	minimumRTO  time.Duration
}

// ForceSetRTT forcefully sets internal RTT counter to duration
func (rtt *RTTCalculator) ForceSetRTT(duration time.Duration) {
	if rtt.isFirst {
		rtt.isFirst = false
	}
	rtt.rtt = duration
	rtt.rttJitter = rtt.rtt / 2
}

// CalculateRTT calculates average of rtt and of duration using nested average equation
func (rtt *RTTCalculator) CalculateRTT(duration time.Duration) {
	//Handle isFirst duration
	if rtt.isFirst {
		rtt.ForceSetRTT(duration)
		return
	}

	//Calculate diff for Jitter
	var diff float64
	if duration > rtt.rtt {
		diff = float64(duration - rtt.rtt)
	} else {
		diff = float64(rtt.rtt - duration)
	}

	//Calculate average RTO
	//rtt.rttJitter = time.Duration((1.0-rttCalculatorNestedAverageBeta)*float64(rtt.rttJitter) + rttCalculatorNestedAverageBeta*diff)
	rtt.rttJitter = NestedAverage(rtt.rttJitter, diff, rttCalculatorNestedAverageBeta)

	//Calculate average RTT
	//rtt.rtt = time.Duration((1.0-rttCalculatorNestedAverageAlpha)*float64(rtt.rtt) + rttCalculatorNestedAverageAlpha*float64(duration))
	rtt.rtt = NestedAverage(rtt.rtt, duration, rttCalculatorNestedAverageAlpha)
}

// GetRTT gets RTT value
func (rtt *RTTCalculator) GetRTT() time.Duration {
	if rtt.isFirst {
		return 0
	}
	return rtt.rtt
}

func (rtt *RTTCalculator) GetRTO() time.Duration {
	//Handle RTO before any RTT
	if rtt.isFirst {
		return rtt.fallbackRTO
	}

	//Calculate RTO
	rto := rtt.rtt + 4*rtt.rttJitter
	if rto < rtt.minimumRTO {
		return rtt.minimumRTO
	}
	return rto
}
