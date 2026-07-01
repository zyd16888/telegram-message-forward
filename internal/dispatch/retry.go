package dispatch

import "time"

// backoff 返回第 attempt 次失败后的重试间隔（指数退避）。
// attempt 从 1 开始：1->2s, 2->4s, 3->8s ...，上限 5 分钟。
func backoff(attempt int) time.Duration {
	const base = 2 * time.Second
	const max = 5 * time.Minute

	d := base
	for i := 1; i < attempt; i++ {
		d *= 2
		if d >= max {
			return max
		}
	}
	if d > max {
		return max
	}
	return d
}
