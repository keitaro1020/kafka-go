package kafka_go

import "time"

type Message struct {
	Value     string     `json:"value"`
	CreatedAt time.Time  `json:"createdAt"`
	SendInfos []SendInfo `json:"sendInfos"`
}

type SendInfo struct {
	SendAt time.Time `json:"sendAt"`
	Server string    `json:"server"`
}
