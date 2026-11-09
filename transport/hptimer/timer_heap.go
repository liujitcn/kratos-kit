package hptimer

// timerHeap 最小堆，用于管理定时任务（触发时间越早，优先级越高）
type timerHeap []*TimerTask

func (h *timerHeap) Len() int           { return len(*h) }
func (h *timerHeap) Less(i, j int) bool { return (*h)[i].At.Before((*h)[j].At) }
func (h *timerHeap) Swap(i, j int)      { (*h)[i], (*h)[j] = (*h)[j], (*h)[i] }

// Push 将定时任务加入堆。
func (h *timerHeap) Push(x any) {
	*h = append(*h, x.(*TimerTask))
}

// Pop 弹出堆尾的定时任务，供容器堆维护最早任务顺序。
func (h *timerHeap) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}
