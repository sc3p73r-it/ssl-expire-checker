package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"ssl-expire-checker/internal/auth"
	"ssl-expire-checker/internal/config"
	"ssl-expire-checker/internal/ssl"
	"ssl-expire-checker/internal/store"
	"ssl-expire-checker/internal/worker"
)

// Domains wires HTTP handlers for domain CRUD and scans.
type Domains struct {
	Pool *pgxpool.Pool
	Cfg  *config.Config
}

type domainRow struct {
	ID          uuid.UUID  `json:"id"`
	URL         string     `json:"url"`
	Issuer      *string    `json:"issuer,omitempty"`
	Expiry      *time.Time `json:"expiry_date,omitempty"`
	Status      string     `json:"status"`
	LastChecked *time.Time `json:"last_checked,omitempty"`
	LastError   *string    `json:"last_error,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

type userScanJob struct {
	ID  uuid.UUID
	URL string
}

// List returns domains for the authenticated user.
func (h *Domains) List(c *gin.Context) {
	uid, ok := auth.UserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	ctx := c.Request.Context()
	rows, err := h.Pool.Query(ctx, `
		select id, url, issuer, expiry_date, status, last_checked, last_error, created_at
		from public.domains
		where user_id = $1
		order by created_at desc
	`, uid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db query failed"})
		return
	}
	defer rows.Close()

	var out []domainRow
	for rows.Next() {
		var r domainRow
		if err := rows.Scan(&r.ID, &r.URL, &r.Issuer, &r.Expiry, &r.Status, &r.LastChecked, &r.LastError, &r.CreatedAt); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db scan failed"})
			return
		}
		out = append(out, r)
	}
	c.JSON(http.StatusOK, gin.H{"domains": out})
}

type addBody struct {
	URL string `json:"url"`
}

// Add inserts a new domain for the authenticated user.
func (h *Domains) Add(c *gin.Context) {
	uid, ok := auth.UserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var body addBody
	if err := c.ShouldBindJSON(&body); err != nil || body.URL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json or empty url"})
		return
	}
	normalizedURL, err := ssl.NormalizeHost(body.URL)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := c.Request.Context()
	var id uuid.UUID
	err = h.Pool.QueryRow(ctx, `
		insert into public.domains (user_id, url, status)
		values ($1, $2, 'pending')
		on conflict (user_id, url) do nothing
		returning id
	`, uid, normalizedURL).Scan(&id)
	if err == pgx.ErrNoRows {
		err = h.Pool.QueryRow(ctx, `select id from public.domains where user_id = $1 and url = $2`, uid, normalizedURL).Scan(&id)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db insert failed"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": id})
}

// Delete removes a domain row owned by the user.
func (h *Domains) Delete(c *gin.Context) {
	uid, ok := auth.UserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	ctx := c.Request.Context()
	ct, err := h.Pool.Exec(ctx, `delete from public.domains where id = $1 and user_id = $2`, id, uid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db delete failed"})
		return
	}
	if ct.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.Status(http.StatusNoContent)
}

// ScanOne scans a single domain and persists the result.
func (h *Domains) ScanOne(c *gin.Context) {
	uid, ok := auth.UserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	ctx := c.Request.Context()
	var url string
	err = h.Pool.QueryRow(ctx, `select url from public.domains where id = $1 and user_id = $2`, id, uid).Scan(&url)
	if err == pgx.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db query failed"})
		return
	}

	res := ssl.Scan(url, h.Cfg.ExpiryThresholdDays)
	if err := store.SaveScanResult(ctx, h.Pool, id, res); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db update failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"result": res})
}

// ScanAll scans all domains for the authenticated user concurrently.
func (h *Domains) ScanAll(c *gin.Context) {
	uid, ok := auth.UserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	ctx := c.Request.Context()
	rows, err := h.Pool.Query(ctx, `select id, url from public.domains where user_id = $1`, uid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db query failed"})
		return
	}
	var jobs []userScanJob
	for rows.Next() {
		var j userScanJob
		if err := rows.Scan(&j.ID, &j.URL); err != nil {
			rows.Close()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db scan failed"})
			return
		}
		jobs = append(jobs, j)
	}
	rows.Close()

	wjobs := make([]worker.Job[userScanJob], len(jobs))
	for i := range jobs {
		wjobs[i] = worker.Job[userScanJob]{Payload: jobs[i]}
	}

	th := h.Cfg.ExpiryThresholdDays
	p := worker.NewPool[userScanJob](h.Cfg.WorkerCount)
	p.Run(wjobs, func(j userScanJob) {
		res := ssl.Scan(j.URL, th)
		_ = store.SaveScanResult(context.Background(), h.Pool, j.ID, res)
	})

	c.JSON(http.StatusOK, gin.H{"scanned": len(jobs)})
}
