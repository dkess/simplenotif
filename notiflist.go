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
			if waitTime > 0 {
			Inner:
				for {
					select {
					case waitTime = <-timeouts:
						if waitTime == 0 {
							break Inner
						}
					case <-time.After(time.Second * time.Duration(waitTime)):
						nextNotif <- true
						break Inner
					}
				}
			}
		}
	}
}

func (n *notif) displayString() string {
	lastLine := n.text[len(n.text)-1]
	return lastLine.summary + " | " + lastLine.body
}

type nfState struct {
	// If a notification's expiration should be canceled, 0 is passed through
	// this channel.  If an expiration should be extended (because the
	// notification is being replaced), the length of the extention is passed.
	timeouts chan<- uint16

	// The channel through which statusline updates are sent.
	statuschange chan<- string

	// An incrementing counter that holds the id to be assigned to the next
	// notification.
	notif_counter uint32

	// The notification currently being displayed to the user on the statusline,
	// or nil if no notifications are being shown.
	currently_showing *list.Element

	// If the user is seeking through past notifications, this is the index of
	// notif.text of the notification being displayed.  If the user is not
	// seeking, this is -1. If currently_showing is nil, this must be -1.
	seeking_at int

	//  The list of notifications.
	// We use a list instead of a slice because container/list gives functions
	// very specific to this problem domain.  When a new notification replaces
	// an old one, List.MoveToBack() is used.  List traversal is easily
	// accomplished by saving the position in currently_showing.
	// The back of the list always contains the most recent notification. All
	// elements are of type *notif.
	notifList *list.List
}

func newNFState(statuschange chan<- string) (
	*nfState,
	<-chan uint16) {

	timeouts := make(chan uint16)

	return &nfState{
		timeouts:          timeouts,
		statuschange:      statuschange,
		notif_counter:     1,
		currently_showing: nil,
		seeking_at:        -1,
		notifList:         list.New(),
	}, timeouts
}

func (s *nfState) updateStatus() {
	_ = "breakpoint"
	if s.currently_showing == nil {
		s.statuschange <- ""
		return
	}
	p := s.currently_showing.Value.(*notif)
	var l string
	var on_msg notiftext
	if s.seeking_at < 0 {
		on_msg = p.text[len(p.text)-1]
		l = ""
	} else {
		on_msg = p.text[s.seeking_at]
		l = "(" + Round(time.Since(on_msg.time), time.Second).String() + " ago) "
	}

	l += on_msg.summary + " | " + on_msg.body
	s.statuschange <- l
}

func (s *nfState) nextStatus(isNewNotif bool) {
	// A new notification should not interrupt seeking, unless seeking
	// the seeking is over (that is, currently_showing == nil)
	if s.seeking_at >= 0 && s.currently_showing != nil {
		return
	}
	s.seeking_at = -1

	// After the following loop is over, this variable will be filled
	// with the most recent permanent notification, if one exists.
	var permanentNotif *notif = nil

	// After the following loop is over, this variable will be true if
	// there are no more notifications to show (other than permanent
	// ones).
	nothingToShow := true

	for e := s.notifList.Front(); e != nil; e = e.Next() {
		p := e.Value.(*notif)
		if (!isNewNotif && e == s.currently_showing) ||
			(isNewNotif && !p.seen_by_user) {

			s.currently_showing = e

			if p.expire_timeout == 0 {
				if !p.seen_by_user {
					permanentNotif = p
				}
			} else {
				p.seen_by_user = true
				if p.expire_timeout < 0 {
					// TODO: replace this with a user-chosen value, or perhaps
					// make it based on notification urgency
					s.timeouts <- 15
				} else {
					s.timeouts <- uint16(p.expire_timeout)
				}

				s.updateStatus()
				nothingToShow = false
				break
			}
		}
	}

	if nothingToShow {
		if permanentNotif == nil {
			s.currently_showing = nil
		}
		s.updateStatus()
	}
}

func (s *nfState) HandleNotifEvent(n *notifEvent) {
	id := n.replaces_id

	// If addNewNotif becomes true, it means a new entry must be added to the
	// list of notifications, (instead of appending content to an already
	// existing one).
	addNewNotif := false

	if id != 0 {
		addNewNotif = true
		// Check if a notification with this id already exists in the
		// list.
		for e := s.notifList.Back(); e != nil; e = e.Prev() {
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

				if e == s.currently_showing {
					s.nextStatus(false)
				} else if s.currently_showing == nil ||
					s.currently_showing.Value.(*notif).expire_timeout == 0 {

					s.nextStatus(true)
				}

				s.notifList.MoveToBack(e)
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
			for e := s.notifList.Front(); e != nil; e = e.Next() {
				p := e.Value.(*notif)
				if p.id == s.notif_counter {
					s.notif_counter++
					continue Outer
				}
			}
			break
		}
		id = s.notif_counter
		s.notif_counter++
	}

	// Add a new notification to the list
	if addNewNotif {
		s.notifList.PushBack(&notif{
			id:             id,
			app_name:       n.app_name,
			text:           []notiftext{n.text},
			actions:        n.actions,
			expire_timeout: n.expire_timeout,
		})

		// The statusline should only be updated if it's not showing anything
		// currently (because if it is, the new content will eventually be shown
		// after timeouts expire).  But if a permanent notification is being
		// shown, the new notification should be displayed now because it will
		// never timeout.
		if s.currently_showing == nil ||
			s.currently_showing.Value.(*notif).expire_timeout == 0 {

			s.nextStatus(true)
		}
	}
	// Tell dbus what ID we chose for this notification
	n.id <- id
}

