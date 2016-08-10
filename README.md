redigo-sentinel
======
[![GoDoc](https://godoc.org/github.com/ljy2010a/redigo-sentinel?status.svg)](https://godoc.org/github.com/ljy2010a/redigo-sentinel)

redigo-sentinel is a [Go](http://golang.org/) sentinel client for the [Redis](http://redis.io/) database base on [Redigo](https://github.com/garyburd/redigo) refer by [sentinel-clients-doc](http://redis.io/topics/sentinel-clients)

Features
--------
* Redis service discovery via Sentinel
* Handling reconnections
* Sentinel failover disconnection
* Connection pools
* Error reporting
* Subscribe to Sentinel events to improve responsiveness 
	- +switch-master
	- +sentinel

Future
------
* Sentinels list automatic refresh
* Connecting to slaves
* Subscribe to Sentinel more events to improve responsiveness 

Documentation
-------------
- [API Reference](https://godoc.org/github.com/ljy2010a/redigo-sentinel)
- [sentinel-clients-doc](http://redis.io/topics/sentinel-clients)


Installation
------------

Install redigo-sentinel using the "go get" command:

    go get github.com/ljy2010a/redigo-sentinel

The Go distribution is Redigo's only dependency.

Related Projects
----------------
- [redigo](github.com/garyburd/redigo/redis) - Redigo is a Go client for the Redis database.

Example 
-------

``` gol
    sentinel := &Sentinel{
		SentinelAddrs: []string{"127.0.0.1:26379", "127.0.0.1:26379"},
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
		masterPool: &redis.Pool{
			MaxIdle:     10,
			MaxActive:   200,
			Wait:        true,
			IdleTimeout: 60 * time.Second,
		},
	}

	err := sentinel.Load()
	if err != nil {
		log.Panicf("%v\n", err)
	}

    pool := sentinel.Pool()
	if pool != nil {
		rconn := pool.Get()
		_, err := rconn.Do(
			"SETEX",
			fmt.Sprintf("test:%d", i),
			time.Second.Seconds()*3600,
			1,
			)
		rconn.Close()
		log.Printf("setex error :  %v \n", err)
	}
```

License
-------

redigo-sentinel is available under the [Apache License, Version 2.0](http://www.apache.org/licenses/LICENSE-2.0.html).