package messaging

import (
	amqp "github.com/rabbitmq/amqp091-go"
)

// RabbitMQBroker implements ports.BabyEventPublisher using RabbitMQ.
type RabbitMQBroker struct {
	conn      *amqp.Connection
	ch        *amqp.Channel
	queueName string
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

	return &RabbitMQBroker{
		conn:      conn,
		ch:        ch,
		queueName: queueName,
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
