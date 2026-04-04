package pool

type GoroutinuePool struct {
	capacity int
	task     chan func()
	// the lambda-func may need to collect result itself
}

func NewGoroutinuePool(capacity int) *GoroutinuePool {
	return &GoroutinuePool{capacity: capacity, task: make(chan func(), capacity)}
}

func (p *GoroutinuePool) worker() {
	for true {
		t := <-p.task
		t()
	}
}

func (p *GoroutinuePool) Start() {
	for i := 0; i < p.capacity; i++ {
		go p.worker()
	}
}

func (p *GoroutinuePool) PutTask(f func()) {
	p.task <- f
}
