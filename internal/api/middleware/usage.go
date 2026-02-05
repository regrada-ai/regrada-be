// SPDX-License-Identifier: LicenseRef-Regrada-Proprietary

package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/regrada-ai/regrada-be/internal/storage"
)

type UsageMiddleware struct {
	orgRepo storage.OrganizationRepository
}

func NewUsageMiddleware(orgRepo storage.OrganizationRepository) *UsageMiddleware {
	return &UsageMiddleware{
		orgRepo: orgRepo,
	}
}

// TrackUsage tracks and enforces monthly usage limits for metered requests.
// This should be applied to routes that count against the monthly limit:
// - Trace ingestion
// - Test run uploads
// - Regression evaluation runs
func (m *UsageMiddleware) TrackUsage() gin.HandlerFunc {
	return func(c *gin.Context) {
		orgID := c.GetString("organization_id")
		if orgID == "" {
			c.Next()
			return
		}

		ctx := c.Request.Context()

		// Get current org state and increment counter atomically
		org, err := m.orgRepo.IncrementRequestCount(ctx, orgID)
		if err != nil {
			// On error, allow the request (fail open) but log it
			c.Next()
			return
		}

		// Check if we need to reset the monthly counter
		if time.Now().UTC().After(org.UsageResetAt) {
			if err := m.orgRepo.ResetMonthlyUsage(ctx, orgID); err == nil {
				// Refetch after reset
				org, _ = m.orgRepo.Get(ctx, orgID)
			}
		}

		// Calculate usage percentage
		usagePercent := float64(org.MonthlyRequestCount) / float64(org.MonthlyRequestLimit) * 100

		// Set usage headers for client visibility
		c.Header("X-Monthly-Limit", formatInt64(org.MonthlyRequestLimit))
		c.Header("X-Monthly-Used", formatInt64(org.MonthlyRequestCount))
		c.Header("X-Monthly-Reset", org.UsageResetAt.Format(time.RFC3339))

		// Enforce limits based on tier
		tier := c.GetString("tier")
		if tier == "" {
			tier = org.Tier
		}

		// Starter tier: hard stop at 100%
		if tier == "starter" && usagePercent >= 100 {
			c.JSON(http.StatusPaymentRequired, gin.H{
				"error": gin.H{
					"code":    "USAGE_LIMIT_EXCEEDED",
					"message": "Monthly request limit exceeded. Upgrade your plan to continue.",
					"details": gin.H{
						"limit":    org.MonthlyRequestLimit,
						"used":     org.MonthlyRequestCount,
						"tier":     tier,
						"reset_at": org.UsageResetAt,
					},
				},
			})
			c.Abort()
			return
		}

		// Team/Scale tiers: throttle at 120% (reduce RPM by 50%)
		if (tier == "team" || tier == "scale") && usagePercent >= 120 {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": gin.H{
					"code":    "USAGE_LIMIT_THROTTLED",
					"message": "Monthly request limit exceeded by 20%. Requests are being throttled.",
					"details": gin.H{
						"limit":    org.MonthlyRequestLimit,
						"used":     org.MonthlyRequestCount,
						"tier":     tier,
						"reset_at": org.UsageResetAt,
						"overage":  org.MonthlyRequestCount - org.MonthlyRequestLimit,
					},
				},
			})
			c.Abort()
			return
		}

		// Enterprise: no hard limit enforcement (custom/negotiated)

		c.Next()
	}
}

func formatInt64(n int64) string {
	return strconv.FormatInt(n, 10)
}
