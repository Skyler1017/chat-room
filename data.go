package main

import (
	"github.com/jinzhu/gorm"
	"time"
)

const (
	TYPE_HUB_HISTORY int8 = 1
	TYPE_SIGN        int8 = 2
	TYPE_HUB_ENTER   int8 = 3
	TYPE_IMG         int8 = 4
	TYPE_MSG         int8 = 5
	TYPE_END         int8 = 6
	TYPE_FILE        int8 = 7
)

type Data struct {
	Header Header `json:"Header"`
	Body   string `json:"Body"`
}

type HubLog struct {
	gorm.Model `json:"-"`
	Type       int8
	Time       string
	Client     string
	Hub        string
	CId        uint
	HubId      uint
	Body       string
}

func (d *Data) toLog() HubLog {
	return HubLog{
		Model:  gorm.Model{},
		Type:   1,
		Time:   d.Header.Time,
		Client: d.Header.Client,
		Hub:    d.Header.Hub,
		CId:    d.Header.CId,
		HubId:  d.Header.HubId,
		Body:   d.Body,
	}
}

func (l *HubLog) toData() Data {
	return Data{
		Header: Header{
			Code:   true,
			Type:   l.Type,
			Time:   l.Time,
			Client: l.Client,
			Hub:    l.Hub,
			CId:    l.CId,
			HubId:  l.HubId,
			Users:  0,
		},
		Body: l.Body,
	}
}

type Header struct {
	Code   bool //success or fail
	Type   int8
	Time   string
	Client string
	Hub    string
	CId    uint
	HubId  uint
	Users  uint
}

type HubInfo struct {
	HubName   string
	Anonymous string
}

func newData(ok bool, Type int8, client string, hub string, CId uint, hubId uint, cnt uint, body string) Data {
	return Data{
		Header: Header{
			Code:   ok,
			Type:   Type,
			HubId:  hubId,
			CId:    CId,
			Client: client,
			Hub:    hub,
			Users:  cnt,
			Time:   time.Now().Format("2006-01-02 15:04:05")},
		Body: body,
	}
}
