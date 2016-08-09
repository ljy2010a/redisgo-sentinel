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
	"time"

	"github.com/garyburd/redigo/redis"
)

type Sentinel struct {
	SentinelAddrs []string

	MasterName string

	SentinelDial func(addr string) (redis.Conn, error)

	PoolDial func(addr string) (redis.Conn, error)

	lastMasterAddr string

	sentinelPools *poolMap

	masterPool *redis.Pool

	stop bool

	// SwitchNotify bool

	switchMaster chan string

	// RecoveryTime time.Duration
}

func (s *Sentinel) LastMasterAddr() string {
	return s.lastMasterAddr
}

// func (s *Sentinel) SwitchMasterChan() chan string {
// 	if s.SwitchNotify {
// 		return s.switchMaster
// 	}
// 	return nil
// }

func (s *Sentinel) Pool() *redis.Pool {
	return s.masterPool
}

func (s *Sentinel) Load() error {
	s.switchMaster = make(chan string, 10)
	s.sentinelPools = newPoolMap()
	// s.RecoveryTime = 30 * time.Second

	log.Printf("sentinel begin to conn %v \n", s.SentinelAddrs)
	for _, addr := range s.SentinelAddrs {
		pool := s.newSentinelPool(addr)
		if pool != nil {
			s.sentinelPools.set(addr, pool)
			go s.sentry(addr, pool)
		}
	}
	var err error
	masterAddr, err := s.masterAddr()
	if err != nil {
		return err
	}

	s.lastMasterAddr = masterAddr
	log.Printf("sentinel load master  %v \n", s.lastMasterAddr)

	s.masterPool.Dial = func() (redis.Conn, error) {
		return s.PoolDial(s.lastMasterAddr)
	}

	go s.monitorSwitchMaster()
	// go s.monitorSentinelAddrs()
	// go scan sentinel addrs
	return err
}

// func (s *Sentinel) monitorSentinelAddrs() {
// }

func (s *Sentinel) monitorSwitchMaster() {
	for {
		select {
		case <-s.switchMaster:
			newAddr := <-s.switchMaster
			if s.lastMasterAddr == newAddr {
				log.Println("lastMasterAddr match the new addr do not need to reconnect")
				break
			}
			s.masterPool.Close()
			redisPool := &redis.Pool{
				MaxIdle:     s.masterPool.MaxIdle,
				MaxActive:   s.masterPool.MaxActive,
				Wait:        s.masterPool.Wait,
				IdleTimeout: s.masterPool.IdleTimeout,
				Dial: func() (redis.Conn, error) {
					return s.PoolDial(s.lastMasterAddr)
				},
			}
			s.masterPool = redisPool
			s.lastMasterAddr = newAddr
			break
		}
	}
}

func (s *Sentinel) masterAddr() (string, error) {
	res, err := s.cmdToSentinels(func(c redis.Conn) (interface{}, error) {
		return getMasterAddrByName(c, s.MasterName)
	})
	if err != nil {
		return "", err
	}
	return res.(string), nil
}

func (s *Sentinel) newSentinelPool(addr string) *redis.Pool {
	return &redis.Pool{
		MaxIdle:     3,
		MaxActive:   10,
		Wait:        true,
		IdleTimeout: 240 * time.Second,
		Dial: func() (redis.Conn, error) {
			return s.SentinelDial(addr)
		},
		TestOnBorrow: func(c redis.Conn, t time.Time) error {
			_, err := c.Do("PING")
			return err
		},
	}
}

func (s *Sentinel) cmdToSentinels(f func(redis.Conn) (interface{}, error)) (interface{}, error) {
	for _, addr := range s.SentinelAddrs {
		pool := s.sentinelPools.get(addr)
		if pool == nil {
			continue
		}
		conn := pool.Get()
		reply, err := f(conn)
		conn.Close()
		if err != nil {
			log.Printf("canot run cmd by sentinel %v \n", addr)
			continue
		}
		return reply, nil
	}
	return nil, fmt.Errorf("no sentinel was useful")
}

func (s *Sentinel) sentry(addr string, pool *redis.Pool) error {
	conn := pool.Get()
	err := subscribeSwitchMaster(
		s.MasterName,
		conn,
		func(oldAddr string, newAddr string) {
			log.Printf("master addr has move to : %v from %v \n", newAddr, oldAddr)
			s.lastMasterAddr = newAddr
			// if s.SwitchNotify {
			s.switchMaster <- newAddr
			// }
		},
		func(err error) {
			log.Println(err)
			s.sentinelPools.del(addr)
		},
	)
	return err
}
