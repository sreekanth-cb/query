//  Copyright (c) 2016 Couchbase, Inc.
//  Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file
//  except in compliance with the License. You may obtain a copy of the License at
//    http://www.apache.org/licenses/LICENSE-2.0
//  Unless required by applicable law or agreed to in writing, software distributed under the
//  License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
//  either express or implied. See the License for the specific language governing permissions
//  and limitations under the License.

/*
GenCache provides a highly concurrent, resizable set of structures, implemented as an
an array of doubly linked lists for sequential access complemented by maps for direct
access.

The ForEach method is not meant to provide a snapshot of the current state of affairs
but rather an almost accurate picture: deletes and inserts are allowed as the scan
takes place.

Since the cache will be maintained by LRU purging, and certain types of access to the cache
will move elements at the top of the bucket, we do maintain two lists: LRU (for cleaning)
and scan (for access): a single list for both operations proved to be inadequate in avoiding
skipping whole swathes of entries or reporting an element twice, caused by entries moving
about in the bucket as the scan occurs.
*/
package util

import (
	"sync"

	atomic "github.com/couchbase/go-couchbase/platform"
)

type genSubList struct {
	next *genElem // next points to head (list), goes in the direction of head (element)
	prev *genElem // prev points to tail (list), goes in the direction of tail (element)
}

type listType int

const (
	_LRU listType = iota
	_SCAN
	_LISTTYPES // Sizer
)

type genElem struct {
	ID       string
	lists    [_LISTTYPES]genSubList
	refcount int32
	deleted  bool
	contents interface{}
}

const _CACHE_SIZE = 1 << 10
const _CACHES = 4

type Operation int

const (
	IGNORE Operation = iota
	AMEND
	REPLACE
)

type GenCache struct {

	// one lock per cache bucket to aid concurrency
	locks [_CACHES]sync.RWMutex

	// this shows intent to lock exclusively
	lockers [_CACHES]int32

	// doubly linked lists for scans and ejections
	lists [_CACHES][_LISTTYPES]genSubList

	// maps for direct access
	maps [_CACHES]map[string]*genElem

	// max size, for LRU lists
	limit   int
	curSize int32
}

func NewGenCache(l int) *GenCache {
	rv := &GenCache{
		limit: l,
	}

	for b := 0; b < _CACHES; b++ {
		rv.maps[b] = make(map[string]*genElem, _CACHE_SIZE)
	}
	return rv
}

// Add (or update, if ID found) entry, eject old entry if we are controlling sie
func (this *GenCache) Add(entry interface{}, id string, process func(interface{}) Operation) {
	cacheNum := HashString(id, _CACHES)
	this.lock(cacheNum)

	elem, ok := this.maps[cacheNum][id]
	if ok {
		op := REPLACE

		// If the element has been found, process the existing entry,
		// determine any conflict, and skip if required
		// The process function may alter the entry contents as
		// required rather than switching it to the new entry
		if process != nil {
			if op = process(elem.contents); op == IGNORE {
				return
			}
		}

		// Move to the front
		this.promote(elem, cacheNum)

		if op == REPLACE {
			elem.contents = entry
		}

		this.locks[cacheNum].Unlock()

	} else {
		ditchOther := false

		// In order not to have to acquire a different lock
		// we try to ditch the LRU entry from this hash node:
		// it makes the list a bit lopsided at the lower end
		// but it buys us performance
		elem = this.lists[cacheNum][_LRU].prev
		if this.limit > 0 && int(this.curSize) >= this.limit {
			if elem != nil {
				this.remove(elem, cacheNum)
			} else {

				// if we had nothing locally, we'll drop
				// an entry from another bucket once we
				// have unlocked this one
				ditchOther = true
			}
		} else {
			atomic.AddInt32(&this.curSize, 1)
		}
		elem = &genElem{
			contents: entry,
			ID:       id,
		}
		this.add(elem, cacheNum)
		this.maps[cacheNum][id] = elem
		this.locks[cacheNum].Unlock()

		// we needed to limit the cache, but our bucket was empty,
		// so we need to find a sacrificial victim somewhere else
		// we choose the one with the highest number of entries
		// for efficiency, we are a bit liberal with locks
		if ditchOther {
			count := 0
			newCacheNum := -1

			for c := 0; c < _CACHES; c++ {
				l := len(this.maps[c])
				if l > count {
					count = l
					newCacheNum = c
				}
			}

			if newCacheNum != -1 {
				this.lock(newCacheNum)
				elem = this.lists[newCacheNum][_LRU].prev
				if elem != nil {
					this.remove(elem, newCacheNum)
					ditchOther = false
				}
				this.locks[newCacheNum].Unlock()
			}

			// after all this, we still didn't find another victim
			// (not even ourselves!), so we need to adjust the count,
			// as it's off by 1
			if ditchOther {
				atomic.AddInt32(&this.curSize, 1)
			}
		}
	}
}

