package main

/*
import (
	"fmt"
)
*/

type eventHandler struct {
	notify chan *notifEvent
	close  chan uint32
}

func NewEventHandler() *eventHandler {
	return &eventHandler{
		notify: make(chan *notifEvent),
		close:  make(chan uint32),
	}
}
