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
	"net"
	"strings"

	"github.com/garyburd/redigo/redis"
)

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
	log.Printf("get-master-addr-by-name , masterName : %v , addr : %v \n", masterName, masterAddr)
	return masterAddr, nil
}

// SENTINEL sentinels
func getSentinels(conn redis.Conn, masterName string) ([]string, error) {
	res, err := redis.Values(conn.Do("SENTINEL", "sentinels", masterName))
	if err != nil {
		return nil, err
	}
	sentinels := make([]string, 0)
	for _, a := range res {
		sm, err := redis.StringMap(a, err)
		if err != nil {
			return sentinels, err
		}
		sentinels = append(sentinels, fmt.Sprintf("%s:%s", sm["ip"], sm["port"]))
	}
	return sentinels, nil
}

const (
	cmd_switch_master = "+switch-master"
	cmd_dup_sentinel  = "-dup-sentinel"
	cmd_sentinel      = "+sentinel"
)

type sentinelSubEvent struct {
	Base         func(msg string)
	SwitchMaster func(oldAddr string, newAddr string)
	Sentinel     func(sentinelAddr string)
	Error        func(err error)
}

// SUBSCRIBE BY SENTINEL
func subscribeSentinel(
	masterName string,
	conn redis.Conn,
	event *sentinelSubEvent,
) error {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Recovered in %v \n", r)
		}
	}()
	psc := redis.PubSubConn{conn}
	psc.Subscribe(cmd_switch_master, cmd_dup_sentinel, cmd_sentinel)
	for {
		switch v := psc.Receive().(type) {
		case redis.Subscription:
			log.Printf(" %s: %s %d\n", v.Channel, v.Kind, v.Count)
		case error:
			log.Printf("Sentinel receive error %v \n", v)
			event.Error(v)
			return v
		case redis.Message:
			log.Printf("%s: message: %s\n", v.Channel, v.Data)
			event.Base(string(v.Data))
			switch v.Channel {
			case cmd_switch_master:
				parts := strings.Split(string(v.Data), " ")
				if parts[0] != masterName {
					log.Printf("sentinel: ignore new %s addr \n ", masterName)
					continue
				}
				oldAddr := net.JoinHostPort(parts[1], parts[2])
				newAddr := net.JoinHostPort(parts[3], parts[4])
				event.SwitchMaster(oldAddr, newAddr)
			case cmd_dup_sentinel:

			case cmd_sentinel:
				// back example : sentinel 127.0.0.1:26378 127.0.0.1 26378 @ mymaster 127.0.0.1 6377
				parts := strings.Split(string(v.Data), " ")
				if parts[0] != "sentinel" {
					log.Printf("not +sentinel \n ")
					continue
				}
				sentinelAddr := net.JoinHostPort(parts[2], parts[3])
				event.Sentinel(sentinelAddr)
			default:
				continue
			}
		}
	}
}
