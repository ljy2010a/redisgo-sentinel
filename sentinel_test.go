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

// go test -coverprofile=c.out
// go tool cover -html=c.out -o c.html
// open c.html

func init() {
	log.SetFlags(log.Lshortfile | log.LstdFlags)
	log.SetPrefix(fmt.Sprintf("[Sentinel] "))
}

func TestSentinel(t *testing.T) {

	startAll(t)
	time.Sleep(time.Second * 3)
	defer func() {
		time.Sleep(time.Second * 3)
		closeAll(t)
		return
	}()

	sentinel := newDefaultSentinel()
	err := sentinel.Load()
	if err != nil {
		t.Fatalf("%v\n", err)
	}
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
			SETEX(sentinel, i)

			if i == 30 {
				log.Println("================ closeMaster ================")
				closeMaster(t)
			}

			if i == 70 {
				log.Println("================ closeSentinel1 ================")
				closeSentinel1(t)
			}

			if i == 90 {
				log.Println("================ startSentinel1 ================")
				startSentinel1(t)
			}

			if i == 120 {
				log.Println("================ startMaster ================")
				startMaster(t)
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
	return sentinel
}

var sentinel1 = make(chan int)
var sentinel2 = make(chan int)
var master = make(chan int)
var slave = make(chan int)

func closeAll(t *testing.T) {
	closeSentinel1(t)
	closeSentinel2(t)
	closeMaster(t)
	closeSlave(t)
}

func startAll(t *testing.T) {
	startMaster(t)
	startSlave(t)
	startSentinel1(t)
	startSentinel2(t)
}

func startSentinel1(t *testing.T) {
	log.Println("startSentinel1")
	sentinel1 = make(chan int)
	go startCmd(
		t,
		"redis-sentinel",
		"test/redis-sentinel-26379",
		sentinel1,
	)
}

func closeSentinel1(t *testing.T) {
	sentinel1 <- 1
}

func startSentinel2(t *testing.T) {
	sentinel2 = make(chan int)
	log.Println("startSentinel2")
	go startCmd(
		t,
		"redis-sentinel",
		"test/redis-sentinel-26378",
		sentinel2,
	)
}

func closeSentinel2(t *testing.T) {
	sentinel2 <- 1
}

func startMaster(t *testing.T) {
	master = make(chan int)
	log.Println("startMaster")
	go startCmd(
		t,
		"redis-server",
		"test/redis-master",
		master,
	)
}

func closeMaster(t *testing.T) {
	master <- 1
}

func startSlave(t *testing.T) {
	slave = make(chan int)
	log.Println("startSlave")
	go startCmd(
		t,
		"redis-server",
		"test/redis-slave",
		slave,
	)
}

func closeSlave(t *testing.T) {
	slave <- 1
}

func startCmd(t *testing.T, cmdname, arg string, close chan int,
) {
	c := make(chan int, 1)
	cmd := exec.Command(cmdname, arg)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	go func() {
		err := cmd.Run()
		if err != nil {
			t.Fatalf("out : %v \n", out.String())
		}
		c <- 1
	}()
	select {
	case <-c:
	case <-close:
		cmd.Process.Kill()
	}
}
