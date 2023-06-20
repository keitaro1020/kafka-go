package kafka_go

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/Shopify/sarama"
	"github.com/gin-gonic/gin"
)

type server struct{}

func NewServer() *server {
	return &server{}
}

func (s *server) Exec() {
	sendTopic := os.Getenv("SEND_TOPIC")
	receiveTopic := os.Getenv("RECEIVE_TOPIC")
	kafkaAddr := os.Getenv("KAFKA_ADDR")

	r := gin.New()

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

	consumer, err := sarama.NewConsumer([]string{kafkaAddr}, config)
	if err != nil {
		log.Fatal(err)
	}
	defer consumer.Close()

	messages := make([]*Message, 0)

	r.GET("/echo", func(gc *gin.Context) {
		gc.JSON(http.StatusOK, map[string]time.Time{
			"result": time.Now(),
		})
	})
	r.POST("/publish", func(gc *gin.Context) {
		type Input struct {
			Message string `json:"message"`
		}

		input := &Input{}
		if err := gc.ShouldBindJSON(input); err != nil {
			log.Printf("ShouldBindJSON Error: %v", err)
			gc.AbortWithError(http.StatusInternalServerError, err)
			return
		}

		message := &Message{
			Value:     input.Message,
			CreatedAt: time.Now(),
			SendInfos: []SendInfo{
				{
					SendAt: time.Now(),
					Server: "server",
				},
			},
		}

		bytes, err := json.Marshal(message)
		if err != nil {
			log.Printf("json.Marshal Error: %v", err)
			gc.AbortWithError(http.StatusInternalServerError, err)
			return
		}

		partition, offset, err := producer.SendMessage(&sarama.ProducerMessage{
			Topic: sendTopic,
			Value: sarama.ByteEncoder(bytes),
		})
		if err != nil {
			log.Printf("SendMessage Error: %v", err)
			gc.AbortWithError(http.StatusInternalServerError, err)
			return
		}

		log.Printf("message send: topic(%s), partition(%d), offset(%d)", sendTopic, partition, offset)
		gc.JSON(http.StatusOK, map[string]string{
			"result": "ok",
		})
	})
	r.GET("/messages", func(gc *gin.Context) {
		gc.JSON(http.StatusOK, messages)
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	partition, err := consumer.ConsumePartition(receiveTopic, 0, sarama.OffsetNewest)
	if err != nil {
		log.Fatal(err)
	}

	go func() {
	consumerFor:
		for {
			select {
			case msg := <-partition.Messages():
				message := &Message{}
				err := json.Unmarshal(msg.Value, message)
				if err != nil {
					log.Printf("consumer json.Unmarshal error : %v", err)
				}
				messages = append(messages, message)
				fmt.Println(fmt.Sprintf("consumed message. message: %v", msg))
			case <-ctx.Done():
				break consumerFor
			}

		}
	}()

	log.Fatal(r.Run())
}
