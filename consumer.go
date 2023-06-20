package kafka_go

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"sync"
	"time"

	"github.com/Shopify/sarama"
)

type consumer struct{}

func NewConsumer() *consumer {
	return &consumer{}
}

func (s *consumer) Exec() {
	sendTopic := os.Getenv("SEND_TOPIC")
	receiveTopic := os.Getenv("RECEIVE_TOPIC")
	cGroup := os.Getenv("CONSUMER_GROUP")
	kafkaAddr := os.Getenv("KAFKA_ADDR")

	config := sarama.NewConfig()
	config.Producer.Return.Successes = true
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Retry.Max = 5

	config.Consumer.Return.Errors = true

	producer, err := sarama.NewSyncProducer([]string{kafkaAddr}, config)
	if err != nil {
		log.Fatal(err)
	}
	defer producer.Close()

	consumerGroup, err := sarama.NewConsumerGroup([]string{kafkaAddr}, cGroup, config)
	if err != nil {
		log.Fatal(err)
	}
	defer consumerGroup.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	consumer := Consumer{
		ready:     make(chan bool),
		group:     cGroup,
		sendTopic: sendTopic,
		producer:  producer,
	}

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			// `Consume` should be called inside an infinite loop, when a
			// server-side rebalance happens, the consumer session will need to be
			// recreated to get the new claims
			if err := consumerGroup.Consume(ctx, []string{receiveTopic}, &consumer); err != nil {
				log.Panicf("Error from consumer: %v", err)
			}
			// check if context was cancelled, signaling that the consumer should stop
			if ctx.Err() != nil {
				return
			}
			consumer.ready = make(chan bool)
		}
	}()

	<-consumer.ready // Await till the consumer has been set up
	log.Println("Sarama consumer up and running!...")

	keepRunning := true
	for keepRunning {
		select {
		case <-ctx.Done():
			log.Println("terminating: context cancelled")
			keepRunning = false
		}
	}
	cancel()
	wg.Wait()
	if err = consumerGroup.Close(); err != nil {
		log.Panicf("Error closing client: %v", err)
	}
}

// Consumer represents a Sarama consumer group consumer
type Consumer struct {
	ready     chan bool
	group     string
	sendTopic string
	producer  sarama.SyncProducer
}

// Setup is run at the beginning of a new session, before ConsumeClaim
func (c *Consumer) Setup(sarama.ConsumerGroupSession) error {
	// Mark the consumer as ready
	close(c.ready)
	return nil
}

// Cleanup is run at the end of a session, once all ConsumeClaim goroutines have exited
func (c *Consumer) Cleanup(sarama.ConsumerGroupSession) error {
	return nil
}

// ConsumeClaim must start a consumer loop of ConsumerGroupClaim's Messages().
func (c *Consumer) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for {
		select {
		case msg := <-claim.Messages():
			log.Printf("Message claimed: value = %s, timestamp = %v, topic = %s", string(msg.Value), msg.Timestamp, msg.Topic)

			message := &Message{}
			err := json.Unmarshal(msg.Value, message)
			if err != nil {
				log.Printf("consumer json.Unmarshal error : %v", err)
			}

			message.SendInfos = append(message.SendInfos, SendInfo{
				SendAt: time.Now(),
				Server: c.group,
			})

			bytes, err := json.Marshal(message)
			if err != nil {
				log.Printf("json.Marshal Error: %v", err)
			}

			partition, offset, err := c.producer.SendMessage(&sarama.ProducerMessage{
				Topic: c.sendTopic,
				Value: sarama.ByteEncoder(bytes),
			})
			if err != nil {
				log.Printf("child send error %v", err)
			}
			log.Printf("message send: topic(%s), partition(%d), offset(%d)", c.sendTopic, partition, offset)

			session.MarkMessage(msg, "")

		case <-session.Context().Done():
			return nil
		}
	}
}
