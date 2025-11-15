package cache

import (
	"container/list"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/indy9000/userstore/models"
)

var (
	ErrUserExists = errors.New("user already exists")
)

type UserCache[T models.BaseUserOps] struct {
	cache       map[string]*list.Element
	lru         *list.List
	mu          sync.RWMutex
	maxCapacity int      // Max cache size for eviction (0 = no limit)
	constructor func() T // Factory for new T instances
	baseFolder  string
}

// entry is a node in the LRU list, holding the key and value.
type entry[T models.BaseUserOps] struct {
	key   string
	value T
}

func NewUserCache[T models.BaseUserOps](baseFolder string, maxCapacity int, constructor func() T) *UserCache[T] {
	return &UserCache[T]{
		cache:       make(map[string]*list.Element, maxCapacity),
		maxCapacity: maxCapacity,
		lru:         list.New(),
		constructor: constructor,
		baseFolder:  baseFolder,
	}
}

func (c *UserCache[T]) loadFromDisk(userId string) (T, error) {
	fn := fmt.Sprintf("%s.json", userId)
	fp := filepath.Join(c.baseFolder, userId, fn)

	d, e := os.ReadFile(fp)
	if e != nil {
		if os.IsNotExist(e) {
			u := c.constructor()
			u.SetUserID(userId)
			u.SetLastUpdated()
			return u, nil
		}
		log.Printf("Failed to load data from file. uid:%s filepath:%s", userId, fp)
		var zero T
		return zero, e
	}

	u := c.constructor()
	e = json.Unmarshal(d, &u)
	if e != nil {
		log.Printf("Failed to unmarshal data from file. uid:%s filepath:%s", userId, fp)
		return u, e
	}
	return u, nil
}

func (c *UserCache[T]) saveToDisk(userId string, d T) error {
	fn := fmt.Sprintf("%s.json", userId)
	fp := filepath.Join(c.baseFolder, userId, fn)
	if err := os.MkdirAll(filepath.Dir(fp), 0750); err != nil {
		return err
	}
	bytes, e := json.Marshal(d)
	if e != nil {
		log.Printf("Failed to marshal data. uid:%s, filepath:%s", userId, fp)
		return e
	}
	tempPath := fp + ".tmp"
	if e = os.WriteFile(tempPath, bytes, 0640); e != nil {
		return e
	}
	return os.Rename(tempPath, fp)
}

// Get returns the cached user, loading it from disk (or creating a new record)
// if necessary. Callers must treat the returned pointer as read-only unless
// they go through Update or take the per-user lock themselves.
func (c *UserCache[T]) Get(userId string) (T, error) {
	// check the cache first
	c.mu.RLock()
	elem, ok := c.cache[userId]
	c.mu.RUnlock()
	if ok {
		c.mu.Lock()
		c.touch(elem) // update lru
		c.mu.Unlock()
		return elem.Value.(*entry[T]).value, nil
	}
	// not found in cache
	// Load from disk
	value, e := c.loadFromDisk(userId)
	if e != nil {
		// return empty value with error
		var zero T
		return zero, e
	}
	// got data from disk, add to the cache
	// And check again (while we are loading from disk, another thread may have
	// added to the cache)
	c.mu.Lock()
	defer c.mu.Unlock()
	elem, ok = c.cache[userId]
	if ok {
		// already aded
		c.touch(elem)
		return elem.Value.(*entry[T]).value, nil
	}

	// Truly missing: Add now
	ent := &entry[T]{key: userId, value: value}
	// add entry to lru
	elem = c.lru.PushFront(ent)
	c.cache[userId] = elem // set in the cache

	if c.maxCapacity > 0 && c.lru.Len() > c.maxCapacity {
		c.evict()
	}
	return value, nil
}

