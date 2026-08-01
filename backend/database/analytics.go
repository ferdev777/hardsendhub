package database

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"hardsend/models"
)

// GetAnalyticsSummary retrieves aggregated conversion funnel and engagement metrics for a given period.
func (db *DB) GetAnalyticsSummary(period string) (*models.AnalyticsSummary, error) {
	from := getPeriodStartTime(period)

	summary := &models.AnalyticsSummary{Period: period}

	query := `
		SELECT
			COALESCE(COUNT(*), 0) AS total_sent,
			COALESCE(SUM(CASE WHEN delivered = 1 THEN 1 ELSE 0 END), 0) AS total_delivered,
			COALESCE(SUM(CASE WHEN opened = 1 THEN 1 ELSE 0 END), 0) AS total_opened,
			COALESCE(SUM(CASE WHEN bounced = 1 THEN 1 ELSE 0 END), 0) AS total_bounced,
			COALESCE(SUM(CASE WHEN complained = 1 THEN 1 ELSE 0 END), 0) AS total_complained
		FROM campaign_invoices
		WHERE status = 'SENT' AND ( ? = '' OR id IN (
			SELECT ci.id FROM campaign_invoices ci
			JOIN campaigns c ON ci.campaign_id = c.id
			WHERE c.created_at >= ?
		))`

	fromStr := ""
	if !from.IsZero() {
		fromStr = from.Format("2006-01-02 15:04:05")
	}

	err := db.conn.QueryRow(query, fromStr, fromStr).Scan(
		&summary.TotalSent,
		&summary.TotalDelivered,
		&summary.TotalOpened,
		&summary.TotalBounced,
		&summary.TotalComplained,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to scan analytics summary: %w", err)
	}

	if summary.TotalSent > 0 {
		summary.OpenRate = float64(summary.TotalOpened) / float64(summary.TotalSent) * 100
		summary.BounceRate = float64(summary.TotalBounced) / float64(summary.TotalSent) * 100
	}

	return summary, nil
}

// GetAnalyticsTimeSeries retrieves data points grouped by time for charts.
func (db *DB) GetAnalyticsTimeSeries(period string) ([]models.AnalyticsPoint, error) {
	from := getPeriodStartTime(period)

	var fmtStr string
	switch period {
	case "today", "day", "realtime":
		fmtStr = "%H:00" // Group by hour of today
	case "week", "month":
		fmtStr = "%Y-%m-%d" // Group by day
	case "year":
		fmtStr = "%Y-%m" // Group by month
	default:
		fmtStr = "%Y-%m-%d"
	}

	fromStr := ""
	if !from.IsZero() {
		fromStr = from.Format("2006-01-02 15:04:05")
	}

	query := fmt.Sprintf(`
		SELECT
			strftime('%s', c.created_at) AS label,
			COUNT(ci.id) AS sent,
			COALESCE(SUM(CASE WHEN ci.delivered = 1 THEN 1 ELSE 0 END), 0) AS delivered,
			COALESCE(SUM(CASE WHEN ci.opened = 1 THEN 1 ELSE 0 END), 0) AS opened,
			COALESCE(SUM(CASE WHEN ci.bounced = 1 THEN 1 ELSE 0 END), 0) AS bounced
		FROM campaign_invoices ci
		JOIN campaigns c ON ci.campaign_id = c.id
		WHERE ci.status = 'SENT' AND ( ? = '' OR c.created_at >= ? )
		GROUP BY label
		ORDER BY label ASC`, fmtStr)

	rows, err := db.conn.Query(query, fromStr, fromStr)
	if err != nil {
		return nil, fmt.Errorf("failed to query analytics time series: %w", err)
	}
	defer rows.Close()

	var points []models.AnalyticsPoint
	for rows.Next() {
		var p models.AnalyticsPoint
		if err := rows.Scan(&p.Label, &p.Sent, &p.Delivered, &p.Opened, &p.Bounced); err != nil {
			return nil, err
		}
		points = append(points, p)
	}

	return points, nil
}

func getPeriodStartTime(period string) time.Time {
	now := time.Now()
	switch period {
	case "realtime":
		return now.Add(-1 * time.Hour)
	case "today", "day":
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	case "week":
		return now.AddDate(0, 0, -7)
	case "month":
		return now.AddDate(0, -1, 0)
	case "year":
		return now.AddDate(-1, 0, 0)
	default:
		return time.Time{} // All time
	}
}

type resendEmailItem struct {
	ID        string   `json:"id"`
	To        []string `json:"to"`
	CreatedAt string   `json:"created_at"`
	LastEvent string   `json:"last_event"`
}

type resendListResponse struct {
	Object  string            `json:"object"`
	HasMore bool              `json:"has_more"`
	Data    []resendEmailItem `json:"data"`
}

// SyncResendWithAPI fetches email statuses from Resend API and updates local SQLite records.
func (db *DB) SyncResendWithAPI(apiKey string) (*models.ResendSyncResult, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("missing Resend API key")
	}

	result := &models.ResendSyncResult{
		StatusCounts: map[string]int{},
	}

	cursor := ""
	client := &http.Client{Timeout: 30 * time.Second}

	for page := 0; page < 30; page++ {
		url := "https://api.resend.com/emails?limit=100"
		if cursor != "" {
			url += "&after=" + cursor
		}

		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return result, fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+apiKey)
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			return result, fmt.Errorf("request to Resend API failed: %w", err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode != 200 {
			return result, fmt.Errorf("resend API status %d: %s", resp.StatusCode, string(body))
		}

		var listResp resendListResponse
		if err := json.Unmarshal(body, &listResp); err != nil {
			return result, fmt.Errorf("failed to decode json: %w", err)
		}

		for _, item := range listResp.Data {
			result.TotalFetched++
			result.StatusCounts[item.LastEvent]++

			eventTime, _ := time.Parse(time.RFC3339, item.CreatedAt)
			if eventTime.IsZero() {
				eventTime = time.Now()
			}

			// Update campaign_invoices by resend_id
			if item.ID != "" && item.LastEvent != "" {
				_ = db.UpdateCampaignInvoiceEngagement(item.ID, "", item.LastEvent, eventTime)
			}

			// Update campaign_invoices and invoices by recipient email
			if item.LastEvent == "bounced" || item.LastEvent == "delivered" || item.LastEvent == "opened" || item.LastEvent == "complained" {
				for _, recipient := range item.To {
					res1, _ := db.conn.Exec(fmt.Sprintf("UPDATE campaign_invoices SET %s = 1 WHERE email = ? AND %s = 0", item.LastEvent, item.LastEvent), recipient)
					res2, _ := db.conn.Exec(fmt.Sprintf("UPDATE invoices SET %s = 1 WHERE recipient_email = ? AND %s = 0", item.LastEvent, item.LastEvent), recipient)
					if res1 != nil {
						n, _ := res1.RowsAffected()
						result.UpdatedInvoices += int(n)
					}
					if res2 != nil {
						n, _ := res2.RowsAffected()
						result.UpdatedInvoices += int(n)
					}
				}
			}
		}

		if !listResp.HasMore || len(listResp.Data) == 0 {
			break
		}
		cursor = listResp.Data[len(listResp.Data)-1].ID
		time.Sleep(250 * time.Millisecond)
	}

	return result, nil
}
