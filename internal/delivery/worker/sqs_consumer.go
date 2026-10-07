package worker

import (
	"context"
	"encoding/json"
	"log"
	"time"

	appcfg "github.com/LLSWE/trading-game/internal/config"
	"github.com/LLSWE/trading-game/internal/usecase"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/fx"
)

type SQSConsumer struct {
	db        *pgxpool.Pool
	sqsClient *sqs.Client
	queueURL  string
	wagerUC   *usecase.WageringUseCase
}

func NewSQSConsumer(cfg *appcfg.Config, db *pgxpool.Pool, wagerUC *usecase.WageringUseCase) (*SQSConsumer, error) {
	ctx := context.TODO()
	customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...any) (aws.Endpoint, error) {
		if cfg.SQSEndpoint != "" {
			return aws.Endpoint{
				URL:               cfg.SQSEndpoint,
				SigningRegion:     cfg.AWSREGION,
				HostnameImmutable: true,
			}, nil
		}
		return aws.Endpoint{}, &aws.EndpointNotFoundError{}
	})

	awsCfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(cfg.AWSREGION),
		config.WithEndpointResolverWithOptions(customResolver),
	)
	if err != nil {
		return nil, err
	}

	client := sqs.NewFromConfig(awsCfg)

	return &SQSConsumer{
		db:        db,
		sqsClient: client,
		queueURL:  cfg.SQSEndpoint + "/000000000000/wager-transactions.fifo",
		wagerUC:   wagerUC,
	}, nil
}

func RegisterSQSConsumerWorker(lc fx.Lifecycle, consumer *SQSConsumer) {
	ctx, cancel := context.WithCancel(context.Background())

	lc.Append(fx.Hook{
		OnStart: func(c context.Context) error {
			log.Println("Starting SQS Consumer worker with Inbox protection...")
			go consumer.Start(ctx)
			return nil
		},
		OnStop: func(c context.Context) error {
			log.Println("Stopping SQS Consumer worker...")
			cancel()
			return nil
		},
	})
}

func (c *SQSConsumer) Start(ctx context.Context) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.pollMessages(ctx)
		}
	}
}

func (c *SQSConsumer) pollMessages(ctx context.Context) {
	output, err := c.sqsClient.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:            aws.String(c.queueURL),
		MaxNumberOfMessages: 5,
		WaitTimeSeconds:     2,
	})
	if err != nil || len(output.Messages) == 0 {
		return
	}

	for _, msg := range output.Messages {
		msgID := aws.ToString(msg.MessageId)
		receiptHandle := aws.ToString(msg.ReceiptHandle)
		body := aws.ToString(msg.Body)

		tx, err := c.db.Begin(ctx)
		if err != nil {
			continue
		}

		consumerName := "wager-consumer"

		_, err = tx.Exec(ctx, `
			INSERT INTO inbox_messages (consumer_name, message_id, message_hash)
			VALUES ($1, $2, $3)
			ON CONFLICT (consumer_name, message_id) DO NOTHING
		`, consumerName, msgID, msgID)
		if err != nil {
			tx.Rollback(ctx)
			continue
		}

		var req usecase.WagerRequest
		if err := json.Unmarshal([]byte(body), &req); err == nil && req.ProviderID != "" {
			_, _ = c.wagerUC.ProcessTransaction(ctx, req)
		}

		if err := tx.Commit(ctx); err == nil {
			_, _ = c.sqsClient.DeleteMessage(ctx, &sqs.DeleteMessageInput{
				QueueUrl:      aws.String(c.queueURL),
				ReceiptHandle: aws.String(receiptHandle),
			})
		}
	}
}
