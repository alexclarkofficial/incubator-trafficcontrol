package health

/*
 * Licensed to the Apache Software Foundation (ASF) under one
 * or more contributor license agreements.  See the NOTICE file
 * distributed with this work for additional information
 * regarding copyright ownership.  The ASF licenses this file
 * to you under the Apache License, Version 2.0 (the
 * "License"); you may not use this file except in compliance
 * with the License.  You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import (
	"fmt"
	"sync"
	"time"

	"github.com/apache/incubator-trafficcontrol/traffic_monitor_golang/common/log"
)

type Time time.Time

func (t Time) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("%d", time.Time(t).Unix())), nil
}

// Event represents an event change in aggregated data. For example, a cache being marked as unavailable.
type Event struct {
	Time        Time   `json:"time"`
	Index       uint64 `json:"index"`
	Description string `json:"description"`
	Name        string `json:"name"`
	Hostname    string `json:"hostname"`
	Type        string `json:"type"`
	Available   bool   `json:"isAvailable"`
}

// Events provides safe access for multiple goroutines readers and a single writer to a stored Events slice.
type ThreadsafeEvents struct {
	events    *[]Event
	m         *sync.RWMutex
	nextIndex *uint64
	max       uint64
}

func copyEvents(a []Event) []Event {
	b := make([]Event, len(a), len(a))
	copy(b, a)
	return b
}

// NewEvents creates a new single-writer-multiple-reader Threadsafe object
func NewThreadsafeEvents(maxEvents uint64) ThreadsafeEvents {
	i := uint64(0)
	return ThreadsafeEvents{m: &sync.RWMutex{}, events: &[]Event{}, nextIndex: &i, max: maxEvents}
}

// Get returns the internal slice of Events for reading. This MUST NOT be modified. If modification is necessary, copy the slice.
func (o *ThreadsafeEvents) Get() []Event {
	o.m.RLock()
	defer o.m.RUnlock()
	return *o.events
}

// Add adds the given event. This is threadsafe for one writer, multiple readers. This MUST NOT be called by multiple threads, as it non-atomically fetches and adds.
func (o *ThreadsafeEvents) Add(e Event) {
	// host="hostname", type=EDGE, available=true, msg="REPORTED - available"
	log.Eventf(time.Time(e.Time), "host=\"%s\", type=%s, available=%t, msg=\"%s\"", e.Hostname, e.Type, e.Available, e.Description)
	o.m.Lock() // TODO test removing
	events := copyEvents(*o.events)
	e.Index = *o.nextIndex
	events = append([]Event{e}, events...)
	if len(events) > int(o.max) {
		events = (events)[:o.max-1]
	}
	// o.m.Lock()
	*o.events = events
	*o.nextIndex++
	o.m.Unlock()
}
