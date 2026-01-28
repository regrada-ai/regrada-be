package email

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/aws/aws-sdk-go-v2/service/ses/types"
)

type Service struct {
	client    *ses.Client
	fromEmail string
	fromName  string
}

type EmailMessage struct {
	To      []string
	Subject string
	Body    string
	IsHTML  bool
}

func NewService(region, fromEmail, fromName string) (*Service, error) {
	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(region),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := ses.NewFromConfig(cfg)

	return &Service{
		client:    client,
		fromEmail: fromEmail,
		fromName:  fromName,
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
