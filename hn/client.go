// Package hn implements a really basic Hacker News client
package hn

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
)

const (
	apiBase = "https://hacker-news.firebaseio.com/v0"
)

type Client struct {
	apiBase string
	cache *ItemCache
}

func (c *Client) defaultify() {
	if c.cache == nil {
		c.apiBase = apiBase
		c.cache = ItemCacheSingleton()
	}
}

func (c *Client) TopItems() ([]int, error) {
	c.defaultify()
	resp, err := http.Get(fmt.Sprintf("%s/topstories.json", c.apiBase))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var ids []int
	err = json.NewDecoder(resp.Body).Decode(&ids)
	return ids, err
}

func (c *Client) processItem(id int, items chan *ItemWithUrl, storyCount Counter) {
	item, ok := c.cache.get(id)
	if !ok {
		c.defaultify()
		resp, err := http.Get(fmt.Sprintf("%s/item/%d.json", c.apiBase, id))
		if err != nil { return }
		defer resp.Body.Close()
		dec := json.NewDecoder(resp.Body)
		err = dec.Decode(&item)
		if err != nil { return }
		c.cache.put(id, item)
	}
	if itemWithUrl := addHost(item); isStoryLink(itemWithUrl) {
	  items <- itemWithUrl
		storyCount.incr()
  }
}

var dashes = [...]string { "", "-", "--", "---", "----", "-----", "------" }

func (c *Client) worker(w int, storyCount Counter, coreCount Counter, nextStoryId IntValue, items chan *ItemWithUrl) {
	for !storyCount.done() {
    c.processItem(nextStoryId(), items, storyCount)
  }
	coreCount.incr()
	if coreCount.done() {
		close(items)
	}
}

type IntValue func() int
type Action func()
type Condition func() bool
type Counter struct {
	i *int
  incr Action
	done Condition
}

func mkNextItem(items []int) IntValue {
	var mutex sync.Mutex
	i := 0
  return func() int {
		mutex.Lock()
		defer mutex.Unlock()
		id := ids[i]
		i++
		return id
	}
}

func mkCounter(max int) Counter {
	i := max
	var mutex sync.Mutex
	return Counter{
		i: &i,
		incr: func() { mutex.Lock(); defer mutex.Unlock(); i--  },
		done: func() bool { mutex.Lock(); defer mutex.Unlock(); return i <= 0 },
	}
}

const NumOfCores=6

func (client *Client) RetrieveStories(numStories int, ids []int) []*ItemWithUrl {
	var stories []*ItemWithUrl
	itemWithUrls := make(chan *ItemWithUrl, 2*numStories)
	nextStoryId := mkNextItem(ids)
  storyCount := mkCounter(numStories)
	coreCount := mkCounter(NumOfCores)

	for core := 1; core <= NumOfCores; core++ {
		go client.worker(core, storyCount, coreCount, nextStoryId, itemWithUrls)
	}

	for {
		item, ok := <- itemWithUrls
		if !ok { break }
		stories = append(stories, item)
	}

	sort.Slice(stories, func(i, j int) bool {
		return stories[i].Item.ID < stories[j].Item.ID
	})
	return stories[:numStories]
}

type Item struct {
	By          string `json:"by"`
	Descendants int    `json:"descendants"`
	ID          int    `json:"id"`
	Kids        []int  `json:"kids"`
	Score       int    `json:"score"`
	Time        int    `json:"time"`
	Title       string `json:"title"`
	Type        string `json:"type"`

	Text string `json:"text"`
	URL  string `json:"url"`
}

func isStoryLink(item *ItemWithUrl) bool {
	return item.Item.Type == "story" && item.Item.URL != ""
}

func addHost(item *Item) *ItemWithUrl {
	ret := ItemWithUrl{Item: item}
	if url, err := url.Parse(ret.Item.URL); err == nil {
		ret.Host = strings.TrimPrefix(url.Hostname(), "www.")
	}
	return &ret
}

type ItemWithUrl struct {
	Item *Item
	Host string
}
