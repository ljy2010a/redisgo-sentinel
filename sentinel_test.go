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

	startAll()
	time.Sleep(time.Second * 3)
	defer func() {
		closeAll()
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
				closeMaster()
			}

			if i == 70 {
				log.Println("================ closeSentinel1 ================")
				closeSentinel1()
			}

			if i == 90 {
				log.Println("================ startSentinel1 ================")
				startSentinel1()
			}

			if i == 120 {
				log.Println("================ startMaster ================")
				startMaster()
			}

		}
	}
Exit:
	sentinel.Close()
	log.Printf("Exit \n")
	time.Sleep(time.Second * 3)
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

func closeAll() {
	closeSentinel1()
	closeSentinel2()
	closeMaster()
	closeSlave()
}

func startAll() {
	startMaster()
	startSlave()
	startSentinel1()
	startSentinel2()
}

func startSentinel1() {
	sentinel1 = make(chan int)
	go startCmd(
		"redis-sentinel",
		"test/redis-sentinel-26379",
		sentinel1,
	)
}

func closeSentinel1() {
	sentinel1 <- 1
}

func startSentinel2() {
	sentinel2 = make(chan int)
	go startCmd(
		"redis-sentinel",
		"test/redis-sentinel-26378",
		sentinel2,
	)
}

func closeSentinel2() {
	sentinel2 <- 1
}

func startMaster() {
	master = make(chan int)
	go startCmd(
		"redis-server",
		"test/redis-master",
		master,
	)
}

func closeMaster() {
	master <- 1
}

func startSlave() {
	slave = make(chan int)
	go startCmd(
		"redis-server",
		"test/redis-slave",
		slave,
	)
}

func closeSlave() {
	slave <- 1
}

func startCmd(cmdname, arg string, close chan int,
) {
	c := make(chan int, 1)
	cmd := exec.Command(cmdname, arg)
	// cmd.Stdout = os.Stdout
	go func() {
		err := cmd.Run()
		if err == nil {
			log.Fatalf("err : %v \n", err)
		}
		c <- 1
	}()
	select {
	case <-c:
		return
	case <-close:
		cmd.Process.Kill()
	}
}
