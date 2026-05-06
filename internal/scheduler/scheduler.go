package scheduler

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"ssl-expire-checker/internal/config"
	"ssl-expire-checker/internal/notify"
	"ssl-expire-checker/internal/ssl"
	"ssl-expire-checker/internal/store"
	"ssl-expire-checker/internal/worker"
)

// DomainJob is a row scanned by the global scheduler.
type DomainJob struct {
	ID     uuid.UUID
	UserID uuid.UUID
	URL    string
}

// RunGlobalScan scans every domain in the database (service DB role bypasses RLS).
func RunGlobalScan(ctx context.Context, pool *pgxpool.Pool, cfg *config.Config) ([]notify.ExpiringDomain, error) {
	rows, err := pool.Query(ctx, `select id, user_id, url from public.domains`)
	if err != nil {
		return nil, err
	}
	var jobs []DomainJob
	for rows.Next() {
		var j DomainJob
		if err := rows.Scan(&j.ID, &j.UserID, &j.URL); err != nil {
			rows.Close()
			return nil, err
		}
		jobs = append(jobs, j)
	}
	rows.Close()

	muJobs := make([]worker.Job[DomainJob], len(jobs))
	for i := range jobs {
		muJobs[i] = worker.Job[DomainJob]{Payload: jobs[i]}
	}

	var alertMu sync.Mutex
	var alert []notify.ExpiringDomain
	th := cfg.ExpiryThresholdDays
	wp := worker.NewPool[DomainJob](cfg.WorkerCount)
	wp.Run(muJobs, func(j DomainJob) {
		res := ssl.Scan(j.URL, th)
		_ = store.SaveScanResult(context.Background(), pool, j.ID, res)
		if res.Status == "expiring" || res.Status == "expired" {
			expStr := ""
			if res.Expiry != nil {
				expStr = res.Expiry.Format(time.RFC3339)
			}
			alertMu.Lock()
			alert = append(alert, notify.ExpiringDomain{URL: j.URL, Status: res.Status, Expiry: expStr})
			alertMu.Unlock()
		}
	})

	return alert, nil
}

// Loop runs an immediate scan, then repeats every ScanIntervalHours.
func Loop(ctx context.Context, pool *pgxpool.Pool, cfg *config.Config) {
	ticker := time.NewTicker(cfg.ScanInterval())
	defer ticker.Stop()

	run := func() {
		items, err := RunGlobalScan(ctx, pool, cfg)
		if err != nil {
			log.Printf("scheduler: global scan failed: %v", err)
			return
		}
		if err := notify.SendDigest(ctx, cfg.WebhookURL, items); err != nil {
			log.Printf("scheduler: webhook failed: %v", err)
		}
	}

	run()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}
