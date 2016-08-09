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

func subscribeSwitchMaster(
	masterName string,
	conn redis.Conn,
	cbReceive func(oldAddr string, newAddr string),
	cbError func(err error),
) error {
	psc := redis.PubSubConn{conn}
	psc.Subscribe("+switch-master")
	for {
		switch v := psc.Receive().(type) {
		case redis.Message:
			log.Printf("%s: message: %s\n", v.Channel, v.Data)
			parts := strings.Split(string(v.Data), " ")
			if parts[0] != masterName {
				log.Printf("sentinel: ignore new %s addr \n ", masterName)
				continue
			}
			oldAddr := net.JoinHostPort(parts[1], parts[2])
			newAddr := net.JoinHostPort(parts[3], parts[4])
			cbReceive(oldAddr, newAddr)
		case redis.Subscription:
			log.Printf(" %s: %s %d\n", v.Channel, v.Kind, v.Count)
		case error:
			log.Printf("Sentinel receive error %v \n", v)
			cbError(v)
			return v
		}
	}
}
