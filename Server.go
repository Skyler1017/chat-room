package main

import (
	"encoding/json"
	"github.com/gorilla/securecookie"
	"github.com/gorilla/websocket"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
	_ "github.com/jinzhu/gorm/dialects/mysql"
)

const ERROR_TEMPLATE = "resource/error.tmpl"

type Server struct {
	addr       string
	hubs       map[string]*Hub
	hubCnt     int
	clientCnt  int
	running    bool
	db         DB
	hashKey    []byte
	blockKey   []byte
	secureCK   *securecookie.SecureCookie
	cookieName string
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func newServer(addr string) *Server {
	return &Server{
		addr:    addr,
		hubs:    make(map[string]*Hub),
		running: false,
	}
}

func (s *Server) run() {
	//s.hashKey = []byte(time.Now().Format("2006-01-02 15:04:05"))
	s.hashKey = []byte("今天也要加油鸭")
	s.blockKey = nil
	s.secureCK = securecookie.New(s.hashKey, s.blockKey)
	s.cookieName = "version1"
	r := mux.NewRouter()
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		s.serveHome(w, r)
	})
	r.HandleFunc("/my", func(w http.ResponseWriter, r *http.Request) {
		s.serveMy(w, r)
	})
	r.HandleFunc("/enter", func(w http.ResponseWriter, r *http.Request) {
		s.serveEnter(w, r)
	})
	r.HandleFunc("/return", func(w http.ResponseWriter, r *http.Request) {
		s.serveReturn(w, r)
	})
	r.HandleFunc("/register", func(w http.ResponseWriter, r *http.Request) {
		s.serveRegister(w, r)
	})
	r.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		s.serveWS(w, r)
	})
	r.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		s.serveLogin(w, r)
	})
	r.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) {
		s.serveUpload(w, r)
	})
	r.HandleFunc("/pic", func(w http.ResponseWriter, r *http.Request) {
		s.servePic(w, r)
	})
	r.PathPrefix("/layui").Handler(&DirHandler{s, false, "resource"})
	srv := &http.Server{
		Handler:      r,
		Addr:         s.addr,
		WriteTimeout: 360 * time.Second,
		ReadTimeout:  360 * time.Second,
	}
	r.PathPrefix("/file").Handler(&DirHandler{s, false, "uploaded"})
	srv2 := &http.Server{
		Handler:      r,
		Addr:         s.addr,
		WriteTimeout: 360 * time.Second,
		ReadTimeout:  360 * time.Second,
	}
	err := s.db.dbConnect()
	if err != nil {
		log.Println("fail to connect the database")
		return
	}
	err = srv.ListenAndServe()
	err = srv2.ListenAndServe()
	log.Println(err)
}

type DirHandler struct {
	s      *Server
	verify bool
	dir    string
}

/* serve 静态文件 */
func (dh *DirHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	log.Println("request for static file ", dh.dir+path)
	if dh.verify {
		ok, _ := dh.s.verifyCookie(w, r)
		if ok {
			http.ServeFile(w, r, dh.dir+path)
		} else {
			log.Println("cookie unverified")
		}
	} else {
		http.ServeFile(w, r, dh.dir+path)
	}
}