func (s *nfState) HideNotif(id uint32) {
	var toHide *notif = nil

	if s.currently_showing.Value.(*notif).id == id {
		toHide = s.currently_showing.Value.(*notif)

		if s.seeking_at <= 0 {
			s.timeouts <- 0
			defer s.nextStatus(true)
		}
	} else {
		for e := s.notifList.Front(); e != nil; e = e.Next() {
			if e.Value.(*notif).id == id {
				toHide = e.Value.(*notif)
				break
			}
		}
	}

	toHide.seen_by_user = true
}

func (s *nfState) HideAllNotifs() {
	if s.currently_showing != nil {
		defer s.updateStatus()
	}

	for e := s.notifList.Front(); e != nil; e = e.Next() {
		e.Value.(*notif).seen_by_user = true
	}

	s.timeouts <- 0
	s.seeking_at = -1
	s.currently_showing = nil
}

func (s *nfState) DismissCurrent() {
	if s.currently_showing == nil {
		return
	}
	if s.seeking_at <= 0 {
		s.timeouts <- 0
		defer s.updateStatus()
	}

	toRemove := s.currently_showing
	s.currently_showing = s.currently_showing.Next()
	s.notifList.Remove(toRemove)

	if s.seeking_at > 0 {
		if s.currently_showing != nil {
			s.seeking_at = len(s.currently_showing.Value.(*notif).text) - 1
			s.updateStatus()
		} else {
			s.seeking_at = -1
			s.nextStatus(true)
		}
	}
}

func (s *nfState) DismissAll() {
	s.notifList = list.New()
	s.seeking_at = -1
	s.timeouts <- 0
	s.nextStatus(true)
}

func (s *nfState) SeekNextMsg() {
	if s.currently_showing == nil {
		return
	}
	p := s.currently_showing.Value.(*notif)
	if s.seeking_at == len(p.text)-1 || s.seeking_at < 0 {
		s.currently_showing = s.currently_showing.Next()
		if s.currently_showing == nil {
			s.seeking_at = -1
			s.nextStatus(true)
		} else {
			s.seeking_at = 0
			s.updateStatus()
		}
	} else if s.seeking_at >= 0 {
		s.seeking_at++
		s.updateStatus()
	}
}

func (s *nfState) SeekPrevMsg() {
	if s.seeking_at < 0 {
		s.timeouts <- 0
		if s.currently_showing != nil {
			s.seeking_at = len(s.currently_showing.Value.(*notif).text) - 1
		} else {
			s.currently_showing = s.notifList.Back()
			if s.currently_showing != nil {
				s.seeking_at = len(s.currently_showing.Value.(*notif).text) - 1
				s.updateStatus()
			}
			return
		}
	}
	if s.seeking_at > 0 {
		s.seeking_at--
		s.updateStatus()
	} else if s.seeking_at == 0 {
		if s.currently_showing.Prev() != nil {
			s.currently_showing = s.currently_showing.Prev()
			p := s.currently_showing.Value.(*notif)
			s.seeking_at = len(p.text) - 1
			s.updateStatus()
		}
	}
}

func (s *nfState) SeekNextNotif() {
	if s.currently_showing != nil {
		s.currently_showing = s.currently_showing.Next()
		if s.currently_showing != nil {
			p := s.currently_showing.Value.(*notif)
			s.seeking_at = len(p.text) - 1
			s.updateStatus()
		} else {
			s.seeking_at = -1
			s.nextStatus(true)
		}
	}
}

func (s *nfState) SeekPrevNotif() {
	if s.currently_showing == nil {
		s.currently_showing = s.notifList.Back()
		if s.currently_showing == nil {
			return
		}
	} else {
		if s.currently_showing.Prev() == nil {
			return
		} else {
			s.currently_showing = s.currently_showing.Prev()
		}
	}
	p := s.currently_showing.Value.(*notif)
	s.seeking_at = len(p.text) - 1
	s.updateStatus()
}

func WatchEvents(eh *eventHandler, statuschange chan<- string,
	remote <-chan RemoteButton) {

	nfs, timeouts := newNFState(statuschange)
	nextNotif := make(chan bool)
	go notifExpireTimer(timeouts, nextNotif)

	for {
		select {
		case n := <-eh.notify:
			fmt.Println("Got notification", n)
			nfs.HandleNotifEvent(n)

		case c := <-eh.close:
			nfs.HideNotif(c)

		case isNewNotif := <-nextNotif:
			nfs.nextStatus(isNewNotif)

		case button := <-remote:
			if button == Hide {
				nfs.HideNotif(nfs.currently_showing.Value.(*notif).id)
			} else if button == HideAll {
				nfs.HideAllNotifs()
			} else if button == Dismiss {
				nfs.DismissCurrent()
			} else if button == DismissAll {
				nfs.DismissAll()
			} else if button == NextMsg {
				nfs.SeekNextMsg()
			} else if button == PrevMsg {
				nfs.SeekPrevMsg()
			} else if button == NextNotif {
				nfs.SeekNextNotif()
			} else if button == PrevNotif {
				nfs.SeekPrevNotif()
			}
		}
	}
}
