package scheduler

import (
	"log"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

// TriggerFunc is called when a schedule fires.
// sessionID identifies the assistant conversation; message is forwarded to it.
type TriggerFunc func(sessionID, message string)

// Scheduler manages both one-shot and periodic scheduled tasks.
type Scheduler struct {
	trigger TriggerFunc

	mu     sync.Mutex
	cron   *cron.Cron    // drives TypePeriodic schedules
	timers []*time.Timer // drives TypeOnce schedules
	done   chan struct{}
}

// New creates a Scheduler that calls trigger whenever a schedule fires.
func New(trigger TriggerFunc) *Scheduler {
	return &Scheduler{
		trigger: trigger,
		done:    make(chan struct{}),
	}
}

// Start loads all enabled schedules from disk and begins dispatching them.
// It is safe to call Start only once; use Reload to refresh schedules at runtime.
func (s *Scheduler) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cron = cron.New()
	if err := s.loadLocked(); err != nil {
		return err
	}
	s.cron.Start()
	log.Printf("[scheduler] started")
	return nil
}

// Stop cancels all pending timers and shuts down the cron runner.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Cancel outstanding one-shot timers.
	for _, t := range s.timers {
		t.Stop()
	}
	s.timers = nil

	// Stop the cron runner (waits for any running job to complete).
	if s.cron != nil {
		ctx := s.cron.Stop()
		<-ctx.Done()
		s.cron = nil
	}

	// Signal the done channel once.
	select {
	case <-s.done:
	default:
		close(s.done)
	}

	log.Printf("[scheduler] stopped")
}

// Reload re-reads schedules from disk and re-registers all tasks.
// Call this after adding or removing a schedule at runtime.
func (s *Scheduler) Reload() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Cancel existing one-shot timers.
	for _, t := range s.timers {
		t.Stop()
	}
	s.timers = nil

	// Replace the cron runner with a fresh instance.
	if s.cron != nil {
		ctx := s.cron.Stop()
		<-ctx.Done()
	}
	s.cron = cron.New()

	if err := s.loadLocked(); err != nil {
		return err
	}
	s.cron.Start()
	log.Printf("[scheduler] reloaded")
	return nil
}

// loadLocked registers all enabled schedules. Must be called with s.mu held.
func (s *Scheduler) loadLocked() error {
	schedules, err := LoadSchedules()
	if err != nil {
		return err
	}

	now := time.Now()
	for _, sc := range schedules {
		if !sc.Enabled {
			continue
		}
		switch sc.Type {
		case TypeOnce:
			s.registerOnce(sc, now)
		case TypePeriodic:
			s.registerPeriodic(sc)
		default:
			log.Printf("[scheduler] unknown schedule type %q for id=%s, skipping", sc.Type, sc.ID)
		}
	}
	return nil
}

// registerOnce schedules a one-shot timer for the given schedule.
// Must be called with s.mu held.
func (s *Scheduler) registerOnce(sc *Schedule, now time.Time) {
	if sc.At == nil {
		log.Printf("[scheduler] once schedule id=%s has no 'at' time, skipping", sc.ID)
		return
	}
	delay := sc.At.Sub(now)
	if delay <= 0 {
		// Already in the past — fire immediately then clean up.
		log.Printf("[scheduler] once schedule id=%s is in the past, firing immediately", sc.ID)
		go s.fireOnce(sc)
		return
	}

	// Capture loop variables for the closure.
	id := sc.ID
	sessionID := sc.SessionID
	message := sc.Message

	t := time.AfterFunc(delay, func() {
		log.Printf("[scheduler] once schedule id=%s fired", id)
		s.trigger(sessionID, message)
		// Remove the schedule after firing — it's a one-shot.
		if err := RemoveSchedule(id); err != nil {
			log.Printf("[scheduler] failed to remove once schedule id=%s: %v", id, err)
		}
	})
	s.timers = append(s.timers, t)
	log.Printf("[scheduler] registered once schedule id=%s fires in %s", id, delay.Round(time.Second))
}

// fireOnce triggers a past-due one-shot schedule and removes it from disk.
func (s *Scheduler) fireOnce(sc *Schedule) {
	s.trigger(sc.SessionID, sc.Message)
	if err := RemoveSchedule(sc.ID); err != nil {
		log.Printf("[scheduler] failed to remove once schedule id=%s: %v", sc.ID, err)
	}
}

// registerPeriodic registers a cron-driven schedule.
// Must be called with s.mu held.
func (s *Scheduler) registerPeriodic(sc *Schedule) {
	if sc.Cron == "" {
		log.Printf("[scheduler] periodic schedule id=%s has no cron expression, skipping", sc.ID)
		return
	}

	id := sc.ID
	sessionID := sc.SessionID
	message := sc.Message

	_, err := s.cron.AddFunc(sc.Cron, func() {
		log.Printf("[scheduler] periodic schedule id=%s fired", id)
		s.trigger(sessionID, message)
	})
	if err != nil {
		log.Printf("[scheduler] failed to register cron for schedule id=%s expr=%q: %v", id, sc.Cron, err)
		return
	}
	log.Printf("[scheduler] registered periodic schedule id=%s cron=%q", id, sc.Cron)
}
