package helpertools

import (
	"sync"
	"time"
)

// Value from https://datatracker.ietf.org/doc/html/rfc6298#section-2 - section 2.3
const RTTCalculatorNestedAverageAlpha = float64(0.125)

// Value from https://datatracker.ietf.org/doc/html/rfc6298#section-2 - section 2.3
const RTTCalculatorNestedAverageBeta = float64(0.25)

// RTTCalculator is struct for calculating RTT
type RTTCalculator struct {
	mutex       sync.Mutex
	rtt         time.Duration
	rttJitter   time.Duration //Also called rttVariance
	isFirst     bool
	fallbackRTT time.Duration
	minimumRTT  time.Duration
	maximumRTT  time.Duration
}

// NewRTTCalculator creates new RTT calculator
func NewRTTCalculator(minimumRTT time.Duration, maximumRTT time.Duration, fallbackRTT time.Duration) *RTTCalculator {
	if minimumRTT > maximumRTT {
		panic("Minimum RTT must be smaller then maximum RTT.")
	}
	return &RTTCalculator{
		rtt:         time.Duration(0),
		rttJitter:   time.Duration(0),
		isFirst:     true,
		fallbackRTT: fallbackRTT,
		minimumRTT:  minimumRTT,
		maximumRTT:  maximumRTT,
		mutex:       sync.Mutex{},
	}
}

// ForceSetRTT forcefully sets internal RTT counter to duration
func (rtt *RTTCalculator) ForceSetRTT(duration time.Duration) {
	//Lock mutex
	rtt.mutex.Lock()
	defer rtt.mutex.Unlock()

	//Set value
	if rtt.isFirst {
		rtt.isFirst = false
	}
	rtt.rtt = duration
	rtt.rttJitter = rtt.rtt / 2
}

// CalculateRTT calculates average of rtt and of duration using nested average equation
func (rtt *RTTCalculator) CalculateRTT(duration time.Duration) {
	//Lock mutex
	rtt.mutex.Lock()
	defer rtt.mutex.Unlock()

	//Handle isFirst duration
	if rtt.isFirst {
		if rtt.isFirst {
			rtt.isFirst = false
		}
		rtt.rtt = duration
		rtt.rttJitter = rtt.rtt / 2
		return
	}

	//Calculate diff for Jitter
	var diff float64
	if duration > rtt.rtt {
		diff = float64(duration - rtt.rtt)
	} else {
		diff = float64(rtt.rtt - duration)
	}

	//Calculate average jitter
	//rtt.rttJitter = time.Duration((1.0-rttCalculatorNestedAverageBeta)*float64(rtt.rttJitter) + rttCalculatorNestedAverageBeta*diff)
	rtt.rttJitter = NestedAverage(rtt.rttJitter, diff, RTTCalculatorNestedAverageBeta)

	//Calculate average RTT
	//rtt.rtt = time.Duration((1.0-rttCalculatorNestedAverageAlpha)*float64(rtt.rtt) + rttCalculatorNestedAverageAlpha*float64(duration))
	rtt.rtt = NestedAverage(rtt.rtt, duration, RTTCalculatorNestedAverageAlpha)
}

// GetRTT gets RTT value
func (rtt *RTTCalculator) GetRTT() time.Duration {
	result, _ := rtt.GetRTTAndJitter()
	return result
}

// GetRTTAndJitter gets RTT value and Jitter
func (rtt *RTTCalculator) GetRTTAndJitter() (rttValue time.Duration, rttJitterValue time.Duration) {
	//Lock mutex
	rtt.mutex.Lock()
	defer rtt.mutex.Unlock()

	//Read value
	if rtt.isFirst {
		return rtt.fallbackRTT, rtt.fallbackRTT / 2
	}
	if rtt.rtt < rtt.minimumRTT {
		return rtt.minimumRTT, rtt.minimumRTT / 2
	}
	if rtt.rtt > rtt.maximumRTT {
		return rtt.maximumRTT, rtt.maximumRTT / 2
	}
	return rtt.rtt, rtt.rttJitter
}

// GetRTO calculates RTO
func (rtt *RTTCalculator) GetRTO() time.Duration {
	//Get value
	rttValue, jitter := rtt.GetRTTAndJitter()

	//Calculate RTO
	return rttValue + 4*jitter
}
