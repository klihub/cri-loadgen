package main

import (
	"strconv"
	"sync"
)

type idgen struct {
	sync.Mutex
	uid int
	pod int
	ctr int
}

var ids = &idgen{}

func (g *idgen) GenUID() string {
	g.Lock()
	defer g.Unlock()
	defer func() { g.uid += 1 }()

	return "uid-" + strconv.Itoa(g.uid)
}

func (g *idgen) GenPodName() string {
	g.Lock()
	defer g.Unlock()
	defer func() { g.pod += 1 }()

	return "pod-" + strconv.Itoa(g.pod)
}

func (g *idgen) GenCtrName() string {
	g.Lock()
	defer g.Unlock()
	defer func() { g.ctr += 1 }()

	return "ctr-" + strconv.Itoa(g.ctr)
}
