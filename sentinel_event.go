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
