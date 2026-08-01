package helpertools

import (
	"sync"
	"time"
)

// Value from https://datatracker.ietf.org/doc/html/rfc6298#section-2 - section 2.3
const rttCalculatorNestedAverageAlpha = float64(0.125)

// Value from https://datatracker.ietf.org/doc/html/rfc6298#section-2 - section 2.3
const rttCalculatorNestedAverageBeta = float64(0.25)

// RTTCalculator is struct for calculating RTT
type RTTCalculator struct {
	mutex       *sync.RWMutex
	rtt         time.Duration
	rttJitter   time.Duration //Also called rttVariance
	isFirst     bool
	fallbackRTO time.Duration
	minimumRTO  time.Duration
}

// NewRTTCalculator creates new RTT calculator
func NewRTTCalculator(minimumRTO time.Duration, fallbackRTO time.Duration) *RTTCalculator {
	return &RTTCalculator{
		rtt:         time.Duration(0),
		rttJitter:   time.Duration(0),
		isFirst:     true,
		fallbackRTO: fallbackRTO,
		minimumRTO:  minimumRTO,
		mutex:       &sync.RWMutex{},
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

	//Calculate average jitter
	//rtt.rttJitter = time.Duration((1.0-rttCalculatorNestedAverageBeta)*float64(rtt.rttJitter) + rttCalculatorNestedAverageBeta*diff)
	rtt.rttJitter = NestedAverage(rtt.rttJitter, diff, rttCalculatorNestedAverageBeta)

	//Calculate average RTT
	//rtt.rtt = time.Duration((1.0-rttCalculatorNestedAverageAlpha)*float64(rtt.rtt) + rttCalculatorNestedAverageAlpha*float64(duration))
	rtt.rtt = NestedAverage(rtt.rtt, duration, rttCalculatorNestedAverageAlpha)
}

// GetRTT gets RTT value
func (rtt *RTTCalculator) GetRTT() time.Duration {
	//Lock mutex
	rtt.mutex.RLock()
	defer rtt.mutex.RUnlock()

	//Read value
	if rtt.isFirst {
		return 0
	}
	return rtt.rtt
}

func (rtt *RTTCalculator) GetRTO() time.Duration {
	//Lock mutex
	rtt.mutex.RLock()
	defer rtt.mutex.RUnlock()

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
