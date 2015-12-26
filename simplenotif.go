package main

import (
	"fmt"
	"github.com/godbus/dbus"
	"os"
)

type foo string

func (f *eventHandler) GetCapabilities() ([]string, *dbus.Error) {
	return []string{"actions", "body", "persistence"}, nil
}

func (eh *eventHandler) Notify(app_name string, replaces_id uint32, app_icon string, summary string, body string, actions []string, hints map[string]dbus.Variant, expire_timeout int32) (uint32, *dbus.Error) {
	fmt.Println("Got notification: ", summary)
	return replaces_id, nil
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
	conn.Export(eh, "/org/freedesktop/Notifications", "org.freedesktop.Notifications")
	if err != nil {
		panic(err)
	}

	fmt.Println("listening")

	select {}
}
