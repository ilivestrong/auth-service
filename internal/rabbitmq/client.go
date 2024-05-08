package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/ilivestrong/auth-service/internal/persist"
	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	sendotp_exchange_name            = "verification"
	exchange_type_topic              = "topic"
	sendotp_verification_routing_key = "SendOTP.newaccount"
	sendotp_queue_binding_key        = "SendOTP.*"
	sendotp_queue_name               = "otp_request"
	otpcreated_queue_name            = "otps_created"
)

type (
	MQClient interface {
		Consume()
		Publish(ctx context.Context, msg string)
	}

	otpInfo struct {
		PhoneNumber string `json:"phone_number"`
		Otp         string `json:"otp"`
	}

	otpMQClient struct {
		ch          *amqp.Channel
		profileRepo persist.ProfileRepo
	}
)

func (otpRPub *otpMQClient) Publish(ctx context.Context, msg string) {
	err := otpRPub.ch.PublishWithContext(ctx,
		sendotp_exchange_name,
		sendotp_verification_routing_key,
		false,
		false,
		amqp.Publishing{
			ContentType: "application/plain",
			Body:        []byte(msg),
		})
	failOnError(err, "Failed to publish a message")
}

func (otpEC *otpMQClient) Consume() {
	msgs, err := otpEC.ch.Consume(otpcreated_queue_name, "", true, false, false, false, nil)
	failOnError(err, "Failed to consume messages from queue")

	var forever chan struct{}
	for d := range msgs {
		log.Printf("OtpCreated event: %s", d.Body)
		otpInfo := getOtpInfo(d.Body)
		otpEC.profileRepo.UpdateOTP(otpInfo.PhoneNumber, otpInfo.Otp)
	}
	<-forever
}

func declareExchange(ch *amqp.Channel, name string) {
	err := ch.ExchangeDeclare(name, exchange_type_topic, true, false, false, false, nil)
	failOnError(err, fmt.Sprintf("failed to declare exchange: %s\n", name))
}

func declareQueue(ch *amqp.Channel) amqp.Queue {
	q, err := ch.QueueDeclare(sendotp_queue_name, false, false, false, false, nil)
	failOnError(err, "failed to declare a queue")
	return q
}

func bindQueueToExchange(q amqp.Queue, exchange string, ch *amqp.Channel) {
	err := ch.QueueBind(q.Name, sendotp_queue_binding_key, exchange, false, nil)
	failOnError(err, fmt.Sprintf("failed to bind queue to exchange: %s\n", exchange))
}

func failOnError(err error, msg string) {
	if err != nil {
		log.Panicf("%s: %s", msg, err)
	}
}

func NewOtpMQClient(amqpConn *amqp.Connection, profileRepo persist.ProfileRepo) MQClient {
	ch, err := amqpConn.Channel()
	failOnError(err, "failed to create message channel")

	declareExchange(ch, sendotp_exchange_name)
	bindQueueToExchange(declareQueue(ch), sendotp_exchange_name, ch)
	return &otpMQClient{ch, profileRepo}
}

func getOtpInfo(event []byte) *otpInfo {
	var info otpInfo
	if err := json.Unmarshal(event, &info); err != nil {
		failOnError(err, "failed to parse OTPcreated event")
		return nil
	}
	return &info
}
