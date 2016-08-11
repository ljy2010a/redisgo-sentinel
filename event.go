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

import "sync"

type InstanceDetail struct {
	Type string
	Name string
	Addr string
}

type eventHandler struct {
	Event       string
	Ch          chan InstanceDetail
	close       chan interface{}
	HandlerFunc func(InstanceDetail)
}

func (e *eventHandler) Serve() {
	for {
		select {
		case <-e.Ch:
			e.HandlerFunc(<-e.Ch)
		case <-e.close:
			return
		}
	}
}

func (e *eventHandler) Close() {
	close(e.close)
}

type eventServer struct {
	handlers map[string]*eventHandler
	mu       sync.RWMutex
}

func newEventServer() *eventServer {
	handlers := make(map[string]*eventHandler)
	eventServer := &eventServer{
		handlers: handlers,
	}
	return eventServer
}

func (e *eventServer) Add(event string, handlerFunc func(InstanceDetail)) {
	e.mu.Lock()
	defer e.mu.Unlock()

	ch := make(chan InstanceDetail)
	eventHandler := &eventHandler{
		Ch:          ch,
		close:       make(chan interface{}),
		HandlerFunc: handlerFunc,
		Event:       event,
	}
	if old, ok := e.handlers[event]; ok {
		old.Close()
	}
	e.handlers[event] = eventHandler
	go eventHandler.Serve()
}

func (e *eventServer) Close() {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, v := range e.handlers {
		v.Close()
	}
}

// func (e *eventServer) Get(event string) *eventHandler {
// 	e.mu.RLock()
// 	defer e.mu.RUnlock()
// 	if eventHandler, ok := e.handlers[event]; ok {
// 		return eventHandler
// 	}
// 	return nil
// }

func (e *eventServer) Push(event string, i InstanceDetail) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if eventHandler, ok := e.handlers[event]; ok {
		eventHandler.Ch <- i
	}
}
