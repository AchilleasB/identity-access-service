package messaging

import (
	"context"
	"encoding/json"
	"time"

	"github.com/AchilleasB/baby-kliniek/identity-access-service/internal/core/ports"
	amqp "github.com/rabbitmq/amqp091-go"
)

func (rmq *RabbitMQBroker) PublishBabyCreated(ctx context.Context, evt ports.CreateBabyEvent) error {
	body, err := json.Marshal(evt)
	if err != nil {
		return err
	}

	// Respect context deadline
	if deadline, ok := ctx.Deadline(); ok {
		if time.Until(deadline) <= 0 {
			return ctx.Err()
		}
	}

	// Use circuit breaker to protect RabbitMQ publish operation
	_, err = rmq.cb.Execute(func() (interface{}, error) {
		err := rmq.ch.PublishWithContext(
			ctx,
			"",            // exchange (default)
			rmq.queueName, // routing key == queue name
			false,         // mandatory
			false,         // immediate
			amqp.Publishing{
				ContentType:  "application/json",
				DeliveryMode: amqp.Persistent,
				Body:         body,
			},
		)
		return nil, err
	})
	return err
}
