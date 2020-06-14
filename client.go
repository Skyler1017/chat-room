package main

import (
	"encoding/json"
	"log"
	"time"

	"github.com/gorilla/websocket"
	"github.com/jinzhu/gorm"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 10 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512

	STATUS_LOGIN    int8 = 0
	STATUS_CHATTING int8 = 1
)

type Client struct {
	gorm.Model `json:"-"`
	Name       string `gorm:"type:varchar(20);not null;unique"`
	Password   string `gorm:"type:varchar(20);not null"`
	Sign       string `gorm:"default:'这个人很懒,还没有设置签名哦~'"`
	HubId      uint
	server     *Server         `gorm:"-"`
	hub        *Hub            `gorm:"-"`
	conn       *websocket.Conn `gorm:"-"`
	send       chan Data       `gorm:"-"`
	status     int8            `gorm:"-"`
}

func (c *Client) readPump() {
	c.conn.SetReadLimit(maxMessageSize)
	err := c.conn.SetReadDeadline(time.Now().Add(pongWait))
	if err != nil {
		log.Println("error: %v", err)
	}
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait)) //检测 pong 的消息
		return nil
	})
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}
		var d Data
		err = json.Unmarshal(message, &d)
		if err != nil {
			log.Println("fail to unmarshal pac ", string(message))
			return
		}
		c.handleRead(d)
	}
}

func (c *Client) handleRead(d Data) error {
	switch d.Header.Type {
	case TYPE_SIGN:
		c.handleSign()
	case TYPE_MSG:
		log.Println("new msg from", c.Name, ": ", d.Body)
		d.Header.Client = c.Name
		d.Header.Hub = c.hub.Name
		c.hub.broadcast <- d
	case TYPE_END:
		log.Println("wrong msg type")
	case TYPE_FILE:
		//TODO :handle file type
	default:
		log.Println("invalid msg type")
		log.Printf("%#+v\n", d)
	}
	return nil
}

func (c *Client) handleSign() {
	d := c.server.db
	sign, err := d.dbGetSign(c.ID)
	if err != nil {
		log.Println(err)
		return
	}
	var data = newData(true, TYPE_SIGN, c.Name, "", 0, c.ID, c.HubId, sign)
	c.Write(data)
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case d, ok := <-c.send: //有数据可以发送
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel.
				closeMsg := websocket.FormatCloseMessage(400, "The room has been closed.")
				c.conn.WriteMessage(websocket.CloseMessage, closeMsg)
				return
			}
			c.Write(d)
			// Add queued chat messages to the current websocket message.
			n := len(c.send)
			for i := 0; i < n; i++ {
				pac := <-c.send
				c.Write(pac)
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}

		}
	}
}

func (c *Client) Write(d Data) {
	err := c.conn.WriteJSON(d)
	if err != nil {
		log.Println(err, " fail to send", d.Body)
	} else {
	}
}
