package main

import (
	"log"
	"time"

	"github.com/gomodule/redigo/redis"
	rediscli "github.com/lolmourne/r-pipeline/client"
	"github.com/lolmourne/r-pipeline/pubsub"
)

func main() {
	log.SetFlags(log.Llongfile | log.Ldate | log.Ltime)

	redisHost := "localhost:6379"
	maxConn := 100

	client := rediscli.New(rediscli.SINGLE_MODE, redisHost,
		maxConn,
		redis.DialReadTimeout(20*time.Second),
		redis.DialWriteTimeout(20*time.Second),
		redis.DialConnectTimeout(20*time.Second))

	pubsub := pubsub.NewRedisPubsub(client)
	pubsub.Subscribe("test", pubsubCallback, true)

	for {

	}
}

func pubsubCallback(message string, err error) {
	log.Println(err, message)
}
