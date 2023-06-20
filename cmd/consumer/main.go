package main

import (
	kafka_go "github.com/keitaro1020/kafka-go"
)

func main() {
	c := kafka_go.NewConsumer()
	c.Exec()
}
