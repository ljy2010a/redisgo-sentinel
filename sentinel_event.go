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
	"log"

	"github.com/garyburd/redigo/redis"
)

// Monitor the `+sentinel` .
// When `+sentinel` tick , check the sentinel if not connected
// to add the sentinel
func (s *Sentinel) addSentinel(newSentinel InstanceDetail) {
	s.wrap(func() {
		log.Printf("monitorSentinelAddrs %v \n", newSentinel.Addr)
		if pool := s.sentinelPools.get(newSentinel.Addr); pool != nil {
			return
		}
		pool := s.newSentinelPool(newSentinel.Addr)
		if pool != nil {
			s.sentinelPools.set(newSentinel.Addr, pool)
			go s.sentry(newSentinel.Addr, pool)
		}
	})
}

// Monitor `switch-master` .
// When `switch-master` tick , check the master addr if equal LastMasterAddr
// to reset masterPool ,
func (s *Sentinel) switchMaster(newMaster InstanceDetail) {
	s.wrap(func() {
		log.Printf(" switchMaster %v \n", newMaster.Addr)
		if s.lastMasterAddr == newMaster.Addr {
			log.Println("the new addr do not need to reconnect")
			return
		}
		s.MasterPool.Close()
		redisPool := &redis.Pool{
			MaxIdle:     s.MasterPool.MaxIdle,
			MaxActive:   s.MasterPool.MaxActive,
			Wait:        s.MasterPool.Wait,
			IdleTimeout: s.MasterPool.IdleTimeout,
			Dial: func() (redis.Conn, error) {
				return s.PoolDial(newMaster.Addr)
			},
		}
		s.MasterPool = redisPool
		s.lastMasterAddr = newMaster.Addr
	})
}