// Set creates a brand-new user in both cache and disk storage. It fails with
// ErrUserExists if the user already exists in memory or on disk.
func (c *UserCache[T]) Set(userId string, initializer func(T)) error {
	exists, e := c.exists(userId)
	if e != nil {
		return e
	}
	if exists {
		return ErrUserExists
	}

	// Create new
	value := c.constructor()
	value.SetUserID(userId)
	value.SetLastUpdated()
	initializer(value)

	// Lock and double-check (in case concurrent Set added)
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.cache[userId]; ok {
		return ErrUserExists
	}
	fn := fmt.Sprintf("%s.json", userId)
	fp := filepath.Join(c.baseFolder, userId, fn)
	if _, e := os.Stat(fp); e == nil {
		return ErrUserExists
	} else if !os.IsNotExist(e) {
		return e
	}

	// Save to disk
	if e := c.saveToDisk(userId, value); e != nil {
		return e
	}

	// Add to cache/LRU
	ent := &entry[T]{key: userId, value: value}
	elem := c.lru.PushFront(ent)
	c.cache[userId] = elem
	if c.maxCapacity > 0 && c.lru.Len() > c.maxCapacity {
		c.evict()
	}
	return nil
}

// Update locks the user, applies the mutation, refreshes lastUpdated, and
// writes the data back to disk before releasing the lock. All user mutations
// should flow through this method to avoid data races and guarantee durability.
func (c *UserCache[T]) Update(userId string, updater func(T)) error {
	user, err := c.Get(userId)
	if err != nil {
		return err
	}

	user.Lock()
	defer user.Unlock()
	updater(user)

	return c.persistLocked(user)
}

// persistLocked writes an updated user to disk and bumps it in the LRU.
// Caller must hold the user lock.
func (c *UserCache[T]) persistLocked(user T) error {
	user.SetLastUpdated()

	userId := user.GetUserID()
	if err := c.saveToDisk(userId, user); err != nil {
		return err
	}

	// protect shared cache/LRU structures while we insert/update the entry
	c.mu.Lock()
	defer c.mu.Unlock()
	if elem, ok := c.cache[userId]; ok {
		// already in cache: refresh value pointer and bump recency
		elem.Value.(*entry[T]).value = user
		c.touch(elem)
	} else {
		// first time we put this user in memory: add to map/LRU + enforce capacity
		ent := &entry[T]{key: userId, value: user}
		elem := c.lru.PushFront(ent)
		c.cache[userId] = elem
		if c.maxCapacity > 0 && c.lru.Len() > c.maxCapacity {
			c.evict()
		}
	}
	return nil
}

func (c *UserCache[T]) Delete(userId string) error {
	c.mu.Lock()
	if elem, ok := c.cache[userId]; ok {
		delete(c.cache, userId)
		c.lru.Remove(elem)
	}
	c.mu.Unlock()

	return c.moveToDeleted(userId)
}

// touch moves the element to the front (MRU position).
func (c *UserCache[T]) touch(elem *list.Element) {
	if elem != c.lru.Front() {
		c.lru.MoveToFront(elem)
	}
}

// finds the least recently used value and removes
// from the cache
func (c *UserCache[T]) evict() {
	if c.lru.Len() == 0 {
		return
	}
	// get the elem from lru
	elem := c.lru.Back()
	k := elem.Value.(*entry[T]).key
	// remove from cache
	delete(c.cache, k)
	// remove from lru
	c.lru.Remove(elem)
}

func (c *UserCache[T]) exists(userId string) (bool, error) {
	c.mu.RLock()
	_, ok := c.cache[userId]
	c.mu.RUnlock()
	if ok {
		return true, nil
	}
	fn := fmt.Sprintf("%s.json", userId)
	fp := filepath.Join(c.baseFolder, userId, fn)
	if _, err := os.Stat(fp); err == nil {
		return true, nil
	} else if os.IsNotExist(err) {
		return false, nil
	} else {
		return false, err
	}
}

func (c *UserCache[T]) moveToDeleted(userId string) error {
	src := filepath.Join(c.baseFolder, userId)
	if _, err := os.Stat(src); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	deletedRoot := filepath.Join(c.baseFolder, "deleted")
	if err := os.MkdirAll(deletedRoot, 0750); err != nil {
		return err
	}
	dst := filepath.Join(deletedRoot, userId)
	if err := os.RemoveAll(dst); err != nil {
		return err
	}
	return os.Rename(src, dst)
}
