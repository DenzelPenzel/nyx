package interval

import "time"

type Interval struct {
	shutdown chan bool
}

func SetInterval(fn func(t time.Time), delay time.Duration) Interval {
	c := Interval{shutdown: make(chan bool)}
	ticker := time.NewTicker(delay)

	go func() {
		for {
			select {
			case <-c.shutdown:
				c.shutdown <- true
				return
			case t := <-ticker.C:
				fn(t)
			}
		}
	}()

	return c
}

func (c Interval) Clear() {
	defer func() {
		recover()
	}()
	c.shutdown <- true
	<-c.shutdown
	close(c.shutdown)
}
