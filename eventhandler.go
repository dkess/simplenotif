package main

/*
import (
	"fmt"
)
*/

type notif_id uint32

type notif struct {
	app_name       string
	id             notif_id
	app_icon       string
	summary        string
	body           string
	actions        []string
	expire_timeout int32
}

type eventHandler struct {
	notify chan *notif
	close  chan notif_id
}

func NewEventHandler() *eventHandler {
	return &eventHandler{
		notify: make(chan *notif),
		close:  make(chan notif_id),
	}
}
