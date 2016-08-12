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
	"bytes"
	"fmt"
	"log"
	"os/exec"
	"testing"
	"time"

	"github.com/garyburd/redigo/redis"
)

// go test -v -race -coverprofile=c.out
// go tool cover -html=c.out -o c.html
// open c.html

func init() {
	log.SetFlags(log.Lshortfile | log.LstdFlags)
	log.SetPrefix(fmt.Sprintf("[Sentinel] "))
}

func TestSentinel(t *testing.T) {

	startAll()
	time.Sleep(time.Second * 3)
	defer func() {
		closeAll()
	}()

	sentinel := newDefaultSentinel()
	err := sentinel.Load()
	if err != nil {
		t.Fatalf("%v\n", err)
	}

	sentinel.Pool()
	sentinel.LastMasterAddr()

	timeTicker := time.NewTicker(time.Second)
	defer timeTicker.Stop()
	i := -1
	for {
		select {
		case <-timeTicker.C:
			i++
			if i > 180 {
				goto Exit
			}
			log.Printf("sentinels : %v \n", sentinel.SentinelsAddrs())
			// SETEX(sentinel, i)

			if i == 30 {
				log.Println("================ closeMaster ================")
				master.close()
			}

			if i == 70 {
				log.Println("================ closeSentinel1 ================")
				sentinel1.close()
			}

			if i == 90 {
				log.Println("================ startSentinel1 ================")
				go sentinel1.startCmd()
			}

			if i == 100 {
				log.Println("================ startMaster ================")
				go master.startCmd()
			}

			if i == 130 {
				log.Println("================ closeSlave ================")
				slave.close()
			}

		}
	}
Exit:
	sentinel.Close()
	log.Printf("Exit \n")

}

func SETEX(sentinel *Sentinel, i int) {
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
		if err != nil {
			log.Printf("setex error :  %v\n", err)
		}
	}

}

func newDefaultSentinel() *Sentinel {
	sentinel := &Sentinel{
		// SentinelAddrs: []string{"127.0.0.1:26379", "127.0.0.1:26378"},
		SentinelAddrs: []string{"127.0.0.1:26379"},
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
			TestOnBorrow: func(c redis.Conn, t time.Time) error {
				if getRole(c) != "master" {
					return fmt.Errorf("Role is not master")
				}
				return nil
			},
		},
		EnableSlaves: true,
		SlavesPoolTe: &redis.Pool{
			MaxIdle:     10,
			MaxActive:   200,
			Wait:        true,
			IdleTimeout: 60 * time.Second,
			TestOnBorrow: func(c redis.Conn, t time.Time) error {
				if getRole(c) != "slaves" {
					return fmt.Errorf("Role is not slaves")
				}
				return nil
			},
		},
		SlavesDial: func(addr string) (redis.Conn, error) {
			c, err := redis.Dial("tcp", addr)
			if err != nil {
				log.Printf("slavespool not Available %v \n", addr)
				return nil, err
			}
			log.Printf("slavespool Available at %v \n", addr)
			return c, nil
		},
	}
	return sentinel
}

var sentinel1 *serverCmd
var sentinel2 *serverCmd
var master *serverCmd
var slave *serverCmd

func closeAll() {
	sentinel1.close()
	time.Sleep(time.Second * 3)
	sentinel2.close()
	time.Sleep(time.Second * 3)
	master.close()
	time.Sleep(time.Second * 3)
	slave.close()
}

func startAll() {

	sentinel1 = newServerCmd("redis-sentinel", "test/redis-sentinel-26379", "redis-sentinel \\*:26379")
	sentinel2 = newServerCmd("redis-sentinel", "test/redis-sentinel-26378", "redis-sentinel \\*:26378")
	master = newServerCmd("redis-server", "test/redis-master", "redis-server 127.0.0.1:6378")
	slave = newServerCmd("redis-server", "test/redis-slave", "redis-server 127.0.0.1:6377")

	go sentinel1.startCmd()
	go sentinel2.startCmd()
	go master.startCmd()
	go slave.startCmd()
}

type serverCmd struct {
	cclose chan int
	cmd    string
	arg    string
	kill   string
	t      *testing.T
}

func newServerCmd(cmd, arg, kill string) *serverCmd {
	serverCmd := &serverCmd{
		cclose: make(chan int, 1),
		cmd:    cmd,
		arg:    arg,
		kill:   kill,
	}
	return serverCmd
}

func (s *serverCmd) startCmd() {
	cmd := exec.Command(s.cmd, s.arg)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Start()
	cmd.Wait()
	log.Printf("kill ProcessState : %v : cmd is %v \n", cmd.ProcessState.Success(), cmd.Args)
	if err != nil {
		// log.Printf("cmd out : %v \n", out.String())
	}

}

func (s *serverCmd) close() {
	// cmd.Process.Kill()

	cmdstr := fmt.Sprintf("ps -ef | grep '%s' | awk '{print $2}' | xargs kill -9 ", s.kill)
	cmd := exec.Command("bash", "-c", cmdstr)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	log.Printf("kill ProcessState : %v : cmd is %v \n", cmd.ProcessState.Success(), cmd.Args)
	if err != nil {
		// log.Printf("cmd out : %v \n", out.String())
	}
}
