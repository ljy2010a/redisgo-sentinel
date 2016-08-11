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
