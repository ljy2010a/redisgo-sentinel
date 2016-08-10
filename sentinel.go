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
	"sync"
	"sync/atomic"
	"time"

	"github.com/garyburd/redigo/redis"
)

// Sentinel maintains a pool of master *redigo.Poll.
// The application calls the Pool method to get the pool.
// NOTICE : as the the switch-master signal tick Sentinel will try to reconnect
// the new master
// you should always get the pool from Sentinel , do not keep the pool for
// your own
// Example like redigo
// The following example shows how to use a Sentinel in application. The
// application creates a Sentinel at application startup and makes it available
// to
// request handlers using a global variable.
type Sentinel struct {
	//keep the sentinel addrs , when pub `+sentinel` will change
	SentinelAddrs []string

	//sentinel mastername
	MasterName string

	// SentinelDial is an application supplied function for creating and
	// configuring a
	// connection.
	//
	// The connection returned from Dial must not be in a special state
	// (subscribed to pubsub channel, transaction started, ...).
	SentinelDial func(addr string) (redis.Conn, error)

	// A concurrent Map to save the sentinel pool
	sentinelPools *poolMap

	// This dial is for the master pool like redigo , but as the sentinel model
	// , the master addr will be filled after sentinel get the addr by name
	//
	// The connection returned from Dial must not be in a special state
	// (subscribed to pubsub channel, transaction started, ...).
	PoolDial func(addr string) (redis.Conn, error)

	// Keep the master *redigo.Pool
	masterPool *redis.Pool

	// SlavesDial is an application supplied function for creating and
	// configuring a
	// connection.
	//
	// The connection returned from Dial must not be in a special state
	// (subscribed to pubsub channel, transaction started, ...).
	SlavesDial func(addr string) (redis.Conn, error)

	// A concurrent Map to save the sentinel pool
	slavesPools *poolMap

	//to save the last addr for master addr
	lastMasterAddr string

	//
	enableSlaves bool

	//
	closed bool

	//
	wg sync.WaitGroup
}

// Return the sentinel's master addr
func (s *Sentinel) LastMasterAddr() string {
	return s.lastMasterAddr
}

// Get the master pool from sentinel.
// The application should run the Load() func below first
// NOTICE : When the master ODOWN , you will get a bad conn from the pool
// you should not keep the pool for your own,please get pool from sentinel
// always
func (s *Sentinel) Pool() *redis.Pool {
	return s.masterPool
}

// Get the sentinelsAddrs snapshot
// do not make sure all the addr available
func (s *Sentinel) SentinelsAddrs() []string {
	s.SentinelAddrs = s.sentinelPools.keys()
	return s.SentinelAddrs
}

// Synchronize
func (s *Sentinel) wrap(f func()) {
	s.wg.Add(1)
	func() {
		if !s.closed {
			f()
		}
		s.wg.Done()
	}()
}

// Close all the pool
func (s *Sentinel) Close() {
	s.closed = true
	s.wg.Wait()
	sentinelAddrs := s.sentinelPools.keys()
	for _, addr := range sentinelAddrs {
		if pool := s.sentinelPools.get(addr); pool != nil {
			pool.Close()
		}
		s.sentinelPools.del(addr)
	}

	if s.masterPool != nil {
		s.masterPool.Close()
	}
}

