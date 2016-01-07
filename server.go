package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
)

type RemoteButton string

const (
	NextMsg    RemoteButton = "nextmsg"
	NextNotif               = "nextnotif"
	PrevMsg                 = "prevmsg"
	PrevNotif               = "prevnotif"
	Dismiss                 = "dismiss"
	DismissAll              = "dismissall"
	Hide                    = "hide"
	HideAll                 = "hideall"
)

func handleClient(conn net.Conn, remote chan<- RemoteButton,
	newsub, delsub chan<- chan string) {

	defer conn.Close()

	ch := make(chan string)
	eCh := make(chan error)
	statusline := make(chan string)

	go func(ch chan<- string, eCh chan<- error) {
		r := bufio.NewReader(conn)
		for {
			line, err := r.ReadString('\n')
			ch <- strings.TrimSpace(line)
			if err != nil {
				eCh <- err
				return
			}
		}
	}(ch, eCh)

	for {
		select {
		case line := <-ch:
			fmt.Println("<-", line)
			if line == "sub" {
				newsub <- statusline
				defer func() {
					delsub <- statusline
				}()
			} else if line != "" {
				remote <- RemoteButton(line)
			}
		case status := <-statusline:
			io.WriteString(conn, status+"\n")
		case _ = <-eCh:
			return
		}
	}
}

func StartServer(remote chan<- RemoteButton, newsub, delsub chan<- chan string) {
	ln, err := net.Listen("tcp", ":8082")
	if err != nil {
		panic(err)
	}

	for {
		conn, err := ln.Accept()
		if err != nil {
			fmt.Fprintln(os.Stderr, "error when a client connected")
		} else {
			go handleClient(conn, remote, newsub, delsub)
		}
	}
}
