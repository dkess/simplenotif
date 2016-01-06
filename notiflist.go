package main

import (
	"container/list"
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
	seen_by_user   bool
}

func notifExpireTimer(timeouts <-chan uint16, nextNotif chan<- bool) {
	for {
		select {
		case waitTime := <-timeouts:
			for {
				select {
				case waitTime = <-timeouts:
				case <-time.After(time.Second * time.Duration(waitTime)):
					nextNotif <- true
					break
				}
			}
		}
	}
}

func (n *notif) displayString() string {
	lastLine := n.text[len(n.text)-1]
	return lastLine.summary + " | " + lastLine.body
}

func WatchEvents(eh *eventHandler, statuschange chan<- string) {
	// used to assign IDs to new notifications
	var notif_counter uint32 = 1

	// When the user is not "seeking" through past notifications,
	// currently_showing is the id of the notification currently being
	// displayed on the statusline.
	var currently_showing uint32 = 0

	timeouts := make(chan uint16)

	// If true is sent, this notification has expired.  If false is sent, that
	// means the notification being displayed has had its text replaced.
	nextNotif := make(chan bool, 3)
	go notifExpireTimer(timeouts, nextNotif)

	// We use a list instead of a slice because container/list gives functions
	// very specific to this problem domain.  When a new notification replaces
	// an old one, List.MoveToBack() is used.  List traversal is also very
	// common and necessary.
	// The back of the list always contains the most recent notification. All
	// elements are of type *notif.
	notifList := list.New()
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
				//for _, p := range notifications {
				for e := notifList.Back(); e != nil; e = e.Prev() {
					p := e.Value.(*notif)
					if p.id == id {
						// replace this notification with new properties, and
						// append the new text
						p.app_name = n.app_name
						p.app_icon = n.app_icon
						p.text = append(p.text, n.text)
						p.actions = n.actions
						p.expire_timeout = n.expire_timeout
						p.seen_by_user = false
						addNewNotif = false

						if p.id == currently_showing {
							nextNotif <- false
						}

						notifList.MoveToBack(e)
						break
					}
				}
			} else {
				// Generate a new notification ID based on the counter, but
				// make sure it isn't already being used by another
				// notification.  If it is, keep incrementing the counter until
				// an unused ID is found.
				addNewNotif = true
			Outer:
				for {
					for e := notifList.Front(); e != nil; e = e.Next() {
						p := e.Value.(*notif)
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
				notifList.PushBack(&notif{
					id:             id,
					app_name:       n.app_name,
					text:           []notiftext{n.text},
					actions:        n.actions,
					expire_timeout: n.expire_timeout,
				})
			}
			// Tell dbus what ID we chose for this notification
			n.id <- id
			for e := notifList.Front(); e != nil; e = e.Next() {
				fmt.Println(e.Value.(*notif))
			}

			if currently_showing == 0 {
				fmt.Println("pipng")
				nextNotif <- true
			}

		case c := <-eh.close:
			fmt.Println("Close notification", c)

		case isNewNotif := <-nextNotif:
			fmt.Println("next notification", isNewNotif)
			for e := notifList.Front(); e != nil; e = e.Next() {
				p := e.Value.(*notif)
				if (!isNewNotif && p.id == currently_showing) ||
					(isNewNotif && !p.seen_by_user) {

					currently_showing = p.id
					p.seen_by_user = true
					if p.expire_timeout < 0 {
						// TODO: replace this with some sort of user-chosen
						// default value, or do it based on notification
						// urgency
						timeouts <- 15
					} else {
						timeouts <- uint16(p.expire_timeout)
					}

					statuschange <- p.displayString()
					break
				}
			}
		}
	}
}