// Remove entry
func (this *GenCache) Delete(id string, cleanup func(interface{})) bool {
	cacheNum := HashString(id, _CACHES)
	this.lock(cacheNum)
	defer this.locks[cacheNum].Unlock()

	elem, ok := this.maps[cacheNum][id]
	if ok {
		if cleanup != nil {
			cleanup(elem.contents)
		}
		this.remove(elem, cacheNum)
		atomic.AddInt32(&this.curSize, -1)
		return true
	}
	return false
}

// Returns an element's contents by id
func (this *GenCache) Get(id string, process func(interface{})) interface{} {
	cacheNum := HashString(id, _CACHES)
	this.locks[cacheNum].RLock()
	defer this.locks[cacheNum].RUnlock()
	elem, ok := this.maps[cacheNum][id]
	if !ok {
		return nil
	} else {
		if process != nil {
			process(elem.contents)
		}
		return elem.contents
	}
}

// Returns an element's contents by id and places it at the top of the bucket
// Also useful to manipulate an element with an exclusive lock
func (this *GenCache) Use(id string, process func(interface{})) interface{} {
	cacheNum := HashString(id, _CACHES)
	this.lock(cacheNum)
	defer this.locks[cacheNum].Unlock()
	elem, ok := this.maps[cacheNum][id]
	if !ok {
		return nil
	} else {

		// Move to the front
		this.promote(elem, cacheNum)

		if process != nil {
			process(elem.contents)
		}
		return elem.contents
	}
}

// List Size
func (this *GenCache) Size() int {
	return int(this.curSize)
}

// LRU cleanup limit
func (this *GenCache) Limit() int {
	return this.limit
}

// Set the list limit
func (this *GenCache) SetLimit(limit int) {

	// this we ought to do with a lock, however
	// we only envisage one request to change the limit
	// every blue moon and it's only Add that's using it
	// to keep the list compact: in the worse case we
	// skip ditching entries, which is done here anyhow...
	this.limit = limit

	// reign in entries a bit
	c := 0
	for this.limit > 0 && int(this.curSize) > this.limit {
		this.lock(c)
		elem := this.lists[c][_LRU].prev
		if elem != nil {
			this.remove(elem, c)
			atomic.AddInt32(&this.curSize, -1)
		}
		this.locks[c].Unlock()
		c = (c + 1) % _CACHES
	}
}

// Return a slice with all the entry id's
func (this *GenCache) Names() []string {
	i := 0

	// we have emergency extra space not to have to append
	// if we can avoid it
	l := int(this.curSize)
	sz := _CACHES + l
	n := make([]string, l, sz)
	this.ForEach(func(id string, entry interface{}) bool {
		if i < l {
			n[i] = id
		} else {
			n = append(n, id)
		}
		i++
		return true
	}, nil)
	return n
}

