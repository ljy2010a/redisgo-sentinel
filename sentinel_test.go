// Copyright 2016 ljy2010a
//
// Licensed under the Apache License, Version 2.0 (the "License"): you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
// WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
// License for the specific language governing permissions and limitations
// under the License.
package sentinel

import (
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/garyburd/redigo/redis"
)

func init() {
	log.SetFlags(log.Lshortfile | log.LstdFlags)
	log.SetPrefix(fmt.Sprintf("[Sentinel]"))
}

func TestSentinel(t *testing.T) {
	sentinel := &Sentinel{
		SentinelAddrs: []string{"127.0.0.1:26379", "127.0.0.1:26379"},
		MasterName:    "mymaster",
		SentinelDial: func(addr string) (redis.Conn, error) {
			c, err := redis.Dial("tcp", addr)
			if err != nil {
				log.Printf("sentinel not Available %v \n", addr)
				return nil, err
			}
			log.Printf("sentinel Available %v \n", addr)
			return c, nil
		},
		PoolDial: func(addr string) (redis.Conn, error) {
			log.Printf("masterpool connect to : %v ", addr)
			c, err := redis.Dial("tcp", addr)
			if err != nil {
				log.Printf("masterpool not Available %v \n", addr)
				return nil, err
			}
			log.Printf("masterpool Available at %v \n", addr)
			return c, nil
		},

		MasterPool: &redis.Pool{
			MaxIdle:     10,
			MaxActive:   200,
			Wait:        true,
			IdleTimeout: 60 * time.Second,
		},
	}

	err := sentinel.Load()
	if err != nil {
		log.Panicf("%v\n", err)
	}
	timeTicker := time.NewTicker(time.Second)
	defer timeTicker.Stop()
	i := 0
	for {
		select {
		case <-timeTicker.C:
			pool := sentinel.Pool()
			if pool != nil {
				rconn := pool.Get()
				_, err := rconn.Do(
					"SETEX",
					fmt.Sprintf("test:%d", i),
					time.Second.Seconds()*3600,
					i,
				)
				rconn.Close()
				log.Printf("setex error :  %v \n", err)
			}
		}
	}
}
