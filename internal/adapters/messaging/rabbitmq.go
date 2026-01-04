package messaging

import (
	"github.com/AchilleasB/baby-kliniek/identity-access-service/internal/config"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/sony/gobreaker"
)

// RabbitMQBroker implements ports.BabyEventPublisher using RabbitMQ.
type RabbitMQBroker struct {
	conn      *amqp.Connection
	ch        *amqp.Channel
	queueName string
	cb        *gobreaker.CircuitBreaker
}

func NewRabbitMQBroker(amqpURL, queueName string) (*RabbitMQBroker, error) {
	conn, err := amqp.Dial(amqpURL)
	if err != nil {
		return nil, err
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, err
	}

	// Declare the queue (idempotent)
	_, err = ch.QueueDeclare(
		queueName,
		true,  // durable
		false, // autoDelete
		false, // exclusive
		false, // noWait
		nil,   // args
	)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, err
	}

	// Configure circuit breaker for RabbitMQ
	cb := config.NewCircuitBreaker("RabbitMQ-Publisher")

	return &RabbitMQBroker{
		conn:      conn,
		ch:        ch,
		queueName: queueName,
		cb:        cb,
	}, nil
}

func (rmq *RabbitMQBroker) Close() error {
	if rmq.ch != nil {
		if err := rmq.ch.Close(); err != nil {
			return err
		}
	}
	if rmq.conn != nil {
		return rmq.conn.Close()
	}
	return nil
}
