package main

import (
	kafka_go "github.com/keitaro1020/kafka-go"
)

func main() {
	s := kafka_go.NewServer()
	s.Exec()
}
