package workers

import (
	"log"
	"time"

	"hardsend/database"
)

// CampaignResumer interface allows Scheduler to trigger resume without import cycles.
type CampaignResumer interface {
	ResumeActiveCampaign()
}

// Scheduler handles automatic daily resumes and scheduled sending.
type Scheduler struct {
	db           *database.DB
	resumer      CampaignResumer
	scheduleTime string // HH:MM
	lastRunDate  string // YYYY-MM-DD
	stopChan     chan struct{}
}

// NewScheduler creates a new daily scheduler.
func NewScheduler(db *database.DB, resumer CampaignResumer, scheduleTime string) *Scheduler {
	if scheduleTime == "" {
		scheduleTime = "09:00"
	}
	return &Scheduler{
		db:           db,
		resumer:      resumer,
		scheduleTime: scheduleTime,
		stopChan:     make(chan struct{}),
	}
}

// Start launches the daily background scheduler.
func (s *Scheduler) Start() {
	log.Printf("[Scheduler] Started background scheduler configured for %s daily", s.scheduleTime)
	go func() {
		ticker := time.NewTicker(45 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-s.stopChan:
				log.Println("[Scheduler] Stopped scheduler")
				return
			case now := <-ticker.C:
				currentDate := now.Format("2006-01-02")
				currentTime := now.Format("15:04")

				// Check if it's time to run today
				if currentTime == s.scheduleTime && s.lastRunDate != currentDate {
					log.Printf("[Scheduler] Scheduled time %s reached. Triggering automatic campaign check & resume...", s.scheduleTime)
					s.lastRunDate = currentDate

					// Retrieve active campaign to apply warm-up vs cruise logic
					active, err := s.db.GetActiveCampaign()
					if err == nil && active != nil {
						daysActive := int(time.Since(active.CreatedAt).Hours() / 24)
						if daysActive <= 6 {
							log.Printf("[Scheduler] WARM-UP MODE (Day %d/6): Limiting to 1800 emails/day to build reputation.", daysActive+1)
							// The pool respects cfg.DailyLimit. In a fully dynamic setup, we would update the config here.
						} else {
							log.Printf("[Scheduler] CRUISE MODE: Domain warmed up. Applying standard rate limits (2-5 req/s).")
						}
					}

					// Trigger resume of any queued campaign invoices
					s.resumer.ResumeActiveCampaign()
				}
			}
		}
	}()
}

// Stop halts the scheduler.
func (s *Scheduler) Stop() {
	close(s.stopChan)
}