/* 通过 post 接口上传图片. 通过 cookie 校验 ip 以及登录信息 */
func (s *Server) servePic(w http.ResponseWriter, r *http.Request) {
	ok, cid := s.verifyCookie(w, r)
	if !ok {
		log.Println("unauthorized pic uploading")
		return
	}
	err := r.ParseForm()
	if err != nil {
		log.Println(err)
		return
	}
	hubName := r.Form["hub"][0]
	clientName := r.Form["client"][0]
	log.Println("client ", clientName, "uploading to ", hubName)
	h := s.hubs[hubName]
	if h == nil {
		log.Println(hubName, "hub is not loaded !")
		return
	}

	err = os.Chdir("uploaded/file")
	if err != nil {
		log.Println(err)
		return
	}
	defer os.Chdir("../..")
	file, handler, err := r.FormFile("file")
	defer file.Close()
	exist, _ := exists(hubName)
	if !exist {
		err = os.Mkdir(hubName, 0777)
	}
	err = os.Chdir(hubName)
	if err != nil {
		log.Println(err, hubName)
		return
	}
	defer os.Chdir("..")
	f, err := os.OpenFile(handler.Filename, os.O_WRONLY|os.O_CREATE, 0777)
	if err == nil {
		defer f.Close()
		_, err := io.Copy(f, file)
		if err != nil {
			log.Println(err)
			return
		}
		h.broadcast <- newData(true, TYPE_IMG, clientName, hubName, h.clientCnt, cid, h.ID, "/file/"+hubName+"/"+handler.Filename) //上传成功广播这条消息
		d := newData(true, TYPE_IMG, "", "", 0, cid, 0, "")                                                                        //回调接口不需要额外信息
		txt, err := json.Marshal(d)
		if err != nil {
			log.Println(err)
		}
		_, err = w.Write(txt)
		if err != nil {
			log.Println(err)
		}
	} else {
		log.Println(err)
		d := newData(false, TYPE_IMG, "", "", 0, cid, 0, "") // 回调接口所以不需要额外信息
		txt, err := json.Marshal(d)
		if err != nil {
			log.Println(err)
		}
		_, err = w.Write(txt)
		if err != nil {
			log.Println(err)
		}
	}
}

/* 通过 post 接口上传文件. 通过 cookie 校验 ip 以及登录信息 */
func (s *Server) serveUpload(w http.ResponseWriter, r *http.Request) {
	ok, cid := s.verifyCookie(w, r)
	if !ok {
		log.Println("unauthorized pic uploading")
		return
	}
	err := r.ParseForm()
	if err != nil {
		log.Println(err)
		return
	}
	hubName := r.Form["hub"][0]
	clientName := r.Form["client"][0]
	log.Println("client ", clientName, "uploading to ", hubName)
	h := s.hubs[hubName]
	if h == nil {
		log.Println(hubName, "hub is not loaded !")
		return
	}
	h.clients.Load(clientName)
	err = os.Chdir("uploaded/file")
	if err != nil {
		log.Println(err)
		return
	}
	defer os.Chdir("../..")
	file, handler, err := r.FormFile("file")
	defer file.Close()
	exist, _ := exists(hubName)
	if !exist {
		err = os.Mkdir(hubName, 0777)
	}
	err = os.Chdir(hubName)
	if err != nil {
		log.Println(err, hubName)
		return
	}
	defer os.Chdir("..")
	f, err := os.OpenFile(handler.Filename, os.O_WRONLY|os.O_CREATE, 0777)
	if err == nil {
		defer f.Close()
		_, err := io.Copy(f, file)
		if err != nil {
			log.Println(err)
			return
		}
		h.broadcast <- newData(true, TYPE_FILE, clientName, hubName, cid, h.ID, h.clientCnt, "/file/"+hubName+"/"+handler.Filename) //上传成功广播这条消息
		d := newData(true, TYPE_FILE, "", "", 0, 0, 0, "")                                                                          //回调接口不需要额外信息
		txt, err := json.Marshal(d)
		if err != nil {
			log.Println(err)
		}
		_, err = w.Write(txt)
		if err != nil {
			log.Println(err)
		}
	} else {
		log.Println(err)
		d := newData(false, TYPE_FILE, "", "", 0, cid, 0, "") // 回调接口所以不需要额外信息
		txt, err := json.Marshal(d)
		if err != nil {
			log.Println(err)
		}
		_, err = w.Write(txt)
		if err != nil {
			log.Println(err)
		}
	}
}

/* 用户注册区分 get 和post 请求 */
func (s *Server) serveRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		http.ServeFile(w, r, "resource/register.html")
	} else if r.Method == "POST" {
		err := r.ParseForm()
		if err != nil {
			log.Println(err)
			return
		}
		values := r.PostForm
		if len(values["name"]) == 0 || len(values["password"]) == 0 {
			log.Printf("can not parse name and password: %#+v\n", values)
			err = errTemplateGenerator(w, ERROR_TEMPLATE, "登陆信息似乎没有填完呢")
			return
		}
		name := values["name"][0]
		password := values["password"][0]
		if !VerifyUserName(name) {
			err = errTemplateGenerator(w, ERROR_TEMPLATE, "用户名好像不太对哦")
			return
		}
		if !VerifyUserPassword(password) {
			err = errTemplateGenerator(w, ERROR_TEMPLATE, "密码好像不太对哦")
			return
		}
		c, err := s.db.dbSignUp(name, password)
		if err == nil {
			err = s.SetCookieHandler(c, w, r)
			if err != nil {
				log.Println(err)

			}
			http.Redirect(w, r, "/my", 302)
		} else {
			log.Println(err)
			err = errTemplateGenerator(w, ERROR_TEMPLATE, err.Error())
			if err != nil {
				log.Println(err)
				http.Redirect(w, r, "/register", 302)
			}
		}
	} else {
		err := errTemplateGenerator(w, ERROR_TEMPLATE, "Method not allowed")
		if err != nil {
			log.Println(err)
		}
	}
}

