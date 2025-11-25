package pubsub

import (
	"bytes"
	"context"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"log"

	amqp "github.com/rabbitmq/amqp091-go"
)

type SimpleQueueType string

type AckType string

const (
	Durable   SimpleQueueType = "durable"
	Transient SimpleQueueType = "transient"

	RegularAck  AckType = "ack"
	NackReque   AckType = "nack_requeue"
	NackDiscard AckType = "nack_discard"
)

func PublishJSON[T any](ch *amqp.Channel, exchange, key string, val T) error {

	data, err := json.Marshal(val)
	if err != nil {
		return err
	}

	ctx := context.Background()
	msg := amqp.Publishing{
		ContentType: "application/json",
		Body:        data,
	}

	return ch.PublishWithContext(ctx, exchange, key, false, false, msg)
}

func PublishGOB[T any](ch *amqp.Channel, exchange, key string, val T) error {

	var data bytes.Buffer
	enc := gob.NewEncoder(&data)
	enc.Encode(val)

	ctx := context.Background()
	msg := amqp.Publishing{
		ContentType: "application/gob",
		Body:        data.Bytes(),
	}

	return ch.PublishWithContext(ctx, exchange, key, false, false, msg)
}

func DeclareAndBind(
	conn *amqp.Connection,
	exchange,
	queueName,
	key string,
	queueType SimpleQueueType,
) (*amqp.Channel, amqp.Queue, error) {

	ch, err := conn.Channel()
	if err != nil {
		log.Fatalf("Could not create channel")
	}

	var durable bool
	var autoDelete bool
	var exclusive bool
	noWait := false

	table := amqp.Table{
		"x-dead-letter-exchange": "peril_dlx",
	}

	switch queueType {
	case Durable:
		durable = true
		autoDelete = false
		exclusive = false
	case Transient:
		durable = false
		autoDelete = true
		exclusive = true
	}

	q, err := ch.QueueDeclare(queueName, durable, autoDelete, exclusive, noWait, table)
	if err != nil {
		log.Fatalf("Could not create queue")
	}

	err = ch.QueueBind(queueName, key, exchange, false, nil)
	if err != nil {
		log.Fatal("Could not bind queue")
	}

	return ch, q, nil

}

func SubscribeJSON[T any](
	conn *amqp.Connection,
	exchange,
	queueName,
	key string,
	queueType SimpleQueueType,
	handler func(T) AckType,
) error {

	ch, _, err := DeclareAndBind(conn, exchange, queueName, key, queueType)
	if err != nil {
		fmt.Println("Error while declaring queue")
	}

	ch.Qos(10, 0, false)

	consumerChan, err := ch.Consume(queueName, "", false, false, false, false, nil)
	if err != nil {
		fmt.Println("Error while declaring queue")
	}

	go func() {
		for m := range consumerChan {
			var someMap T
			err := json.Unmarshal(m.Body, &someMap)
			if err != nil {
				fmt.Println("Could not unmarshal ")
			}

			ackType := handler(someMap)

			switch ackType {
			case RegularAck:
				m.Ack(false)
				fmt.Println("Ack message")
			case NackDiscard:
				m.Nack(false, false)
				fmt.Println("NackDiscard message")
			case NackReque:
				m.Nack(false, true)
				fmt.Println("NackReque message")
			}
		}
	}()

	return nil
}

func SubscribeGOB[T any](
	conn *amqp.Connection,
	exchange,
	queueName,
	key string,
	queueType SimpleQueueType,
	handler func(T) AckType,
) error {

	ch, _, err := DeclareAndBind(conn, exchange, queueName, key, queueType)
	if err != nil {
		fmt.Println("Error while declaring queue")
	}

	ch.Qos(10, 0, false)

	consumerChan, err := ch.Consume(queueName, "", false, false, false, false, nil)
	if err != nil {
		fmt.Println("Error while declaring queue")
	}

	go func() {
		for m := range consumerChan {
			var g T

			b := bytes.NewBuffer(m.Body)
			dec := gob.NewDecoder(b)
			err := dec.Decode(&g)

			if err != nil {
				fmt.Println("could not decode message ", err.Error())
				m.Nack(false, true)
				continue
			}

			ackType := handler(g)

			switch ackType {
			case RegularAck:
				m.Ack(false)
				fmt.Println("Ack message")
			case NackDiscard:
				m.Nack(false, false)
				fmt.Println("NackDiscard message")
			case NackReque:
				m.Nack(false, true)
				fmt.Println("NackReque message")
			}
		}
	}()

	return nil
}
