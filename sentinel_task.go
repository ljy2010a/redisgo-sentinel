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

// New the SentinelPool for sentinel
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
			if getRole(c) != "sentinel" {
				return fmt.Errorf("Role is not sentinel")
			}
			return nil
		},
	}
}

// provide the sentinel addrs , conn and set the map
func (s *Sentinel) refreshSentinels(addrs []string) {
	s.wrap(func() {
		for _, addr := range addrs {
			if pool := s.sentinelPools.get(addr); pool != nil {
				continue
			}
			pool := s.newSentinelPool(addr)
			if pool != nil {
				s.sentinelPools.set(addr, pool)
				go s.sentry(addr, pool)
			}
		}
	})
}

// task for refresh slaves
func (s *Sentinel) taskRefreshSlaves() {
	timeTicker := time.NewTicker(time.Second * 60)
	defer timeTicker.Stop()
	for {
		select {
		case <-timeTicker.C:
			if s.closed {
				goto Exit
			}
			addrs, err := s.slavesAddrs()
			if err != nil {
				goto Exit
			}
			s.refreshSlaves(addrs)
		}
	}
Exit:
	log.Printf("Exit taskRefreshSlaves\n")
}

// New the SentinelPool for sentinel
func (s *Sentinel) newSlavesPool(addr string) *redis.Pool {
	return &redis.Pool{
		MaxIdle:     s.SlavesPoolTe.MaxIdle,
		MaxActive:   s.SlavesPoolTe.MaxActive,
		Wait:        s.SlavesPoolTe.Wait,
		IdleTimeout: s.SlavesPoolTe.IdleTimeout,
		Dial: func() (redis.Conn, error) {
			return s.SlavesDial(addr)
		},
		TestOnBorrow: s.SlavesPoolTe.TestOnBorrow,
	}
}

// provide the slaves addrs , conn and set the map
func (s *Sentinel) refreshSlaves(addrs []string) {
	s.wrap(func() {
		for _, addr := range addrs {
			if pool := s.slavesPools.get(addr); pool != nil {
				continue
			}
			pool := s.newSlavesPool(addr)
			if pool != nil {
				s.slavesPools.set(addr, pool)
			}
		}
	})
}

// task for refresh sentinels
func (s *Sentinel) taskRefreshSentinel() {
	timeTicker := time.NewTicker(time.Second * 30)
	defer timeTicker.Stop()
	for {
		select {
		case <-timeTicker.C:
			if s.closed {
				goto Exit
			}
			addrs, err := s.sentinelAddrs()
			if err != nil {
				goto Exit
			}
			s.refreshSentinels(addrs)
		}
	}
Exit:
	log.Printf("Exit taskRefreshSentinel\n")
}
