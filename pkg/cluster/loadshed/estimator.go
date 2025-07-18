package loadshed

import (
	"sync"
	"time"
)

type Estimator struct {
	mu         sync.Mutex
	avg        time.Duration
	alpha      float64
	lastUpdate time.Time
}

func NewEstimator(alpha float64) *Estimator {
	return &Estimator{
		alpha: alpha,
	}
}

func (e *Estimator) Update(sample time.Duration) {
	if sample <= 0 {
		return
	}
	
	e.mu.Lock()
	defer e.mu.Unlock()

	now := time.Now()

	if e.avg == 0 {
		e.avg = sample
	} else {
		//Source: https://www.cmcmarkets.com/de-de/hilfe/glossar/e/exponentieller-gleitender-durchschnitt
		e.avg = time.Duration(float64(sample)*e.alpha + float64(e.avg)*(1-e.alpha))
	}

	e.lastUpdate = now
}

func (e *Estimator) Get() time.Duration {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.avg == 0 {
		return 200 * time.Millisecond
	}
	return e.avg
}
