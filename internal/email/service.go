package email

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/aws/aws-sdk-go-v2/service/ses/types"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	sesv2types "github.com/aws/aws-sdk-go-v2/service/sesv2/types"
	"github.com/aws/aws-sdk-go-v2/service/sns"
)

type Service struct {
	client      *ses.Client
	sesv2Client *sesv2.Client
	snsClient   *sns.Client
	fromEmail   string
	fromName    string
	adminEmail  string
	snsTopicArn string
	contactList string
	frontendURL string
}

type EmailMessage struct {
	To      []string
	Subject string
	Body    string
	IsHTML  bool
}

func NewService(region, fromEmail, fromName, adminEmail, snsTopicArn, contactList, frontendURL string) (*Service, error) {
	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(region),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := ses.NewFromConfig(cfg)
	sesv2Client := sesv2.NewFromConfig(cfg)
	snsClient := sns.NewFromConfig(cfg)

	return &Service{
		client:      client,
		sesv2Client: sesv2Client,
		snsClient:   snsClient,
		fromEmail:   fromEmail,
		fromName:    fromName,
		adminEmail:  adminEmail,
		snsTopicArn: snsTopicArn,
		contactList: contactList,
		frontendURL: frontendURL,
	}, nil
}

func (s *Service) SendEmail(ctx context.Context, msg *EmailMessage) error {
	destination := &types.Destination{
		ToAddresses: msg.To,
	}

	var body types.Body
	if msg.IsHTML {
		body = types.Body{
			Html: &types.Content{
				Data:    aws.String(msg.Body),
				Charset: aws.String("UTF-8"),
			},
		}
	} else {
		body = types.Body{
			Text: &types.Content{
				Data:    aws.String(msg.Body),
				Charset: aws.String("UTF-8"),
			},
		}
	}

	message := &types.Message{
		Subject: &types.Content{
			Data:    aws.String(msg.Subject),
			Charset: aws.String("UTF-8"),
		},
		Body: &body,
	}

	var source string
	if s.fromName != "" {
		source = fmt.Sprintf("%s <%s>", s.fromName, s.fromEmail)
	} else {
		source = s.fromEmail
	}

	input := &ses.SendEmailInput{
		Source:      aws.String(source),
		Destination: destination,
		Message:     message,
	}

	_, err := s.client.SendEmail(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}

// NewsletterSignup handles newsletter signup including:
// - Adding contact to SES contact list
// - Publishing to SNS topic
// - Sending welcome email
// - Notifying admin
func (s *Service) NewsletterSignup(ctx context.Context, email string) error {
	// Add contact to SES contact list (if configured)
	if s.contactList != "" {
		err := s.addContactToList(ctx, email)
		if err != nil {
			// Log but don't fail - contact might already exist
			fmt.Printf("Contact list error (non-blocking): %v\n", err)
		}
	}

	// Publish to SNS topic (if configured)
	if s.snsTopicArn != "" {
		err := s.publishToSNS(ctx, email)
		if err != nil {
			fmt.Printf("SNS publish error (non-blocking): %v\n", err)
		}
	}

	// Send welcome email
	err := s.sendWelcomeEmail(ctx, email)
	if err != nil {
		fmt.Printf("Welcome email error (non-blocking): %v\n", err)
	}

	// Notify admin (if configured)
	if s.adminEmail != "" {
		err := s.notifyAdmin(ctx, email)
		if err != nil {
			fmt.Printf("Admin notification error (non-blocking): %v\n", err)
		}
	}

	return nil
}

func (s *Service) addContactToList(ctx context.Context, email string) error {
	// Small delay to ensure SESv2 is ready
	time.Sleep(100 * time.Millisecond)

	input := &sesv2.CreateContactInput{
		ContactListName: aws.String(s.contactList),
		EmailAddress:    aws.String(email),
		TopicPreferences: []sesv2types.TopicPreference{
			{
				TopicName:          aws.String("newsletter"),
				SubscriptionStatus: sesv2types.SubscriptionStatusOptIn,
			},
		},
	}

	_, err := s.sesv2Client.CreateContact(ctx, input)
	return err
}

func (s *Service) publishToSNS(ctx context.Context, email string) error {
	message := fmt.Sprintf(`{"email":"%s","timestamp":"%s","source":"landing_page"}`,
		email, time.Now().Format(time.RFC3339))

	input := &sns.PublishInput{
		TopicArn: aws.String(s.snsTopicArn),
		Subject:  aws.String("New Regrada Newsletter Signup"),
		Message:  aws.String(message),
	}

	_, err := s.snsClient.Publish(ctx, input)
	return err
}

func (s *Service) sendWelcomeEmail(ctx context.Context, email string) error {
	htmlBody := `
<html>
  <body style="font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', 'Courier New', monospace; background-color: #1D1F21; color: #C5C8C6; padding: 20px; margin: 0;">
    <h2 style="color: #81A2BE; font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', 'Courier New', monospace;">Welcome to Regrada!</h2>
    <p style="font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', 'Courier New', monospace;">Thank you for signing up for Regrada updates!</p>
    <p style="font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', 'Courier New', monospace;">We'll keep you posted on our launch and latest developments.</p>
    <p style="color: #969896; margin-top: 30px; font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', 'Courier New', monospace;">
      <em>CI for AI behavior — catch regressions before they ship.</em>
    </p>
    <p style="color: #969896; margin-top: 30px; font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', 'Courier New', monospace;">- The Regrada Team</p>
    <hr style="border: none; border-top: 1px solid #373B41; margin: 40px 0;">
    <p style="color: #707880; font-size: 12px; font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', 'Courier New', monospace;">
      <a href="{{amazonSESUnsubscribeUrl}}" style="color: #81A2BE; text-decoration: none;">Unsubscribe</a> from these emails
    </p>
  </body>
</html>`

	textBody := `Thank you for signing up for Regrada updates!

We'll keep you posted on our launch and latest developments.

CI for AI behavior — catch regressions before they ship.

- The Regrada Team

---
To unsubscribe from these emails, click here: {{amazonSESUnsubscribeUrl}}`

	input := &sesv2.SendEmailInput{
		FromEmailAddress: aws.String(s.fromEmail),
		Destination: &sesv2types.Destination{
			ToAddresses: []string{email},
		},
		Content: &sesv2types.EmailContent{
			Simple: &sesv2types.Message{
				Subject: &sesv2types.Content{
					Data:    aws.String("Welcome to Regrada!"),
					Charset: aws.String("UTF-8"),
				},
				Body: &sesv2types.Body{
					Text: &sesv2types.Content{
						Data:    aws.String(textBody),
						Charset: aws.String("UTF-8"),
					},
					Html: &sesv2types.Content{
						Data:    aws.String(htmlBody),
						Charset: aws.String("UTF-8"),
					},
				},
			},
		},
		ConfigurationSetName: aws.String("regrada-email-config"),
	}

	// Add list management options if contact list is configured
	if s.contactList != "" {
		input.ListManagementOptions = &sesv2types.ListManagementOptions{
			ContactListName: aws.String(s.contactList),
			TopicName:       aws.String("newsletter"),
		}
	}

	_, err := s.sesv2Client.SendEmail(ctx, input)
	return err
}

func (s *Service) notifyAdmin(ctx context.Context, email string) error {
	textBody := fmt.Sprintf("New email signup:\n\nEmail: %s\nTimestamp: %s",
		email, time.Now().Format(time.RFC3339))

	input := &sesv2.SendEmailInput{
		FromEmailAddress: aws.String(s.fromEmail),
		Destination: &sesv2types.Destination{
			ToAddresses: []string{s.adminEmail},
		},
		Content: &sesv2types.EmailContent{
			Simple: &sesv2types.Message{
				Subject: &sesv2types.Content{
					Data:    aws.String(fmt.Sprintf("New signup: %s", email)),
					Charset: aws.String("UTF-8"),
				},
				Body: &sesv2types.Body{
					Text: &sesv2types.Content{
						Data:    aws.String(textBody),
						Charset: aws.String("UTF-8"),
					},
				},
			},
		},
	}

	_, err := s.sesv2Client.SendEmail(ctx, input)
	return err
}

// SendInviteEmail sends an organization invite email
func (s *Service) SendInviteEmail(ctx context.Context, toEmail, inviterName, orgName, role, token string) error {
	inviteURL := fmt.Sprintf("%s/invite/%s", s.frontendURL, token)

	htmlBody := fmt.Sprintf(`
<html>
  <body style="font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', 'Courier New', monospace; background-color: #1D1F21; color: #C5C8C6; padding: 20px; margin: 0;">
    <h2 style="color: #81A2BE;">You've been invited to join %s on Regrada</h2>
    <p>%s has invited you to join <strong>%s</strong> as a <strong>%s</strong>.</p>
    <p style="margin: 30px 0;">
      <a href="%s" style="background-color: #81A2BE; color: #1D1F21; padding: 12px 24px; text-decoration: none; font-weight: bold;">Accept Invitation</a>
    </p>
    <p style="color: #969896;">Or copy this link: %s</p>
    <p style="color: #969896; margin-top: 30px;">
      <em>CI for AI behavior — catch regressions before they ship.</em>
    </p>
    <p style="color: #969896;">- The Regrada Team</p>
  </body>
</html>`, orgName, inviterName, orgName, role, inviteURL, inviteURL)

	textBody := fmt.Sprintf(`You've been invited to join %s on Regrada

%s has invited you to join %s as a %s.

Accept the invitation: %s

---
CI for AI behavior — catch regressions before they ship.

- The Regrada Team`, orgName, inviterName, orgName, role, inviteURL)

	input := &sesv2.SendEmailInput{
		FromEmailAddress: aws.String(fmt.Sprintf("%s <%s>", s.fromName, s.fromEmail)),
		Destination: &sesv2types.Destination{
			ToAddresses: []string{toEmail},
		},
		Content: &sesv2types.EmailContent{
			Simple: &sesv2types.Message{
				Subject: &sesv2types.Content{
					Data:    aws.String(fmt.Sprintf("You've been invited to join %s on Regrada", orgName)),
					Charset: aws.String("UTF-8"),
				},
				Body: &sesv2types.Body{
					Text: &sesv2types.Content{
						Data:    aws.String(textBody),
						Charset: aws.String("UTF-8"),
					},
					Html: &sesv2types.Content{
						Data:    aws.String(htmlBody),
						Charset: aws.String("UTF-8"),
					},
				},
			},
		},
	}

	_, err := s.sesv2Client.SendEmail(ctx, input)
	return err
}
