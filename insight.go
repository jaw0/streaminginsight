// Copyright (c) 2017
// Author: Jeff Weisberg <jaw @ tcp4me.com>
// Created: 2017-Aug-12 19:33 (EDT)
// Function: streaming moving mean + percentile metrics

// inspired by https://github.com/VividCortex/gohistogram
// but concurrency and different math
// see also: http://jmlr.org/papers/volume11/ben-haim10a/ben-haim10a.pdf

package streaminginsight

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"time"
)

type bucket struct {
	value float64
	count float64
}

type S struct {
	samp  chan int
	stop  chan struct{}
	bins  int
	beta  float64
	lock  sync.RWMutex
	hist  []bucket
	total bucket
}

// New returns a new object with the specified number of bins and decay time constant.
// typically bins is between 50-100, and the time constant from seconds to minutes.
// it is safe to use the returned object in multiple goroutines concurrently.
func New(bins int, t time.Duration) *S {

	var beta float64
	var tick time.Duration

	if t > 10*time.Second {
		tick = time.Second
	} else {
		tick = t / 10
	}
	beta = math.Pow(.5, float64(tick)/float64(t))

	s := &S{
		samp: make(chan int, 1000),
		stop: make(chan struct{}),
		hist: make([]bucket, 0, bins*5/4),
		bins: bins,
		beta: beta,
	}
	go s.work(tick)
	return s
}

// Close stops any and all goroutines maintaining the object behind the curtain.
func (s *S) Close() {
	close(s.stop)
}

// Add adds a new value.
func (s *S) Add(dt int) {
	// if the channel buffer is full, drop the value.
	// we're looking for insight, not exact values
	select {
	case s.samp <- dt:
		break
	default:
		break
	}
}

// Percentile returns the (approximation of) value for the specified percentile.
// pct should be in the range of (0..100)
func (s *S) Percentile(pct float64) float64 {

	s.lock.RLock()
	defer s.lock.RUnlock()

	if len(s.hist) == 0 {
		return 0
	}

	lim := s.total.count * pct / 100.0

	for _, b := range s.hist {
		lim -= b.count

		if lim <= 0 {
			return b.value
		}
	}

	return s.hist[len(s.hist)-1].value
}

// Mean returns the mean (average) value.
func (s *S) Mean() float64 {

	s.lock.RLock()
	defer s.lock.RUnlock()

	if s.total.count <= 0 {
		return 0
	}
	return s.total.value / s.total.count
}

func (s *S) String() string {

	out := ""

	s.lock.RLock()
	defer s.lock.RUnlock()

	if len(s.hist) == 0 {
		return ""
	}

	max := s.hist[0].count
	for _, b := range s.hist {
		if b.count > max {
			max = b.count
		}
	}

	scale := 80.0 / max

	for _, b := range s.hist {
		out += fmt.Sprintf("%8d ", int(b.value))
		len := int(b.count * scale)
		out += strings.Repeat("#", len)
		out += "\n"
	}

	return out
}

func (s *S) work(tick time.Duration) {

	tock := time.NewTicker(tick)

	for {
		select {
		case <-s.stop:
			tock.Stop()
			return
		case v := <-s.samp:
			s.add(v)
			s.maybereduce()
		case <-tock.C:
			s.decay()
			s.reduce()
		}
	}
}

func (s *S) decay() {

	s.lock.Lock()
	defer s.lock.Unlock()

	for i, _ := range s.hist {
		s.hist[i].count *= s.beta
	}

	s.total.count *= s.beta
	s.total.value *= s.beta
}

func (s *S) add(v int) {

	s.lock.Lock()
	defer s.lock.Unlock()

	s.total.count++
	s.total.value += float64(v)

	for i, _ := range s.hist {
		b := &s.hist[i]

		if v == int(b.value+.5) {
			// add to matching bucket
			b.count++
			return
		}

		if v < int(b.value+.5) {
			// insert new bucket
			nb := bucket{value: float64(v), count: 1}
			s.hist = append(s.hist, bucket{})
			copy(s.hist[i+1:], s.hist[i:])
			s.hist[i] = nb
			return
		}
	}

	// insert at end
	s.hist = append(s.hist, bucket{value: float64(v), count: 1})
}

func (s *S) maybereduce() {

	// temporarily allow some overage
	if len(s.hist) > s.bins+s.bins/5 {
		s.reduce()
	}
}

func (s *S) reduce() {

	s.lock.Lock()
	defer s.lock.Unlock()

	if len(s.hist) < s.bins {
		return
	}

	// remove empty
	epsilon := 0.5 / float64(s.bins)
	for i := 0; i < len(s.hist); i++ {
		b := &s.hist[i]

		if b.count < epsilon {
			copy(s.hist[i:], s.hist[i+1:])
			s.hist = s.hist[:len(s.hist)-1]
		}
	}

	// merge close
	for len(s.hist) > s.bins {
		i := s.closest()
		// s.hist = append(s.hist[:i], s.merged(i), s.hist[i+2:]...)
		s.hist[i] = s.merged(i)
		copy(s.hist[i+1:], s.hist[i+2:])
		s.hist = s.hist[:len(s.hist)-1]
	}

}

func (s *S) closest() int {

	ci := 0
	cv := s.hist[1].value - s.hist[0].value

	for i := 1; i < s.bins-1; i++ {
		d := s.hist[i+1].value - s.hist[i].value

		if d < cv {
			cv = d
			ci = i
		}
	}

	return ci
}

func (s *S) merged(i int) bucket {

	c := s.hist[i].count + s.hist[i+1].count
	a := s.hist[i].count*s.hist[i].value + s.hist[i+1].count*s.hist[i+1].value
	return bucket{count: c, value: a / c}
}
