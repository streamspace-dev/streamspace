package plugins

import (
	"fmt"
	"log"

	"github.com/robfig/cron/v3"
)

// PluginScheduler provides cron-based scheduling for plugins
type PluginScheduler struct {
	cron       *cron.Cron
	pluginName string
	jobIDs     map[string]cron.EntryID // jobName -> entryID
}

// NewPluginScheduler creates a new plugin scheduler
func NewPluginScheduler(cronInstance *cron.Cron, pluginName string) *PluginScheduler {
	return &PluginScheduler{
		cron:       cronInstance,
		pluginName: pluginName,
		jobIDs:     make(map[string]cron.EntryID),
	}
}

// Schedule schedules a job using cron syntax
// cronExpr examples:
//   - "*/5 * * * *" - every 5 minutes
//   - "0 * * * *"   - every hour
//   - "0 0 * * *"   - daily at midnight
//   - "@hourly"     - every hour
//   - "@daily"      - every day at midnight
func (ps *PluginScheduler) Schedule(jobName string, cronExpr string, job func()) error {
	// Remove existing job if any
	if existingID, exists := ps.jobIDs[jobName]; exists {
		ps.cron.Remove(existingID)
		delete(ps.jobIDs, jobName)
	}

	// Wrap job with plugin context and error handling
	wrappedJob := func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[Plugin:%s] Scheduled job %s panicked: %v", ps.pluginName, jobName, r)
			}
		}()

		log.Printf("[Plugin:%s] Running scheduled job: %s", ps.pluginName, jobName)
		job()
	}

	// Add job to cron
	entryID, err := ps.cron.AddFunc(cronExpr, wrappedJob)
	if err != nil {
		return fmt.Errorf("failed to schedule job %s for plugin %s: %w", jobName, ps.pluginName, err)
	}

	ps.jobIDs[jobName] = entryID
	log.Printf("[Plugin:%s] Scheduled job %s with expression: %s", ps.pluginName, jobName, cronExpr)

	return nil
}

// Remove removes a scheduled job
func (ps *PluginScheduler) Remove(jobName string) {
	if entryID, exists := ps.jobIDs[jobName]; exists {
		ps.cron.Remove(entryID)
		delete(ps.jobIDs, jobName)
		log.Printf("[Plugin:%s] Removed scheduled job: %s", ps.pluginName, jobName)
	}
}

// RemoveAll removes all scheduled jobs for this plugin
func (ps *PluginScheduler) RemoveAll() {
	for jobName, entryID := range ps.jobIDs {
		ps.cron.Remove(entryID)
		log.Printf("[Plugin:%s] Removed scheduled job: %s", ps.pluginName, jobName)
	}
	ps.jobIDs = make(map[string]cron.EntryID)
}

// ListJobs returns all scheduled job names for this plugin
func (ps *PluginScheduler) ListJobs() []string {
	jobs := make([]string, 0, len(ps.jobIDs))
	for jobName := range ps.jobIDs {
		jobs = append(jobs, jobName)
	}
	return jobs
}

// IsScheduled checks if a job is scheduled
func (ps *PluginScheduler) IsScheduled(jobName string) bool {
	_, exists := ps.jobIDs[jobName]
	return exists
}

// ScheduleInterval schedules a job to run at a fixed interval
// interval examples: "5m", "1h", "30s"
func (ps *PluginScheduler) ScheduleInterval(jobName string, interval string, job func()) error {
	// Convert interval to cron expression
	var cronExpr string

	switch interval {
	case "1m", "1 minute":
		cronExpr = "* * * * *"
	case "5m", "5 minutes":
		cronExpr = "*/5 * * * *"
	case "10m", "10 minutes":
		cronExpr = "*/10 * * * *"
	case "15m", "15 minutes":
		cronExpr = "*/15 * * * *"
	case "30m", "30 minutes":
		cronExpr = "*/30 * * * *"
	case "1h", "1 hour", "hourly":
		cronExpr = "@hourly"
	case "2h", "2 hours":
		cronExpr = "0 */2 * * *"
	case "4h", "4 hours":
		cronExpr = "0 */4 * * *"
	case "6h", "6 hours":
		cronExpr = "0 */6 * * *"
	case "12h", "12 hours":
		cronExpr = "0 */12 * * *"
	case "24h", "1 day", "daily":
		cronExpr = "@daily"
	case "weekly":
		cronExpr = "@weekly"
	case "monthly":
		cronExpr = "@monthly"
	default:
		return fmt.Errorf("unsupported interval: %s", interval)
	}

	return ps.Schedule(jobName, cronExpr, job)
}
