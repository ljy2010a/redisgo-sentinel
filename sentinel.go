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
	"sync"

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
	MasterPool *redis.Pool

	// SlavesDial is an application supplied function for creating and
	// configuring a
	// connection.
	//
	// The connection returned from Dial must not be in a special state
	// (subscribed to pubsub channel, transaction started, ...).
	SlavesDial func(addr string) (redis.Conn, error)

	// A concurrent Map to save the sentinel pool
	slavesPools *poolMap

	// Slaves Pool templete
	SlavesPoolTe *redis.Pool

	//to save the last addr for master addr
	lastMasterAddr string

	// enable to get the slaves pool
	EnableSlaves bool

	// when master odwon  switch a slaves for readonly
	AutoSwitchSlaves bool

	//
	eventSrv *eventServer
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
	return s.MasterPool
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
	log.Println("close sentinel begin wait all proc stop")
	s.wg.Wait()
	log.Println("all proc stop")
	s.eventSrv.Close()
	sentinelAddrs := s.sentinelPools.keys()
	for _, addr := range sentinelAddrs {
		if pool := s.sentinelPools.get(addr); pool != nil {
			pool.Close()
		}
		s.sentinelPools.del(addr)
	}

	if s.MasterPool != nil {
		s.MasterPool.Close()
	}
	log.Println("close sentinel done ")
}

// Begin to run the Sentinel. Here is the Process below
// 1. Connect the sentinels , add it to sentinelPools , start sentry() to
// subscribe the news from sentinel-server
// 2. Get the master addr from sentinel
// 3. Start the monitors to keep the sentinel available
func (s *Sentinel) Load() error {
	s.sentinelPools = newPoolMap()
	s.eventSrv = newEventServer()

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

	s.MasterPool.Dial = func() (redis.Conn, error) {
		return s.PoolDial(s.lastMasterAddr)
	}

	go s.taskRefreshSentinel()

	if s.EnableSlaves {
		s.slavesPools = newPoolMap()
		slaveAddrs, err := s.slavesAddrs()
		if err != nil {
			return err
		}
		s.refreshSlaves(slaveAddrs)
		go s.taskRefreshSlaves()
	}

	s.eventSrv.Add(cmd_switch_master, s.switchMaster)
	s.eventSrv.Add(cmd_sentinel, s.addSentinel)
	return err
}

// Sentinel sentry for sub events
func (s *Sentinel) sentry(addr string, pool *redis.Pool) error {
	var failTimes int64
RESTART:
	if s.closed {
		return nil
	}
	conn := pool.Get()
	err := s.subscribeSentinel(conn)

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

