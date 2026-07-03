package dispatch

// Notifier 用于在新投递任务入队后唤醒 worker。
//
// 信号是非阻塞、可合并的：worker 已经被唤醒但尚未消费时，后续通知不再堆积。
type Notifier struct {
	ch chan struct{}
}

// NewNotifier 创建投递任务唤醒器。
func NewNotifier() *Notifier {
	return &Notifier{ch: make(chan struct{}, 1)}
}

// Notify 非阻塞发送一次唤醒信号。
func (n *Notifier) Notify() {
	if n == nil {
		return
	}
	select {
	case n.ch <- struct{}{}:
	default:
	}
}

// C 返回只读唤醒信号通道。
func (n *Notifier) C() <-chan struct{} {
	if n == nil {
		return nil
	}
	return n.ch
}
