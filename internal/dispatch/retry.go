package dispatch

import (
	"math/rand/v2"
	"time"
)

// backoff 返回第 attempt 次失败后的重试间隔（指数退避）。
// attempt 从 1 开始：1->2s, 2->4s, 3->8s ...，上限 5 分钟。
func backoff(attempt int) time.Duration {
	const base = 2 * time.Second
	const max = 5 * time.Minute

	d := base
	for i := 1; i < attempt; i++ {
		d *= 2
		if d >= max {
			return withJitter(max)
		}
	}
	if d > max {
		return withJitter(max)
	}
	return withJitter(d)
}

func withJitter(d time.Duration) time.Duration {
	if d <= 0 {
		return 0
	}
	// 0.8x - 1.2x，避免多个 worker 同时击穿下游。
	return time.Duration(float64(d) * (0.8 + rand.Float64()*0.4))
}
