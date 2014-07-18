// Copyright 2014 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package queue

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/garyburd/redigo/redis"
	"github.com/tsuru/config"
	"github.com/tsuru/tsuru/log"
	"io"
	"net"
	"time"
)

type redismqQ struct {
	name    string
	prefix  string
	pool    *redis.Pool
	maxSize int
	psc     *redis.PubSubConn
}

func (r *redismqQ) Pub(msg []byte) error {
	conn := r.pool.Get()
	defer conn.Close()
	_, err := conn.Do("PUBLISH", r.key(), msg)
	return err
}

func (r *redismqQ) UnSub() error {
	if r.psc == nil {
		return nil
	}
	err := r.psc.Unsubscribe()
	if err != nil {
		return err
	}
	err = r.psc.Close()
	if err != nil {
		return err
	}
	return nil
}

func (r *redismqQ) Sub() (chan []byte, error) {
	r.psc = &redis.PubSubConn{Conn: r.pool.Get()}
	msgChan := make(chan []byte)
	err := r.psc.Subscribe(r.key())
	if err != nil {
		return nil, err
	}
	go func() {
		defer close(msgChan)
		for {
			switch v := r.psc.Receive().(type) {
			case redis.Message:
				msgChan <- v.Data
			case redis.Subscription:
				if v.Count == 0 {
					return
				}
			case error:
				log.Errorf("Error receiving messages from channel %s: %s", r.key(), v.Error())
				return
			}
		}
	}()
	return msgChan, nil
}

func (r *redismqQ) Put(m *Message, delay time.Duration) error {
	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(m)
	if err != nil {
		return err
	}
	if delay > 0 {
		go func() {
			time.Sleep(delay)
			r.put(buf.String())
		}()
		return nil
	} else {
		return r.put(buf.String())
	}
}

func (r *redismqQ) put(message string) error {
	conn := r.pool.Get()
	defer conn.Close()
	conn.Send("MULTI")
	conn.Send("LPUSH", r.key(), message)
	conn.Send("LTRIM", r.key(), 0, r.maxSize-1)
	_, err := conn.Do("EXEC")
	return err
}

func (r *redismqQ) key() string {
	return r.prefix + ":" + r.name
}

func (r *redismqQ) Get(timeout time.Duration) (*Message, error) {
	conn := r.pool.Get()
	defer conn.Close()
	secTimeout := int(timeout.Seconds())
	if secTimeout < 1 {
		secTimeout = 1
	}
	payload, err := conn.Do("BRPOP", r.key(), secTimeout)
	if err != nil {
		return nil, err
	}
	if payload == nil {
		return nil, &timeoutError{timeout: timeout}
	}
	items := payload.([]interface{})
	data := items[1].([]byte)
	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil && err != io.EOF {
		return nil, fmt.Errorf("Invalid message: %q", data)
	}
	return &msg, nil
}

type redismqQFactory struct{}

func (factory redismqQFactory) Get(name string) (Q, error) {
	return factory.get(name, "factory")
}

func (redismqQFactory) get(name, consumerName string) (*redismqQ, error) {
	host, err := config.GetString("redis-queue:host")
	if err != nil {
		host = "localhost"
	}
	port, err := config.GetString("redis-queue:port")
	if err != nil {
		if nport, err := config.GetInt("redis-queue:port"); err != nil {
			port = "6379"
		} else {
			port = fmt.Sprintf("%d", nport)
		}
	}
	password, _ := config.GetString("redis-queue:password")
	db, err := config.GetInt("redis-queue:db")
	if err != nil {
		db = 3
	}
	maxIdle, _ := config.GetInt("redis-queue:pool-max-idle-conn")
	if maxIdle == 0 {
		maxIdle = 20
	}
	idleTimeout, _ := config.GetDuration("redis-queue:pool-idle-timeout")
	if idleTimeout == 0 {
		idleTimeout = 300e9
	}
	pool := redis.NewPool(func() (redis.Conn, error) {
		conn, err := redis.Dial("tcp", host+":"+port)
		if err != nil {
			return nil, err
		}
		if password != "" {
			_, err = conn.Do("AUTH", password)
			if err != nil {
				return nil, err
			}
		}
		_, err = conn.Do("SELECT", db)
		return conn, err
	}, maxIdle)
	pool.IdleTimeout = idleTimeout
	return &redismqQ{name: name, pool: pool}, nil
}

func (factory redismqQFactory) Handler(f func(*Message), names ...string) (Handler, error) {
	name := "default"
	if len(names) > 0 {
		name = names[0]
	}
	consumerName := fmt.Sprintf("handler-%d", time.Now().UnixNano())
	queue, err := factory.get(name, consumerName)
	if err != nil {
		return nil, err
	}
	return &executor{
		inner: func() {
			if message, err := queue.Get(5e9); err == nil {
				log.Debugf("Dispatching %q message to handler function.", message.Action)
				go func(m *Message) {
					f(m)
					if m.fail {
						queue.Put(m, 0)
					}
				}(message)
			} else {
				if e, ok := err.(*net.OpError); ok && e.Op == "dial" {
					time.Sleep(5e9)
				}
			}
		},
	}, nil
}
