package live

import (
	"conexiuni-cluj/database"
	"conexiuni-cluj/jobs"
	"strings"
	"sync"
	"time"
)

const (
	archiveRetention  = 400 * 24 * time.Hour
	archiveSettle     = time.Hour
	ArchiveStartDelay = 10 * time.Minute
	archiveHour       = 3
	archiveMinute     = 30
	ArchiveOverdue    = 26 * time.Hour
)

type ArchiveRun struct {
	At       time.Time
	Took     time.Duration
	Copied   int
	Versions int
	Pruned   int
	Err      string
	Summary  database.ArchiveSummary
}

func (eng *Engine) StartArchiver(r *jobs.Runner, loc *time.Location) {
	if eng.Archive == nil {
		return
	}
	r.Go(func() {
		if !r.Sleep(ArchiveStartDelay) {
			return
		}
		for {
			eng.checkObservedSchedule(time.Now())
			eng.runArchive(time.Now())
			eng.tuneVacationWindow(time.Now())
			eng.tuneSeedFill(time.Now())
			eng.restoreSeededWindows(time.Now())
			if !r.Sleep(time.Until(nextArchiveRun(time.Now(), loc))) {
				return
			}
		}
	})
}

func nextArchiveRun(now time.Time, loc *time.Location) time.Time {
	local := now.In(loc)
	next := time.Date(local.Year(), local.Month(), local.Day(), archiveHour, archiveMinute, 0, 0, loc)
	if !next.After(local) {
		next = time.Date(local.Year(), local.Month(), local.Day()+1, archiveHour, archiveMinute, 0, 0, loc)
	}
	return next
}

func (eng *Engine) runArchive(now time.Time) {
	started := time.Now()
	run := ArchiveRun{At: now}
	var errs []string
	copied, err := eng.CopyToArchive(now.Add(-archiveSettle))
	for _, n := range copied {
		run.Copied += n
	}
	if err != nil {
		errs = append(errs, "copy: "+err.Error())
	}
	if run.Versions, err = eng.ArchiveVersions(now); err != nil {
		errs = append(errs, "versions: "+err.Error())
	}
	if run.Pruned, err = eng.PruneArchive(now.Add(-archiveRetention)); err != nil {
		errs = append(errs, "prune: "+err.Error())
	}
	if run.Summary, err = eng.SummarizeArchive(); err != nil {
		errs = append(errs, "summary: "+err.Error())
	}
	run.Err = strings.Join(errs, "; ")
	run.Took = time.Since(started)
	if run.Err != "" {
		eng.log("archive failed: %s", run.Err)
	} else {
		eng.log("archive: copied %d rows, %d new timetable or stop versions, pruned %d in %s", run.Copied, run.Versions, run.Pruned, run.Took.Round(time.Millisecond))
	}
	eng.lastArchive.Lock()
	eng.lastArchive.run = &run
	eng.lastArchive.Unlock()
}

func (eng *Engine) lastArchiveRun() *ArchiveRun {
	eng.lastArchive.Lock()
	defer eng.lastArchive.Unlock()
	return eng.lastArchive.run
}

type lastArchiveState struct {
	sync.Mutex
	run *ArchiveRun
}
