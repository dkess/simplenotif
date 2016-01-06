package main

func WatchSubscribers(newsub, delsub <-chan chan string, statuschange <-chan string) {
	subs := make([]chan<- string, 0)
	status := ""
	for {
		select {
		case sub := <-newsub:
			subs = append(subs, sub)
			sub <- status
		case sub := <-delsub:
			for n, s := range subs {
				if s == sub {
					l := len(subs)
					subs[n] = subs[l-1]
					subs = subs[:l-1]
					break
				}
			}
		case status = <-statuschange:
			for _, s := range subs {
				s <- status
			}
		}
	}
}
