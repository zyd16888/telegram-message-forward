// Package clock 抽象时间获取，便于测试注入。
package clock

import "time"

// Clock 抽象当前时间。
type Clock interface {
	Now() time.Time
}

// System 使用系统时间。
type System struct{}

// Now 返回当前系统时间。
func (System) Now() time.Time { return time.Now() }
