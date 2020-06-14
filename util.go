package main

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"time"
)

/* 判断本路径下文件是否存在 */
func exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return true, err
}

/* 从 remote addr 获取 ip 地址 */
func getIP(addr string) string {
	pos := 0
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			pos = i
			break
		}
	}
	return addr[:pos]
}

/* 检查用户的 cookie,返回 ok 和 client_id */
func (s *Server) verifyCookie(w http.ResponseWriter, r *http.Request) (bool, uint) {
	cookie, err := s.ReadCookieHandler(w, r)
	if err != nil {
		log.Println(err)
		return false, 0
	}
	sign := cookie["Sign"]
	if sign != "valid" {
		log.Println("fail to verify client's signature")
		return false, 0
	}
	ip := cookie["IP"]
	if ip != getIP(r.RemoteAddr) {
		log.Printf("changed ip, %s -> %s\n", ip, r.RemoteAddr)
		return false, 0
	}
	id, _ := strconv.ParseUint(cookie["Id"], 10, 32)
	return true, uint(id)
}

/* 设置cookie */
func (s *Server) SetCookieHandler(c Client, w http.ResponseWriter, r *http.Request) error {
	value := map[string]string{
		"Id":      strconv.FormatInt(int64(c.ID), 10),
		"HubName": c.Name,
		"Sign":    "valid",
		"IP":      getIP(r.RemoteAddr),
	}
	if encoded, err := s.secureCK.Encode(s.cookieName, value); err == nil {
		cookie := &http.Cookie{
			Name:     s.cookieName,
			Value:    encoded,
			Path:     "",
			Secure:   false,
			HttpOnly: true,
			Expires:  time.Now().AddDate(0, 0, 10), //一天过期
		}
		http.SetCookie(w, cookie)
		return nil
	} else {
		return err
	}
}

/* 读取cookie */
func (s *Server) ReadCookieHandler(w http.ResponseWriter, r *http.Request) (map[string]string, error) {
	if cookie, err := r.Cookie(s.cookieName); err == nil {
		value := make(map[string]string)
		if err = s.secureCK.Decode(s.cookieName, cookie.Value, &value); err == nil {
			return value, nil
		} else {
			return map[string]string{}, err
		}
	} else {
		return map[string]string{}, err
	}
}

/* 登陆或注册失败时生成错误消息并返回上一页的js代码 */
func errTemplateGenerator(w http.ResponseWriter, filename, msg string) error {
	tpl, err := template.ParseFiles(filename)
	if err != nil {
		log.Println(err)
	}
	var mp = map[string]string{
		"msg": msg,
	}
	err = tpl.Execute(w, mp)
	if err != nil {
		log.Println(err)
	}
	return err
}

/* Enter页面所需要的模版信息 */
type EnterInfo struct {
	HubName    string
	ClientName string
	HubId      int
	ClientId   int
	Sign       string
}

/* 用模板生成 enter 页面 */
func enterTemplateGenerator(w http.ResponseWriter, filename, hub_name string, client_name string, hub_id uint, client_id uint, sign string) error {
	tpl, err := template.ParseFiles(filename)
	if err != nil {
		log.Println(err)
		return err
	}
	p := EnterInfo{
		HubName:    hub_name,
		ClientName: client_name,
		HubId:      int(hub_id),
		ClientId:   int(client_id),
		Sign:       sign,
	}
	err = tpl.Execute(w, p)
	if err != nil {
		log.Println(err)
		return err
	}
	return nil
}

/* My页面所需要的模板信息 */
type MyInfo struct {
	HubName    string
	ClientName string
	HubId      int
	ClientId   int
	Sign       string
	Data       Data
}

/* 用模板生成 my 页面 */
func myTemplateGenerator(w http.ResponseWriter, filename string, hub_name string, client_name string, hub_id uint, client_id uint, sign string, d Data) error {
	tpl, err := template.ParseFiles(filename)
	if err != nil {
		log.Println(err)
		return err
	}
	p := MyInfo{
		HubName:    hub_name,
		ClientName: client_name,
		HubId:      int(hub_id),
		ClientId:   int(client_id),
		Sign:       sign,
		Data:       d,
	}
	err = tpl.Execute(w, p)
	if err != nil {
		log.Println(err)
		return err
	}
	return nil
}

/* 生成主页面所需要的足迹 Data 包 */
func (s *Server) handleVisitHistory(c Client) (Data, error) {
	d := s.db
	histories, err := d.dbGetVisitHistory(c.ID)
	if err != nil {
		log.Println(err)
		return Data{}, err
	}
	data := newData(true, TYPE_HUB_HISTORY, c.Name, "", 0, c.ID, c.HubId, "")
	type hub struct {
		Id   uint
		Name string
		Log  string
	}
	var li []hub
	max := 7
	if len(histories) < 7 {
		max = len(histories)
	}
	for i := 0; i < max; i++ {
		hubId := histories[i].HubId
		var h Hub
		s.db.db.Where("ID=?", hubId).First(&h)
		logs, err := s.db.dbGetData(hubId)
		if err != nil {
			log.Println(err)
			return Data{}, err
		}
		var chat_log string
		for j := 0; j < len(logs); j++ {
			chat_log += logs[j].Header.Client + ": " + logs[j].Body + "\n"
		}
		li = append(li, hub{
			h.ID, h.Name, chat_log,
		})
	}
	str, err := json.Marshal(li)
	if err != nil {
		log.Println(err)
		return Data{}, err
	}
	data.Body = string(str) //TODO better way ?
	return data, err
}

/* 模板生成hub页面 */
func hubTemplateGenerator(w http.ResponseWriter, filename, hub_name string, client_name string, hub_id uint, client_id uint, sign string) error {
	tpl, err := template.ParseFiles(filename)
	if err != nil {
		log.Println(err)
		return err
	}
	p := EnterInfo{
		HubName:    hub_name,
		ClientName: client_name,
		HubId:      int(hub_id),
		ClientId:   int(client_id),
		Sign:       sign,
	}
	err = tpl.Execute(w, p)
	if err != nil {
		log.Println(err)
		return err
	}
	return nil
}

/* 匹配电子邮箱 */
func VerifyEmailFormat(email string) bool {
	//pattern := `\w+([-+.]\w+)*@\w+([-.]\w+)*\.\w+([-.]\w+)*` //匹配电子邮箱
	pattern := `^[0-9a-z][_.0-9a-z-]{0,31}@([0-9a-z][0-9a-z-]{0,30}[0-9a-z]\.){1,4}[a-z]{2,4}$`
	reg := regexp.MustCompile(pattern)
	return reg.MatchString(email)
}

/* 检验用户名是否符合条件 */
func VerifyUserName(name string) bool {
	pattern := `^[a-z0-9_-]{3,16}$`
	reg := regexp.MustCompile(pattern)
	return reg.MatchString(name)
}

/* 检验密码是否符合条件 */
func VerifyUserPassword(password string) bool {
	pattern := `^[a-z0-9_-]{6,18}$`
	reg := regexp.MustCompile(pattern)
	return reg.MatchString(password)
}
