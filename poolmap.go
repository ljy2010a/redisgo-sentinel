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
	"sync"

	"github.com/garyburd/redigo/redis"
)

// concurrent map for pool
type poolMap struct {
	pools map[string]*redis.Pool
	mu    sync.RWMutex
}

func newPoolMap() *poolMap {
	return &poolMap{
		pools: make(map[string]*redis.Pool),
	}
}

func (p *poolMap) get(addr string) *redis.Pool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if _, ok := p.pools[addr]; ok {
		return p.pools[addr]
	}
	return nil
}

func (p *poolMap) set(addr string, pool *redis.Pool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, ok := p.pools[addr]; ok {
		p.pools[addr] = pool
	} else {
		p.pools[addr] = pool
	}
}

func (p *poolMap) del(addr string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, ok := p.pools[addr]; ok {
		delete(p.pools, addr)
	}
}

func (p *poolMap) keys() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	keys := []string{}
	for k := range p.pools {
		keys = append(keys, k)
	}
	return keys
}
