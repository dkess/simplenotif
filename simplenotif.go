package main

import (
	"fmt"
	"github.com/godbus/dbus"
	"os"
	"time"
)

func (f *eventHandler) GetCapabilities() ([]string, *dbus.Error) {
	return []string{"actions", "body", "persistence"}, nil
}

func (eh *eventHandler) Notify(app_name string, replaces_id uint32, app_icon string, summary string, body string, actions []string, hints map[string]dbus.Variant, expire_timeout int32) (uint32, *dbus.Error) {
	getId := make(chan uint32, 1)
	eh.notify <- &notifEvent{
		app_name:    app_name,
		replaces_id: replaces_id,
		app_icon:    app_icon,
		text: notiftext{
			time:    time.Now(),
			summary: summary,
			body:    body,
		},
		actions:        actions,
		expire_timeout: expire_timeout,
		id:             getId,
	}

	// this should return the ID of the notification
	return <-getId, nil
}

func (eh *eventHandler) CloseNotification(id uint32) *dbus.Error {
	return nil
}

func (eh *eventHandler) GetServerInformation() (string, string, string, string, *dbus.Error) {
	return "simplenotif", "https://dkess.me", "0.0.0", "1", nil
}

func main() {
	conn, err := dbus.SessionBus()
	if err != nil {
		panic(err)
	}

	reply, err := conn.RequestName("org.freedesktop.Notifications",
		dbus.NameFlagDoNotQueue)
	if err != nil {
		panic(err)
	}
	if reply != dbus.RequestNameReplyPrimaryOwner {
		fmt.Fprintln(os.Stderr, "name already taken")
		os.Exit(1)
	}

	eh := NewEventHandler()
	conn.Export(eh, "/org/freedesktop/Notifications",
		"org.freedesktop.Notifications")
	if err != nil {
		panic(err)
	}

	newsub := make(chan chan string)
	delsub := make(chan chan string)
	statuschange := make(chan string)

	go WatchSubscribers(newsub, delsub, statuschange)

	go StartServer(newsub, delsub)

	WatchEvents(eh, statuschange)
}