// Begin to run the Sentinel. Here is the Process below
// 1. Connect the sentinels , add it to sentinelPools , start sentry() to
// subscribe the news from sentinel-server
// 2. Get the master addr from sentinel
// 3. Start the monitors to keep the sentinel available
func (s *Sentinel) Load() error {
	s.sentinelPools = newPoolMap()
	s.slavesPools = newPoolMap()
	s.closed = false
	log.Printf("sentinel begin to conn %v \n", s.SentinelAddrs)

	// connect the sentinel user offer
	s.refreshSentinels(s.SentinelAddrs)

	log.Printf("sentinel has to conn %v \n", s.sentinelPools.keys())

	// search for the other sentinel
	sentinelAddrs, err := s.sentinelAddrs()
	if err != nil {
		return err
	}

	// connect the left over sentinel
	s.refreshSentinels(sentinelAddrs)

	//reset the connected sentinelAddrs
	s.SentinelAddrs = s.sentinelPools.keys()

	// get the master addr form sentinel
	masterAddr, err := s.masterAddr()
	if err != nil {
		return err
	}

	s.lastMasterAddr = masterAddr
	log.Printf("sentinel load master  %v \n", s.lastMasterAddr)

	s.masterPool.Dial = func() (redis.Conn, error) {
		return s.PoolDial(s.lastMasterAddr)
	}

	go s.taskRefreshSentinel()
	// TODO : ADD SLAVES POOL FOR CONN
	// TODO : MAKE THE LOG PRETTY
	return err
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

// Monitor the `+sentinel` .
// When `+sentinel` tick , check the sentinel if not connected
// to add the sentinel
func (s *Sentinel) monitorSentinelAddrs(addr string) {
	s.wrap(func() {
		log.Printf("monitorSentinelAddrs %v \n", addr)
		if pool := s.sentinelPools.get(addr); pool != nil {
			return
		}
		pool := s.newSentinelPool(addr)
		if pool != nil {
			s.sentinelPools.set(addr, pool)
			go s.sentry(addr, pool)
		}
	})
}

// Monitor `switch-master` .
// When `switch-master` tick , check the master addr if equal LastMasterAddr
// to reset masterPool ,
func (s *Sentinel) monitorSwitchMaster(oldAddr string, newAddr string) {
	s.wrap(func() {
		if s.lastMasterAddr == newAddr {
			log.Println("the new addr do not need to reconnect")
			return
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

// Get the master addr from sentinel conn
func (s *Sentinel) masterAddr() (string, error) {
	res, err := s.cmdToSentinels(
		func(c redis.Conn) (interface{}, error) {
			return getMasterAddrByName(c, s.MasterName)
		},
	)
	if err != nil {
		return "", err
	}
	return res.(string), nil
}

// Get the sentinels from sentinel conn
func (s *Sentinel) sentinelAddrs() ([]string, error) {
	res, err := s.cmdToSentinels(
		func(c redis.Conn) (interface{}, error) {
			return getSentinels(c, s.MasterName)
		},
	)
	if err != nil {
		return nil, err
	}
	return res.([]string), nil
}

// Get the slaves from sentinel conn
func (s *Sentinel) slavesAddrs() ([]string, error) {
	res, err := s.cmdToSentinels(
		func(c redis.Conn) (interface{}, error) {
			return getSlaves(c, s.MasterName)
		},
	)
	if err != nil {
		return nil, err
	}
	return res.([]string), nil
}

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
			_, err := c.Do("PING")
			return err
		},
	}
}

// Run the cmd to sentinels muliply until get the result
// If all the sentinel fail return `no sentinel was useful`
func (s *Sentinel) cmdToSentinels(
	f func(redis.Conn) (interface{}, error),
) (interface{}, error) {
	addrs := s.sentinelPools.keys()
	for _, addr := range addrs {
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

// Sentinel sentry for sub events
func (s *Sentinel) sentry(addr string, pool *redis.Pool) error {
	var failTimes int64
RESTART:
	conn := pool.Get()
	event := &sentinelSubEvent{
		Base: func(msg string) {
			atomic.StoreInt64(&failTimes, 0)
		},
		SwitchMaster: s.monitorSwitchMaster,
		Sentinel:     s.monitorSentinelAddrs,
		Error: func(err error) {
			log.Println(err)
			atomic.AddInt64(&failTimes, 1)
		},
	}

	err := subscribeSentinel(
		s.MasterName,
		conn,
		event,
	)

	if s.closed {
		return nil
	}

	if atomic.LoadInt64(&failTimes) > 3 {
		if pool := s.sentinelPools.get(addr); pool != nil {
			pool.Close()
		}
		s.sentinelPools.del(addr)
	} else {
		goto RESTART
	}

	return err
}
