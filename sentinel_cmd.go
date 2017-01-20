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
	"net"
	"strings"

	"github.com/garyburd/redigo/redis"
)

// ROLE
func getRole(c redis.Conn) string {
	res, err := c.Do("ROLE")
	if err != nil {
		logger.Error(err)
		return ""
	}
	rres, ok := res.([]interface{})
	if ok {
		if str, err := redis.String(rres[0], nil); err != nil {
			logger.Error(err)
		} else {
			return str
		}
	}
	return ""
}

// SENTINEL get-master-addr-by-name
func getMasterAddrByName(conn redis.Conn, masterName string) (string, error) {
	res, err := redis.Strings(conn.Do("SENTINEL", "get-master-addr-by-name", masterName))
	if err != nil {
		return "", err
	}
	if len(res) != 2 {
		return "", fmt.Errorf("master-addr not right")
	}
	masterAddr := net.JoinHostPort(res[0], res[1])
	logger.Debugf("get-master-addr-by-name , masterName : %v , addr : %v ", masterName, masterAddr)
	return masterAddr, nil
}

// SENTINEL sentinels
func getSentinels(conn redis.Conn, masterName string) ([]string, error) {
	res, err := redis.Values(conn.Do("SENTINEL", "sentinels", masterName))
	if err != nil {
		return nil, err
	}
	sentinels := []string{}
	for _, a := range res {
		sm, err := redis.StringMap(a, err)
		if err != nil {
			return sentinels, err
		}
		sentinels = append(sentinels, net.JoinHostPort(sm["ip"], sm["port"]))
	}
	return sentinels, nil
}

// SENTINEL slaves
func getSlaves(conn redis.Conn, masterName string) ([]string, error) {
	res, err := redis.Values(conn.Do("SENTINEL", "slaves", masterName))
	if err != nil {
		return nil, err
	}
	slaves := []string{}
	for _, a := range res {
		sm, err := redis.StringMap(a, err)
		if err != nil {
			return slaves, err
		}
		slaves = append(slaves, net.JoinHostPort(sm["ip"], sm["port"]))
	}
	return slaves, nil
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
			logger.Errorf("canot run cmd by sentinel %v ", addr)
			continue
		}
		return reply, nil
	}
	return nil, fmt.Errorf("no sentinel was useful")
}

const (
	cmd_switch_master = "+switch-master"
	cmd_dup_sentinel  = "-dup-sentinel"
	cmd_sentinel      = "+sentinel"
	cmd_add_o_down    = "+odown"
	cmd_min_o_down    = "+odown"
	cmd_reboot        = "+reboot"
)

func (s *Sentinel) subscribeSentinel(conn redis.Conn) error {
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("Recovered in %v ", r)
		}
	}()
	psc := redis.PubSubConn{
		Conn: conn,
	}
	psc.Subscribe(
		cmd_switch_master,
		cmd_sentinel,
		cmd_add_o_down,
		cmd_min_o_down,
		cmd_reboot,
	)
	for {
		switch v := psc.Receive().(type) {
		case redis.Subscription:
			logger.Debugf(" %s: %s %d ", v.Channel, v.Kind, v.Count)
		case error:
			logger.Errorf("Sentinel receive error %v", v)
			return v
		case redis.Message:
			logger.Debugf("%s: message: %s ", v.Channel, v.Data)
			s.analysisMessage(v.Channel, v.Data)
		}
	}
}

func (s *Sentinel) analysisMessage(channel string, data []byte) {
	switch channel {
	case cmd_switch_master:
		parts := strings.Split(string(data), " ")
		// oldAddr := net.JoinHostPort(parts[1], parts[2])
		newAddr := net.JoinHostPort(parts[3], parts[4])
		instance := InstanceDetail{
			Addr: newAddr,
		}
		s.eventSrv.Push(channel, instance)
	case cmd_sentinel:
		parts := strings.Split(string(data), " ")
		if parts[0] != "sentinel" {
			logger.Error("not +sentinel")
			return
		}
		sentinelAddr := net.JoinHostPort(parts[2], parts[3])
		instance := InstanceDetail{
			Addr: sentinelAddr,
		}
		s.eventSrv.Push(channel, instance)
	case cmd_add_o_down:
	default:
		return
	}
}