/* 用户登录请求, 区分 get 和 post*/
func (s *Server) serveLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		// cookie 有效则无需登录
		ok, _ := s.verifyCookie(w, r)
		if ok {
			http.Redirect(w, r, "/my", 302)
		} else {
			http.ServeFile(w, r, "resource/login.html")
		}
	} else if r.Method == "POST" {
		err := r.ParseForm()
		if err != nil {
			log.Println(err)
			err = errTemplateGenerator(w, ERROR_TEMPLATE, "提交的表单似乎有问题呢")
			if err != nil {
				log.Println(err)
				//http.ServeFile(w, r, "/login")
			}
			return
		}
		values := r.PostForm
		if len(values["name"]) == 0 || len(values["password"]) == 0 {
			log.Printf("can not parse name and password: %#+v\n", values)
			err = errTemplateGenerator(w, ERROR_TEMPLATE, "登陆信息似乎没有填完呢")
			if err != nil {
				log.Println(err)
				http.ServeFile(w, r, "/login")
			}
			return
		}
		name := values["name"][0]
		password := values["password"][0]
		c, err := s.db.dbSignIn(name, password)
		// 登录成功,设置cookie并重定向到主页面
		if err == nil {
			err = s.SetCookieHandler(c, w, r)
			if err != nil {
				log.Println(err)
				http.Redirect(w, r, "/login", 302)
			}
			http.Redirect(w, r, "/my", 302)
			// 登录失败, 提示并返回上一页
		} else {
			log.Println(err)
			err = errTemplateGenerator(w, ERROR_TEMPLATE, err.Error())
			// 模板生成失败, 直接返回login页面
			if err != nil {
				log.Println(err)
				http.ServeFile(w, r, "resource/login.html")
			}
		}
	} else {
		_, err := w.Write([]byte("method not allowed"))
		if err != nil {
			log.Println(err)
		}
	}
}

func (s *Server) serveMy(w http.ResponseWriter, r *http.Request) {
	ok, cid := s.verifyCookie(w, r)
	if ok {
		c, err := s.db.dbGetClient(cid)
		if err != nil {
			log.Println(err)
			return
		}
		d, err := s.handleVisitHistory(c)
		if err != nil {
			log.Println(err)
			return
		}
		_ = myTemplateGenerator(w, "resource/my.tmpl", "", c.Name, 0, c.ID, c.Sign, d)
		//http.ServeFile(w, r, "resource/my.html")
	} else {
		http.Redirect(w, r, "/login", 302)
	}
}

