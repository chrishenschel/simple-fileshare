package cleanup

import (
	"log"
	"time"

	"github.com/chrishenschel/simple-fileshare/internal/storage"
)

type Job struct {
	store    *storage.Store
	interval time.Duration
}

func New(store *storage.Store, interval time.Duration) *Job {
	if interval <= 0 {
		interval = time.Minute
	}
	return &Job{store: store, interval: interval}
}

func (j *Job) RunOnce() {
	due, err := j.store.DueIDs(time.Now())
	if err != nil {
		log.Printf("cleanup: list due: %v", err)
		return
	}
	for _, id := range due {
		if err := j.store.Delete(id); err != nil {
			log.Printf("cleanup: delete %s: %v", id, err)
			continue
		}
		log.Printf("cleanup: deleted expired share %s", id)
	}
}

func (j *Job) Start() {
	go func() {
		t := time.NewTicker(j.interval)
		defer t.Stop()
		j.RunOnce()
		for range t.C {
			j.RunOnce()
		}
	}()
}
