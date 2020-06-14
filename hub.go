package main

import (
	"github.com/jinzhu/gorm"
	"log"
	"os"
	"sync"
)

type Hub struct {
	gorm.Model
	Name       string       `gorm:"type:varchar(20);unique;not null"`
	server     *Server      `gorm:"-"`
	clients    sync.Map     `gorm:"-"`
	clientCnt  uint         `gorm:"-"`
	broadcast  chan Data    `gorm:"-"`
	register   chan *Client `gorm:"-"`
	unregister chan *Client `gorm:"-"`
	running    chan bool    `gorm:"-"`
	fd         os.File      `gorm:"-"`
	userCnt    int          `gorm:"-"`
	logCache   []Data       `gorm:"-"`
	anonymous  bool         `gorm:"-"`
}

func (h *Hub) run() {
	for {
		select {
		case t := <-h.running:
			if !t {
				return
			}
		case c := <-h.register:
			{
				h.clients.Store(c, true)
				h.clientCnt += 1
				c.hub = h
				c.HubId = h.ID
				log.Println("new client entered: ", h.Name)
			}
		case c := <-h.unregister:
			{
				if _, ok := h.clients.Load(c); ok {
					h.clients.Delete(c)
					h.clientCnt -= 1
					close(c.send)
				}
			}
		case msg := <-h.broadcast: //处理可以广播出去的消息(聊天消息)
			err := h.server.db.dbAddData(msg)
			if err != nil {
				log.Println(err, "fail to append logs")
			}
			h.logCache = append(h.logCache, msg)
			cnt := 0
			h.clients.Range(func(key, value interface{}) bool {
				c := key.(*Client)
				select {
				case c.send <- msg: //client 可以发送则发送
					cnt++
				default: //否则关掉这条管道
					close(c.send)
					h.clients.Delete(c)
					h.clientCnt -= 1
				}
				return true
			})
			if cnt == 0 {
				h.shutdown(true)
			}
		}

	}
}

func (h *Hub) shutdown(soft bool) {
	h.running <- false
	if soft {
	}
	var unsentMsg []Data
	for msg := range h.broadcast {
		unsentMsg = append(unsentMsg, msg)
	}
	h.clients.Range(func(key, value interface{}) bool {
		c := key.(*Client)
		close(c.send)
		return true
	})
}