/* enter页面，分为POST请求和GET请求 */
func (s *Server) serveEnter(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		ok, cid := s.verifyCookie(w, r)
		if ok {
			c, err := s.db.dbGetClient(cid)
			if err != nil {
				log.Println(err)
				return
			}
			//GET请求还未进入房间
			err = enterTemplateGenerator(w, "resource/enter.tmpl", "", c.Name, c.HubId, c.ID, c.Sign)
			if err != nil {
				log.Println(err)
				return
			}
		} else {
			http.Redirect(w, r, "/login", 302)
		}
	} else if r.Method == "POST" { //POST 请求会进入对应的房间
		ok, cid := s.verifyCookie(w, r)
		if !ok {
			http.Redirect(w, r, "/login", 302)
			return
		}
		//得到房间号
		err := r.ParseForm()
		if err != nil {
			log.Println(err)
			err = errTemplateGenerator(w, ERROR_TEMPLATE, "房间号似乎有问题呢")
			if err != nil {
				log.Println(err)
			}
			return
		}
		values := r.PostForm
		if len(values["HubName"]) == 0 || len(values["Anonymous"]) == 0 {
			log.Printf("can not parse hub name: %#+v\n", values)
			err = errTemplateGenerator(w, ERROR_TEMPLATE, "房间信息似乎没有填好呢")
			if err != nil {
				log.Println(err)
			}
			return
		}
		hubName := values["HubName"][0]
		anonymous := values["Anonymous"][0]
		//获得用户的ID
		c, err := s.db.dbGetClient(cid)
		if err != nil {
			log.Println(err)
			return
		}
		// 加载房间
		var h *Hub
		if s.hubs[hubName] == nil {
			log.Println("没有找到hub, loading ", hubName)
			h, err = s.loadHub(HubInfo{
				HubName:   hubName,
				Anonymous: anonymous,
			})
			if err != nil {
				log.Println(err)
				return
			}
		} else {
			log.Println("找到hub, entering", hubName)
			h = s.hubs[hubName]
		}
		go h.run()
		//生成模板返回
		err = hubTemplateGenerator(w, "resource/hub.tmpl", hubName, c.Name, h.ID, c.ID, c.Sign)
		if err != nil {
			log.Println(err)
			return
		}
	} else {
		_, _ = w.Write([]byte("Method not allowed\n"))
	}
}

func (s *Server) serveReturn(w http.ResponseWriter, r *http.Request) {
	ok, cid := s.verifyCookie(w, r)
	if !ok {
		http.Redirect(w, r, "/login", 302)
		return
	}
	hubName := r.URL.Query()["HubName"][0]
	//获得用户的ID
	c, err := s.db.dbGetClient(cid)
	if err != nil {
		log.Println(err)
		return
	}
	// 加载房间
	var h *Hub
	if s.hubs[hubName] == nil {
		log.Println("没有找到hub, loading ", hubName)
		h, err = s.loadHub(HubInfo{
			HubName:   hubName,
			Anonymous: "false",
		})
		if err != nil {
			log.Println(err)
			return
		}
	} else {
		log.Println("找到hub, entering", hubName)
		h = s.hubs[hubName]
	}
	go h.run()
	//生成模板返回
	err = hubTemplateGenerator(w, "resource/hub.tmpl", hubName, c.Name, h.ID, c.ID, c.Sign)
	if err != nil {
		log.Println(err)
		return
	}

}

/* 根据cookie是否存在重定向至登录页面或主页面 */
func (s *Server) serveHome(w http.ResponseWriter, r *http.Request) {
	ok, _ := s.verifyCookie(w, r)
	if ok {
		http.Redirect(w, r, "/my", 302)
	} else {
		http.ServeFile(w, r, "resource/login.html")
	}
}

/* 处理房间内的 WS 连接 */
func (s *Server) serveWS(w http.ResponseWriter, r *http.Request) {
	ok, id := s.verifyCookie(w, r)
	if !ok {
		log.Println("unauthorized ws connection")
		return
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	c, err := s.db.dbGetClient(id)
	if err != nil {
		log.Println(err)
		return
	}
	hubName := r.URL.Query()["HubName"][0]
	h := s.hubs[hubName]
	s.entryHub(&c, h)
	c.conn = conn
	c.send = make(chan Data, 64)
	c.server = s
	go c.readPump()
	go c.writePump()
	for i := 0; i < len(c.hub.logCache); i++ {
		c.Write(c.hub.logCache[i])
	}
}

func (s *Server) loadHub(info HubInfo) (*Hub, error) {
	h, err := s.db.dbInitOrCreateHub(info, s)
	if err == nil {
		s.hubs[info.HubName] = h
		go h.run()
		logs, err := s.db.dbGetData(h.ID)
		if err != nil {
			log.Println(err)
			return nil, err
		}
		for j := 0; j < len(logs); j++ {
			h.logCache = append(h.logCache, logs[j])
		}
		log.Println("new hub and chat history loaded")
		return h, nil
	}
	log.Println(err)
	return nil, err
}

func (s *Server) entryHub(c *Client, h *Hub) {
	h.register <- c
	_ = s.db.dbAddVisitHistory(c, h)
	log.Println("enter hub done")
}
