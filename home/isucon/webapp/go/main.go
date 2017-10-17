package main

import (
	"database/sql"
	"fmt"
	//"github.com/go-martini/martini"
	_ "github.com/go-sql-driver/mysql"
	//"github.com/martini-contrib/render"
	//"github.com/martini-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/gin-contrib/sessions"
	"log"
	//"net"
	//"net/http"
	"os"
	"strconv"
)

var db *sql.DB
var (
	UserLockThreshold int
	IPBanThreshold    int
)

func init() {
	log.Println("initialize start")
	dsn := fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s?parseTime=true&loc=Local",
		getEnv("ISU4_DB_USER", "root"),
		getEnv("ISU4_DB_PASSWORD", ""),
		getEnv("ISU4_DB_HOST", "localhost"),
		getEnv("ISU4_DB_PORT", "3306"),
		getEnv("ISU4_DB_NAME", "isu4_qualifier"),
	)

	var err error

	db, err = sql.Open("mysql", dsn)
	if err != nil {
		panic(err)
	}

	UserLockThreshold, err = strconv.Atoi(getEnv("ISU4_USER_LOCK_THRESHOLD", "3"))
	if err != nil {
		panic(err)
	}

	IPBanThreshold, err = strconv.Atoi(getEnv("ISU4_IP_BAN_THRESHOLD", "10"))
	if err != nil {
		panic(err)
	}

	initializeInmemmoryDB()
	log.Println("initialize done")
}

func main() {
	//m := martini.Classic()
	r := gin.Default()

	store := sessions.NewCookieStore([]byte("secret-isucon"))
	//m.Use(sessions.Sessions("isucon_go_session", store))
	r.Use(sessions.Sessions("isucon_go_session", store))

	//m.Use(martini.Static("../public"))
	r.Static("/images", "../public/images")
	r.Static("/stylesheets", "../public/stylesheets")
	//m.Use(render.Renderer(render.Options{
	//	Layout: "layout",
	//}))
	r.LoadHTMLGlob("templates/*")

	//m.Get("/", func(r render.Render, session sessions.Session) {
	r.GET("/", func(c *gin.Context) {
		//r.HTML(200, "index", map[string]string{"Flash": getFlash(session, "notice")})
		c.HTML(200, "index.tmpl", gin.H{"Flash": getFlash(sessions.Default(c), "notice")})
	})

	//m.Post("/login", func(req *http.Request, r render.Render, session sessions.Session) {
	r.POST("/login", func(c *gin.Context) {
		user, err := attemptLogin(c.Request)
		session := sessions.Default(c)
		notice := ""
		if err != nil || user == nil {
			switch err {
			case ErrBannedIP:
				notice = "You're banned."
			case ErrLockedUser:
				notice = "This account is locked."
			default:
				notice = "Wrong username or password"
			}

			session.Set("notice", notice)
			session.Save()
			//r.Redirect("/")
			c.Redirect(302, "/")
			return
		}

		session.Set("user_id", strconv.Itoa(user.ID))
		session.Save()
		//r.Redirect("/mypage")
		c.Redirect(302, "/mypage")
	})

	//m.Get("/mypage", func(r render.Render, session sessions.Session) {
	r.GET("/mypage", func(c *gin.Context) {
		session := sessions.Default(c)
		currentUser := getCurrentUser(session.Get("user_id"))

		if currentUser == nil {
			session.Set("notice", "You must be logged in")
			session.Save()
			//r.Redirect("/")
			c.Redirect(302, "/")
			return
		}

		currentUser.getLastLogin()
		//r.HTML(200, "mypage", currentUser)
		c.HTML(200, "mypage.tmpl", currentUser)
	})

	//m.Get("/report", func(r render.Render) {
	r.GET("/report", func(c *gin.Context) {
		//r.JSON(200, map[string][]string{
		c.JSON(200, gin.H{
			"banned_ips":   bannedIPs(),
			"locked_users": lockedUsers(),
		})
	})

/*
	m.Get("/version", func(r render.Render) {
		r.JSON(200, map[string]string{
			"version": "1",
		})
	})

	m.Get("/LoginLogDB", func(r render.Render) {
		r.JSON(200, LoginLogDB)
	})
	m.Get("/LoginLogDBIndexIP", func(r render.Render) {
		r.JSON(200, LoginLogDBIndexIP)
	})
	m.Get("/P", func(r render.Render) {
		r.JSON(200, LastLoginDBIndexUserID[195001:195100])
	})
*/

	var socket_path = "/tmp/golang-webapp.sock"
	os.Remove(socket_path)

/*
	socketListener, err := net.Listen("unix", socket_path)
	if err != nil {
		panic(err)
	}

	http.Serve(socketListener, m)
*/
	r.RunUnix(socket_path)
}
