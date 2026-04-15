package scheduler

import (
	"context"
	"encoding/json"
	"log"
	"time"
)

// Task represents a periodic job.
type Task struct {
	Name     string
	Interval time.Duration
	Fn       func(ctx context.Context) error
}

// Scheduler runs periodic tasks in the background.
type Scheduler struct {
	tasks  []Task
	cancel context.CancelFunc
}

func New() *Scheduler {
	return &Scheduler{}
}

func (s *Scheduler) Register(name string, interval time.Duration, fn func(ctx context.Context) error) {
	s.tasks = append(s.tasks, Task{Name: name, Interval: interval, Fn: fn})
}

// Start launches all registered tasks in background goroutines.
func (s *Scheduler) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	for _, task := range s.tasks {
		go s.run(ctx, task)
	}
	structLog("info", "scheduler_start", "", map[string]interface{}{"task_count": len(s.tasks)})
}

func (s *Scheduler) run(ctx context.Context, t Task) {
	ticker := time.NewTicker(t.Interval)
	defer ticker.Stop()
	// Run once immediately
	s.exec(ctx, t)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.exec(ctx, t)
		}
	}
}

func (s *Scheduler) exec(ctx context.Context, t Task) {
	defer func() {
		if r := recover(); r != nil {
			structLog("error", "scheduler_panic", t.Name, map[string]interface{}{
				"error_class": "panic",
				"error":       formatPanic(r),
			})
		}
	}()
	if err := t.Fn(ctx); err != nil {
		structLog("error", "scheduler_error", t.Name, map[string]interface{}{
			"error_class": "task_failure",
			"error":       err.Error(),
		})
	}
}

func (s *Scheduler) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
	structLog("info", "scheduler_stop", "", nil)
}

// structLog emits a structured JSON log line with common fields.
func structLog(level, action, task string, extra map[string]interface{}) {
	entry := map[string]interface{}{
		"level":     level,
		"component": "scheduler",
		"action":    action,
	}
	if task != "" {
		entry["task"] = task
	}
	for k, v := range extra {
		entry[k] = v
	}
	b, _ := json.Marshal(entry)
	log.Printf("%s", b)
}

func formatPanic(r interface{}) string {
	switch v := r.(type) {
	case error:
		return v.Error()
	case string:
		return v
	default:
		return "unknown panic"
	}
}
