package proxy

import (
	"bytes"
	"context"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

const (
	MaxNotifyBuffer = 1000
)

type NotifyManager struct {
	ctx     context.Context
	webhook string
	slack   string

	C chan string
}

func NewNotifyManager(ctx context.Context, an *AdminNotify) *NotifyManager {
	nm := &NotifyManager{
		ctx:     ctx,
		webhook: an.Webhook,
		slack:   an.Slack,
		C:       make(chan string, MaxNotifyBuffer),
	}

	go nm.run()

	return nm
}

func (nm *NotifyManager) run() {
	for {
		select {
		case <-nm.ctx.Done():
			return

		case m := <-nm.C:
			log.Printf("[NotifyManager] msg=%s\n", m)
			if nm.webhook != "" {
				cl := &http.Client{Timeout: 500 * time.Millisecond}
				res, err := cl.Post(nm.webhook, "application/json", bytes.NewBufferString(m))
				if err != nil {
					log.Printf("[NotifyManager] err=%s msg=%s\n", err, m)
				} else {
					bs, err := ioutil.ReadAll(res.Body)
					if err != nil {
						log.Printf("[NotifyManager/response] err=%s\n", err)
					} else {
						log.Printf("[NotifyManager/response] content=%s\n", bs)
					}
				}
			}
		}
	}
}
