package pubsub

import (
	"errors"
	"log"
	"sync"
	"time"

	"github.com/gomodule/redigo/redis"
	rclib "github.com/lolmourne/r-pipeline/client"
)

var (
	ErrPubsubIsNotExist = errors.New("redis pubsub key is not exist")
)

type RedisPubsub interface {
	Subscribe(key string, callback func(string, error), autoReconnect bool)
	Unsubscribe(key string) error
}

type RedisPubsubImpl struct {
	mu         *sync.RWMutex
	client     rclib.RedisClient
	subscriber map[string]*Subscriber
}

type Subscriber struct {
	key           string
	conn          redis.Conn
	pubsubCon     redis.PubSubConn
	callback      func(string, error)
	autoReconnect bool
	disconnected  chan bool
	unsubscribe   chan bool
}

func (s *Subscriber) StartSubscribe() {
	timerPingSend := time.NewTicker(5 * time.Second)
	defer timerPingSend.Stop()
	timerPingReceived := time.NewTicker(10 * time.Second)
	defer timerPingReceived.Stop()
	lastTimePingReceived := time.Now().Add(15 * time.Second)

	if err := s.pubsubCon.Subscribe(s.key); err != nil {
		log.Println(err)
		s.disconnected <- true
		return
	}

	go func() {
	exit:
		for {
			select {
			case <-timerPingSend.C:
				s.conn.Do("PUBLISH", s.key, "PING")
			case <-timerPingReceived.C:
				if time.Now().After(lastTimePingReceived) {
					log.Println("UNSUBSCRIBE", s.key)
					s.pubsubCon.Unsubscribe(s.key)
					break exit
				}
			}
		}
		s.pubsubCon.Conn.Close()
		s.conn.Close()
		s.disconnected <- true
	}()

exit:
	for s.conn.Err() == nil {
		select {
		case <-s.unsubscribe:
			log.Println("triggered unsubscribe", s.key)
			s.autoReconnect = false
			s.pubsubCon.Unsubscribe(s.key)
			s.pubsubCon.Close()
			s.conn.Close()
			break exit
		default:
			switch data := s.pubsubCon.Receive().(type) {
			case redis.Message:
				if string(data.Data) == "PING" {
					lastTimePingReceived = lastTimePingReceived.Add(15 * time.Second)
				} else {
					s.callback(string(data.Data), nil)
				}
			case error:
				log.Println(data)
				break exit
			}
		}
	}
	s.disconnected <- true
}

func NewRedisPubsub(redisClient rclib.RedisClient) RedisPubsub {
	return &RedisPubsubImpl{
		mu:         &sync.RWMutex{},
		client:     redisClient,
		subscriber: make(map[string]*Subscriber),
	}
}

func (rp *RedisPubsubImpl) Unsubscribe(key string) error {
	rp.mu.Lock()
	defer rp.mu.Unlock()
	subs := rp.subscriber[key]
	if subs == nil {
		log.Println(ErrPubsubIsNotExist)
		return ErrPubsubIsNotExist
	}
	go func() {
		subs.unsubscribe <- true
	}()

	return nil
}

func (rp *RedisPubsubImpl) Subscribe(key string, callback func(string, error), autoReconnect bool) {
	rp.mu.Lock()
	defer rp.mu.Unlock()
	subs := rp.subscriber[key]
	if subs == nil {
		log.Println("Start subscription", key)
		if subscriber := rp.subscriber[key]; subscriber == nil {

			host, err := rp.client.GetNodeIPByKey(key, false)
			if err != nil {
				go rp.retrySubscriber(key, callback, autoReconnect)
				return
			}

			conn, _, err := rp.client.GetConn(host)
			if err != nil {
				go rp.retrySubscriber(key, callback, autoReconnect)
				return
			}

			_, err = conn.Do("PING")
			if err != nil {
				go rp.retrySubscriber(key, callback, autoReconnect)
				return
			}

			pubSubCon, _, err := rp.client.GetConn(host)
			_, err = conn.Do("PING")
			if err != nil {
				go rp.retrySubscriber(key, callback, autoReconnect)
				return
			}

			_, err = pubSubCon.Do("PING")
			if err != nil {
				go rp.retrySubscriber(key, callback, autoReconnect)
				return
			}

			disconnected := make(chan bool, 1)
			unsubscribe := make(chan bool, 1)
			rp.subscriber[key] = &Subscriber{
				key:           key,
				conn:          conn,
				pubsubCon:     redis.PubSubConn{Conn: pubSubCon},
				callback:      callback,
				autoReconnect: autoReconnect,
				disconnected:  disconnected,
				unsubscribe:   unsubscribe,
			}

			go rp.subscriber[key].StartSubscribe()
			go rp.monitorSubscriber(rp.subscriber[key])
		}
	} else {
		log.Println("subscription exist", key)
	}
}

func (rp *RedisPubsubImpl) retrySubscriber(key string, callback func(string, error), autoReconnect bool) {
	rp.mu.Lock()
	rp.subscriber[key] = nil
	rp.mu.Unlock()
	time.Sleep(time.Duration(15) * time.Second)
	rp.Subscribe(key, callback, autoReconnect)
}

func (rp *RedisPubsubImpl) monitorSubscriber(sub *Subscriber) {
	for {
		select {
		case <-sub.disconnected:
			if sub.autoReconnect {
				log.Println("Reconnecting subscriber...", sub.key)
				time.Sleep(time.Duration(5) * time.Second)

				key := sub.key
				callback := sub.callback
				autoReconnect := sub.autoReconnect

				rp.mu.Lock()
				rp.subscriber[sub.key] = nil
				rp.mu.Unlock()

				rp.Subscribe(key, callback, autoReconnect)
			} else {
				log.Println("Unsubscribe:", sub.key)
				rp.mu.Lock()
				rp.subscriber[sub.key] = nil
				rp.mu.Unlock()
			}
			break
		}
	}

}
