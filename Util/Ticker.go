package Util

import (
	"fmt"
	"time"
)

type Ticker struct {
	WaitTime time.Duration
	LoopFunc func()
	ExitFunc func()
}

func NewTicker(waitTime time.Duration, loopFunc, exitFunc func()) *Ticker {
	ticker := Ticker{WaitTime: waitTime, LoopFunc: loopFunc, ExitFunc: exitFunc}
	return &ticker
}

func (t *Ticker) Start() {
	goTicker := time.NewTicker(t.WaitTime)
	go func() {
		for {
			select {
			case <-goTicker.C:
				t.LoopFunc()
			}
		}
	}()
}

func (t *Ticker) Close() {
	t.ExitFunc()
}

func TickerTestMain() {
	tick := NewTicker(time.Second*1, func() {
		fmt.Println("loop")
	}, func() {
		fmt.Println("exit")
	})
	tick.Start()
	CtrlCExitWithFunc(tick.Close)
}
