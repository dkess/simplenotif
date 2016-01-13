package main

import (
	"testing"
	"time"
)

func makeTestNotif(nfs *nfState, timeoutchan <-chan uint16, id uint32, s, b string) (uint32, uint16) {
	getId := make(chan uint32)
	go func(g chan uint32) {
		nfs.HandleNotifEvent(&notifEvent{
			app_name:    "test",
			replaces_id: id,
			app_icon:    "",
			text: notiftext{
				time:    time.Now(),
				summary: s,
				body:    b,
			},
			actions:        []string{},
			expire_timeout: -1,
			id:             g,
		})
	}(getId)

	var ret uint32
	gotId := false
	var timeout uint16 = 0
	gotTimeout := false

	for !(gotId && (gotTimeout || timeoutchan == nil)) {
		select {
		case ret = <-getId:
			gotId = true
		case timeout = <-timeoutchan:
			gotTimeout = true
		}
	}

	select {
	case timeout = <-timeoutchan:
	default:
	}

	return ret, timeout
}

func TestNotifList(t *testing.T) {
	statuschange := make(chan string, 1000)

	nfs, timeouts := newNFState(statuschange)
	if nfs.notifList.Len() != 0 {
		t.Error("bad number of elements in notifList")
	}

	// Add the first notification
	id1, waitTime1 := makeTestNotif(nfs, timeouts, 0, "1", "0")
	t.Logf("n1 has id %f and timeout %f", id1, waitTime1)
	if id1 == 0 {
		t.Error("notif was assigned a zero id")
	}
	if waitTime1 <= 0 {
		t.Error("notif is not expiring properly")
	}

	if nfs.seeking_at >= 0 {
		t.Error("seeking when it's not supposed to!")
	}
	if nfs.notifList.Len() != 1 {
		t.Error("bad number of elements in notifList")
	}
	if nfs.notifList.Front() != nfs.currently_showing {
		t.Error("first notification was added incorrectly")
	}
	p1 := nfs.currently_showing.Value.(*notif)
	if p1.id != id1 {
		t.Error("n1 was not assigned the right id")
	}
	if len(p1.text) != 1 {
		t.Error("n1 text list has bad len")
	}
	if p1.text[0].summary != "1" {
		t.Error("n1 was given the wrong summary")
	}
	if !p1.seen_by_user {
		t.Error("n1 not being shown to uesr")
	}

	// Add the second notification
	id2, waitTime2 := makeTestNotif(nfs, nil, 0, "2", "0")
	t.Logf("n2 has id %f and timeout %f", id2, waitTime2)
	// second notification should not have returned a timeout, indicated by 0
	if waitTime2 != 0 {
		t.Error("n2 should not have timed out")
	}
	if id2 == 0 {
		t.Error("n2 was given zero id")
	}
	if id2 == id1 {
		t.Error("first 2 notifications were given the same id:", id1)
	}

	t.Logf("n2 has id %f and timeout %f", id2, waitTime2)

	if nfs.seeking_at >= 0 {
		t.Error("seeking when it's not supposed to!")
	}
	if nfs.notifList.Len() != 2 {
		t.Error("bad number of elements in notifList")
	}
	if nfs.notifList.Front() != nfs.currently_showing {
		t.Error("n2 messed up display of first")
	}
	if nfs.notifList.Front().Value.(*notif).id != id1 {
		t.Error("n1 got messed up after n2 was sent")
	}
	p2 := nfs.notifList.Back().Value.(*notif)
	if p2.id != id2 {
		t.Error("n2 was not assigned the right id")
	}
	if len(p2.text) != 1 {
		t.Error("n2 text list has bad len")
	}
	if p2.text[0].summary != "2" {
		t.Error("n2 was given the wrong summary")
	}
	if p2.seen_by_user {
		t.Error("n2 seen by user too soon")
	}

	// Update the second notification, which should not affect the currently
	// displayed notification
	id2_1, waitTime2_1 := makeTestNotif(nfs, nil, id2, "2", "1")
	if waitTime2_1 != 0 {
		t.Error("n2.1 should not have timed out")
	}
	if id2_1 != id2 {
		t.Errorf("n2.1 was given id %f but expected %f", id2_1, id2)
	}
	if nfs.notifList.Len() != 2 {
		t.Error("bad number of elements in notifList")
	}
	if nfs.currently_showing != nfs.notifList.Front() {
		t.Error("currently_showing was changed after an update to a different notif")
	}
	if nfs.currently_showing.Value.(*notif).id != id1 {
		t.Error("updating a notification messed up the id of currently_showing")
	}
	if len(nfs.notifList.Back().Value.(*notif).text) != 2 {
		t.Error("update to n2 was not applied correctly")
	}

	// Update the first notification, which should affect the display
	id1_1, waitTime1_1 := makeTestNotif(nfs, timeouts, id1, "1", "1")
	if id1_1 != id1 {
		t.Errorf("n1.1 was given id %f but expected %f", id1_1, id1)
	}
	if waitTime1_1 == 0 {
		t.Error("n1.1 should have reset the timer but did not")
	}
	if nfs.notifList.Len() != 2 {
		t.Error("bad number of elements in notifList")
	}
	// after the update, currently_showing should move to the back
	if nfs.currently_showing == nfs.notifList.Front() {
		t.Error("currently_showing points to the wrong notif")
	}
	if nfs.currently_showing.Value.(*notif).id != id1 {
		t.Error("updating a notification messed up the id of currently_showing")
	}
	if len(nfs.notifList.Front().Value.(*notif).text) != 2 {
		t.Error("update to n1 was not applied correctly")
	}
	if nfs.seeking_at != -1 {
		t.Error("seeking at the wrong time!")
	}

	// Timeouts expire, statusline should shift to now show n2
	// We should expect a new timeout counter
	go func(timeouts <-chan uint16) {
		select {
		case waitTime2_2 := <-timeouts:
			if waitTime2_2 == 0 {
				t.Error("n2's timeout was 0 when it should be >0")
			}
		case <-time.After(time.Second):
			t.Error("n2 never sent a timeout")
		}
	}(timeouts)

	nfs.nextStatus(true)

	if nfs.currently_showing == nfs.notifList.Back() {
		t.Error("currently_showing points to the wrong notif")
	}
	if nfs.notifList.Len() != 2 {
		t.Error("bad number of elements in notifList")
	}
	if nfs.currently_showing.Value.(*notif).id != id2 {
		t.Errorf("currently_showing has id %f but expected n2.id %f",
			nfs.currently_showing.Value.(*notif).id, id2)
	}
	if nfs.seeking_at != -1 {
		t.Error("seeking at the wrong time!")
	}

	// test seeking: first, PrevMsg and PrevNotif, which should both do nothing
	// in this case because n2 is at the front of the list
	nfs.SeekPrevMsg()

	// same tests as above, because nothing should have changed
	if nfs.currently_showing == nfs.notifList.Back() {
		t.Error("currently_showing points to the wrong notif")
	}
	if nfs.notifList.Len() != 2 {
		t.Error("bad number of elements in notifList")
	}
	if nfs.currently_showing.Value.(*notif).id != id2 {
		t.Errorf("currently_showing has id %f but expected n2.id %f",
			nfs.currently_showing.Value.(*notif).id, id2)
	}
	if nfs.seeking_at != -1 {
		t.Error("seeking at the wrong time!")
	}

	// add a new notification, n3
	id3, waitTime3 := makeTestNotif(nfs, nil, 0, "3", "0")
	if id3 == 0 || id3 == id1 || id3 == id2 {
		t.Error("n3 was given bad id ", id3)
	}
	if waitTime3 != 0 {
		t.Error("n3 was given a timeout when it shouldn't have")
	}
	if nfs.notifList.Len() != 3 {
		t.Errorf("bad number of elements in notifList: expected %f; got %f", 3, nfs.notifList.Len())
	}
	if nfs.currently_showing != nfs.notifList.Front() {
		t.Error("currently_showing should be pointing to the front of list")
	}
	if nfs.currently_showing.Value.(*notif).id != id2 {
		t.Errorf("currently_showing has id %f but expected n2.id %f",
			nfs.currently_showing.Value.(*notif).id, id2)
	}
	if nfs.seeking_at != -1 {
		t.Error("seeking at the wrong time!")
	}

	//
}
