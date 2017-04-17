package util

import (
	"time"
)

var WallClock = &FnClock{
	NowFn: func() time.Time { return time.Now().UTC() },
}

type Clock interface {
	Now() time.Time
}

type FnClock struct {
	NowFn func() time.Time
}

var _ Clock = &FnClock{}

func (f *FnClock) Now() time.Time {
	return f.NowFn()
}
