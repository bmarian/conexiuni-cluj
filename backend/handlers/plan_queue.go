package handlers

import (
	"runtime"
	"sync"
	"time"
)

// nginx's proxy_read_timeout: a plan still waiting by then has nobody to answer.
const proxyReadTimeout = 60 * time.Second

// Plans of every city share the cores; one is left for the live streams and nginx.
var planSlots = make(chan struct{}, max(1, runtime.GOMAXPROCS(0)-1))

// One plan at a time per client, so one address can never hold more than one core.
var planTurns = struct {
	sync.Mutex
	byClient map[string]*planTurn
}{byClient: map[string]*planTurn{}}

type planTurn struct {
	slot  chan struct{}
	users int
}

// acquirePlanSlot waits for the client's earlier plan, then for a free core.
func acquirePlanSlot(client string) (release func(), ok bool) {
	planTurns.Lock()
	turn := planTurns.byClient[client]
	if turn == nil {
		turn = &planTurn{slot: make(chan struct{}, 1)}
		planTurns.byClient[client] = turn
	}
	turn.users++
	planTurns.Unlock()
	leave := func() {
		planTurns.Lock()
		if turn.users--; turn.users == 0 {
			delete(planTurns.byClient, client)
		}
		planTurns.Unlock()
	}

	deadline := time.NewTimer(proxyReadTimeout)
	defer deadline.Stop()
	select {
	case turn.slot <- struct{}{}:
	case <-deadline.C:
		leave()
		return nil, false
	}
	select {
	case planSlots <- struct{}{}:
	case <-deadline.C:
		<-turn.slot
		leave()
		return nil, false
	}
	return func() {
		<-planSlots
		<-turn.slot
		leave()
	}, true
}
