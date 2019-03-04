package server

import (
	"github.com/Thoro/bfd/pkg/api"
	"github.com/eapache/channels"
)

type watcher struct {
	peer   *Peer
	realCh chan *api.PeerStateResponse
	ch     *channels.InfiniteChannel
}

func NewWatcher() *watcher {
	return &watcher{
		realCh: make(chan *api.PeerStateResponse, 8),
		ch:     channels.NewInfiniteChannel(),
	}
}

func (w *watcher) Notify(state *api.PeerStateResponse) {
	w.ch.In() <- state
}

func (w *watcher) Event() <-chan *api.PeerStateResponse {
	return w.realCh
}

func (w *watcher) loop() {
	for ev := range w.ch.Out() {
		w.realCh <- ev.(*api.PeerStateResponse)
	}

	close(w.realCh)
}

func (w *watcher) Stop() {
	w.peer.Lock()

	for idx, watcher := range w.peer.watchers {
		if watcher == w {
			a := w.peer.watchers
			a[idx] = a[len(a)-1]
			a[len(a)-1] = nil
			a = a[:len(a)-1]
			w.peer.watchers = a
			break
		}
	}

	w.peer.Unlock()

	w.ch.Close()

	for range w.ch.Out() {
	}

	for range w.realCh {
	}
}
