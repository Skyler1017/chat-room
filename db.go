package main

import (
	"errors"
	"log"
	"sync"
	"time"

	"github.com/jinzhu/gorm"
)

const (
	DUPLICATE_CLIENT = "try to create a client that already exsits"
	DUPLICATE_HUB    = "try to create a hub that already exsits"
	NULL_HUB         = "try to add log to a null hub"
	WRONG_PASSWORD   = "password is incorrect"
)

type DB struct {
	db *gorm.DB
}

type VisitHistory struct {
	gorm.Model `json:"-"`
	CId        uint `gorm:"type:varchar(20);primary-key;not null"`
	HubId      uint `gorm:"type:varchar(20);primary-key;not null"`
}

func (d *DB) dbConnect() error {
	db, err := gorm.Open("mysql", "root:BrhmY@5468Ph@(9.134.34.125:3306)/WHH?charset=utf8&parseTime=True&loc=Local")
	if err != nil {
		log.Println("cannot open database", err)
		return errors.New("cannot open database" + err.Error())
	}
	db.LogMode(true)
	d.db = db
	db.AutoMigrate(&Hub{})
	db.AutoMigrate(&Client{})
	db.AutoMigrate(&HubLog{})
	db.AutoMigrate(&VisitHistory{})
	return nil
}

func (d *DB) dbGetClient(id uint) (Client, error) {
	var c Client
	d.db.Where("id=?", id).First(&c)
	return c, d.db.Error
}

func (d *DB) dbSignUp(name, password string) (Client, error) {
	var cnt int8
	target := d.db.Where("name = ?", name).Find(&Client{})
	target.Count(&cnt)
	if d.db.Error != nil {
		return Client{}, d.db.Error
	}
	if cnt == 0 {
		c := Client{Name: name, Password: password}
		d.db.Create(&c)
		return c, d.db.Error
	} else {
		return Client{}, errors.New("用户名似乎已被注册了呢")
	}
}

func (d *DB) dbSignIn(name, password string) (Client, error) {
	var c Client
	d.db.Where("name = ?", name).First(&c)
	if c.Password == password {
		return c, nil
	}
	if d.db.RecordNotFound() {
		return c, errors.New("用户似乎并不存在呢")
	}
	return c, errors.New("用户名还是密码错了哦，再检查一下吧")
}

func (d *DB) dbInitOrCreateHub(info HubInfo, s *Server) (*Hub, error) {
	var cnt int8
	target := d.db.Where("name = ?", info.HubName).Find(&Hub{})
	target.Count(&cnt)
	if d.db.Error != nil {
		return nil, d.db.Error
	}
	h := Hub{
		register:   make(chan *Client, 256),
		unregister: make(chan *Client, 256),
		broadcast:  make(chan Data, 256),
		clients:    sync.Map{},
		running:    make(chan bool),
		server:     s,
		Name:       info.HubName,
	}
	if cnt == 0 {
		if info.Anonymous == "on" {
			h.anonymous = true
		} else {
			h.anonymous = false
		}
		d.db.Create(&h)
		return &h, d.db.Error
	} else {
		d.db.Where("name = ?", info.HubName).First(&h)
		return &h, nil
	}
}

func (d *DB) dbDeleteClient(c *Client) error {
	d.db.Delete(c)
	return d.db.Error
}

func (d *DB) dbAddVisitHistory(c *Client, h *Hub) error {
	history := VisitHistory{
		CId:   c.ID,
		HubId: h.ID,
	}
	d.db.FirstOrCreate(&history, history)
	d.db.Model(&history).Update("updated_at", time.Now())
	return d.db.Error
}

func (d *DB) dbAddHub(h *Hub) error {
	var cnt int8
	d.db.Find(&Hub{}).Where(h).Count(&cnt)
	if cnt == 0 {
		d.db.Create(h).Scan(h)
		return d.db.Error
	}
	return errors.New(DUPLICATE_HUB)
}

func (d *DB) dbDeleteHub(h *Hub) error {
	d.db.Delete(h)
	return d.db.Error
}

func (d *DB) dbAddData(data Data) error {
	var cnt int8
	d.db.Where("id=?", data.Header.HubId).Find(&Hub{}).Count(&cnt)
	if cnt == 0 {
		return errors.New(NULL_HUB)
	}
	l := data.toLog()
	d.db.Create(&l)
	return d.db.Error
}

func (d *DB) dbGetData(hubId uint) ([]Data, error) {
	var logList []HubLog
	d.db.Where("hub_id=?", hubId).Find(&logList)
	var dataList []Data
	for i := range logList {
		dataList = append(dataList, logList[i].toData())
	}
	return dataList, d.db.Error
}

func (d *DB) dbGetVisitHistory(cid uint) ([]VisitHistory, error) {
	var ans []VisitHistory
	d.db.Where("c_id = ?", cid).Find(&ans)
	return ans, d.db.Error
}

func (d *DB) dbGetSign(cid uint) (string, error) {
	var c Client
	d.db.Where("id=?", cid).First(&c)
	return c.Sign, d.db.Error
}

func (d *DB) dbAlterName(cid uint) error {
	//TODO
	return nil
}
