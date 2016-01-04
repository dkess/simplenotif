package main

import (
	"fmt"
	"time"
)

type notiftext struct {
	time    time.Time
	summary string
	body    string
}

type notifEvent struct {
	app_name       string
	replaces_id    uint32
	app_icon       string
	text           notiftext
	actions        []string
	expire_timeout int32
	id             chan uint32
}

type notif struct {
	id             uint32
	app_name       string
	app_icon       string
	text           []notiftext
	actions        []string
	expire_timeout int32
}

func WatchEvents(eh *eventHandler) {
	// used to assign IDs to new notifications
	var notif_counter uint32 = 1
	notifications := make([]*notif, 0)
	for {
		select {
		case n := <-eh.notify:
			fmt.Println("Got notification", n)
			id := n.replaces_id

			// If addNewNotif is true, it means we have to add a new entry
			// to our list of notifications, (instead of replacing an old one)
			addNewNotif := false

			if id != 0 {
				addNewNotif = true
				for _, p := range notifications {
					if p.id == id {
						// replace this notification with new properties, and
						// append the new text
						p.app_name = n.app_name
						p.app_icon = n.app_icon
						p.text = append(p.text, n.text)
						p.actions = n.actions
						p.expire_timeout = n.expire_timeout
						addNewNotif = false
						break
					}
				}
			} else {
				// Generate a new notification ID based on the counter, but
				// make sure it hasn't been stolen by a naughty program that
				// sets its own IDs and took the ID of the current count.
				addNewNotif = true
			Outer:
				for {
					for _, p := range notifications {
						if p.id == notif_counter {
							notif_counter++
							continue Outer
						}
					}
					break
				}
				id = notif_counter
				notif_counter++
			}

			// Add this new notification
			if addNewNotif {
				notifications = append(notifications, &notif{
					id:             id,
					app_name:       n.app_name,
					text:           []notiftext{n.text},
					actions:        n.actions,
					expire_timeout: n.expire_timeout,
				})
			}
			// Tell dbus what ID we chose for this notification
			n.id <- id
			for i, p := range notifications {
				fmt.Println(i, p)
			}

		case c := <-eh.close:
			fmt.Println("Close notification", c)
		}
	}
}
