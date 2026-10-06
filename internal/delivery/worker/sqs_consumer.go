package worker

import (
	"context"
	"fmt"
	"log"
	"time"

	appcfg "github.com/LLSWE/trading-game/internal/config"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/fx"
)

type OutboxDispatcher struct {
	db        *pgxpool.Pool
	sqsClient *sqs.Client
	queueURL  string
}

func NewOutboxDispatcher(cfg *appcfg.Config, db *pgxpool.Pool) (*OutboxDispatcher, error) {
	customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...any) (aws.Endpoint, error) {
		if cfg.SQSEndpoint != "" {
			return aws.Endpoint{
				URL:           cfg.SQSEndpoint,
				SigningRegion: cfg.AWSREGION,
			}, fmt.Errorf("unknown endpoint")
		}
		return aws.Endpoint{}, &aws.EndpointNotFoundError{}
	})

	awsCfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(cfg.AWSREGION),
		config.WithEndpointResolverWithOptions(customResolver),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load aws config for outbox: %w", err)
	}

	client := sqs.NewFromConfig(awsCfg)

	return &OutboxDispatcher{
		db:        db,
		sqsClient: client,
		queueURL:  cfg.SQSEndpoint + "/000000000000/wager-transactions.fifo",
	}, nil
}

func RegisterOutboxWorker(lc fx.Lifecycle, dispatcher *OutboxDispatcher) {
	ctx, cancel := context.WithCancel(context.Background())

	lc.Append(fx.Hook{
		OnStart: func(c context.Context) error {
			log.Println("Starting Outbox Dispatcher worker...")
			go dispatcher.Start(ctx)
			return nil
		},
		OnStop: func(c context.Context) error {
			log.Println("Stopping Outbox Dispatcher worker gracefully...")
			cancel()
			return nil
		},
	})
}

func (d *OutboxDispatcher) Start(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Outbox Dispatcher stopped.")
			return
		case <-ticker.C:
			if err := d.dispatchBatch(ctx); err != nil {
				log.Printf("Error dispatching outbox batch: %v\n", err)
			}
		}
	}
}

func (d *OutboxDispatcher) dispatchBatch(ctx context.Context) error {
	tx, err := d.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	query := `
		SELECT id, aggregate_id, event_type, payload 
		FROM outbox_events 
		WHERE status = 'PENDING' 
		ORDER BY created_at ASC 
		LIMIT 10 
		FOR UPDATE SKIP LOCKED
	`
	rows, err := tx.Query(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()

	type OutboxEvent struct {
		ID          string
		AggregateID string
		EventType   string
		Payload     []byte
	}

	var events []OutboxEvent
	for rows.Next() {
		var ev OutboxEvent
		if err := rows.Scan(&ev.ID, &ev.AggregateID, &ev.EventType, &ev.Payload); err != nil {
			return err
		}
		events = append(events, ev)
	}
	rows.Close()

	if len(events) == 0 {
		return tx.Commit(ctx)
	}

	for _, ev := range events {
		_, pubErr := d.sqsClient.SendMessage(ctx, &sqs.SendMessageInput{
			QueueUrl:               aws.String(d.queueURL),
			MessageBody:            aws.String(string(ev.Payload)),
			MessageGroupId:         aws.String(ev.AggregateID),
			MessageDeduplicationId: aws.String(ev.ID),
		})

		if pubErr != nil {
			log.Printf("Failed to publish outbox event %s to SQS: %v\n", ev.ID, pubErr)

			_, _ = tx.Exec(ctx, `UPDATE outbox_events SET attempts = attempts + 1 WHERE id = $1`, ev.ID)
			continue
		}

		_, _ = tx.Exec(ctx, `UPDATE outbox_events SET status = 'PUBLISHED' WHERE id = $1`, ev.ID)
	}

	return tx.Commit(ctx)
}
