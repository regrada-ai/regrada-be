package handlers

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/regrada-ai/regrada-be/internal/email"
)

type EmailHandler struct {
	emailService *email.Service
}

func NewEmailHandler(emailService *email.Service) *EmailHandler {
	return &EmailHandler{
		emailService: emailService,
	}
}

// SendEmail sends an email via AWS SES
func (h *EmailHandler) SendEmail(c *gin.Context) {
	var req struct {
		To      []string `json:"to" binding:"required"`
		Subject string   `json:"subject" binding:"required"`
		Body    string   `json:"body" binding:"required"`
		IsHTML  bool     `json:"is_html"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "INVALID_REQUEST",
				"message": err.Error(),
			},
		})
		return
	}

	msg := &email.EmailMessage{
		To:      req.To,
		Subject: req.Subject,
		Body:    req.Body,
		IsHTML:  req.IsHTML,
	}

	if err := h.emailService.SendEmail(c.Request.Context(), msg); err != nil {
		log.Printf("Failed to send email: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "EMAIL_SEND_FAILED",
				"message": "Failed to send email",
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Email sent successfully",
	})
}

// NewsletterSignup handles newsletter signups from the landing page
func (h *EmailHandler) NewsletterSignup(c *gin.Context) {
	var req struct {
		Email string `json:"email" binding:"required,email"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "INVALID_REQUEST",
				"message": "Invalid email address",
			},
		})
		return
	}

	if err := h.emailService.NewsletterSignup(c.Request.Context(), req.Email); err != nil {
		log.Printf("Failed to process newsletter signup: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "SIGNUP_FAILED",
				"message": "Failed to process signup. Please try again.",
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Successfully signed up!",
	})
}