// Scan the list
//
// As noted in the starting comments, this is not a consistent snapshot
// but rather a a low cost, almost accurate view
//
// For each element, we cater for actions with the bucket locked (must be non blocking)
// and blocking actions with the bucket available
// Since, for blocking operations, the entry is not guaranteed to exist, any data needed by them
// must be set up in the non blocking part
// both functions should return false if processing needs to stop
func (this *GenCache) ForEach(nonBlocking func(string, interface{}) bool,
	blocking func() bool) {
	cont := true

	for b := 0; b < _CACHES; b++ {
		sharedLock := true
		this.locks[b].RLock()
		nextElem := this.lists[b][_SCAN].prev
		if nextElem == nil {
			this.locks[b].RUnlock()
			continue
		}

		// mark tail element as in use, so that they don't disappear mid scan
		atomic.AddInt32(&nextElem.refcount, 1)
		for {
			elem := nextElem
			nextElem = elem.lists[_SCAN].next

			// mark next element as in use so that it doesn't get removed from
			// the list and we get lost mid scan...
			if nextElem != nil {
				atomic.AddInt32(&nextElem.refcount, 1)
			}

			// somebody had deleted the element  in the interim, so skip it
			if elem.deleted {

				// and if no longer referenced, get rid of it for real
				if elem.refcount == 1 {

					// promote the lock
					this.locks[b].Unlock()
					sharedLock = false
					this.lock(b)

					// if we are still the only referencer, remove
					if elem.refcount == 1 {
						this.lists[b][_SCAN].ditch(elem, _SCAN)
					}
				}

			} else {

				// perform the non blocking action
				if nonBlocking != nil {
					cont = nonBlocking(elem.ID, elem.contents)
				}
			}

			// release current element
			atomic.AddInt32(&elem.refcount, -1)

			// unlock the cache
			if sharedLock {

				// if we don't have waiters or blocking actions we can just continue
				if nextElem != nil && cont && blocking == nil && this.lockers[b] == 0 {
					continue
				}
				this.locks[b].RUnlock()
			} else {
				this.locks[b].Unlock()
			}

			// peform the blocking action
			if cont && !elem.deleted && blocking != nil {
				cont = blocking()
			}

			// things went wrong, or got settled early
			if !cont {
				return
			}

			// end of this bucket, onto the next
			if nextElem == nil {
				break
			}

			// restart the scan
			this.locks[b].RLock()
			sharedLock = true
		}
	}
}

func (this *GenCache) lock(cacheNum int) {
	atomic.AddInt32(&this.lockers[cacheNum], 1)
	this.locks[cacheNum].Lock()
	atomic.AddInt32(&this.lockers[cacheNum], -1)
}

// in all of the following methods, the bucket is expected to be already exclusively locked
func (this *GenCache) add(elem *genElem, cacheNum int) {
	this.lists[cacheNum][_LRU].insert(elem, _LRU)
	this.lists[cacheNum][_SCAN].insert(elem, _SCAN)
}

func (this *GenCache) promote(elem *genElem, cacheNum int) {
	this.lists[cacheNum][_LRU].ditch(elem, _LRU)
	this.lists[cacheNum][_LRU].insert(elem, _LRU)
}

func (this *GenCache) remove(elem *genElem, cacheNum int) {
	delete(this.maps[cacheNum], elem.ID)
	this.lists[cacheNum][_LRU].ditch(elem, _LRU)
	if elem.refcount > 0 {
		elem.deleted = true
	} else {
		this.lists[cacheNum][_SCAN].ditch(elem, _SCAN)
	}
}

func (this *genSubList) insert(elem *genElem, list listType) {
	elem.lists[list].next = nil
	if this.next == nil {
		this.next = elem
		this.prev = elem
		elem.lists[list].prev = nil
	} else {
		elem.lists[list].prev = this.next
		elem.lists[list].prev.lists[list].next = elem
		this.next = elem
	}

}

func (this *genSubList) ditch(elem *genElem, list listType) {

	// corner cases: head
	if elem == this.next {
		this.next = elem.lists[list].prev

		// ...and tail
		if elem == this.prev {
			this.prev = elem.lists[list].next
		} else {
			elem.lists[list].prev.lists[list].next = nil
		}

		// tail
	} else if elem == this.prev {
		this.prev = elem.lists[list].next
		elem.lists[list].next.lists[list].prev = nil

		// middle
	} else {
		prev := elem.lists[list].prev
		next := elem.lists[list].next
		prev.lists[list].next = next
		next.lists[list].prev = prev
	}

	// help the GC
	elem.lists[list].next = nil
	elem.lists[list].prev = nil
}
