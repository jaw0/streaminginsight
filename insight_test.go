// Copyright (c) 2017
// Author: Jeff Weisberg <jaw @ tcp4me.com>
// Created: 2017-Aug-13 00:43 (EDT)
// Function:

// 100 bins, big alpha, 100k * random[0..1k) - look

package streaminginsight

import (
	//"fmt"
	"math/rand"
	"testing"
	"time"
)

func TestSS1(t *testing.T) {

	s := New(100, time.Second)
	s.Close()

	for i := 0; i < 10000; i++ {
		r := rand.Intn(100)
		s.add(r)
	}
	s.reduce()

	m := s.Mean()
	nf := s.Percentile(95)

	if m < 48 || m > 52 {
		t.Fail()
	}
	if nf < 93 || nf > 97 {
		t.Fail()
	}
}

func TestSS2(t *testing.T) {

	s := New(100, time.Second)
	s.Close()

	for i := 0; i < 1000; i++ {
		r := rand.Intn(50) + rand.Intn(50)
		s.add(r)
	}
	for i := 0; i < 1000; i++ {
		s.decay()
	}
	for i := 0; i < 1000; i++ {
		r := rand.Intn(5) + rand.Intn(5) + 90
		s.add(r)
	}

	s.reduce()

	m := s.Mean()
	nf := s.Percentile(95)

	if m < 92 || m > 96 {
		t.Fail()
	}
	if nf < 95 || nf > 99 {
		t.Fail()
	}

	//fmt.Printf("m %f 95 %f 99 %f\n", s.Mean(), s.Percentile(95), s.Percentile(99))

}
