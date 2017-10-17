package main

import (
	"database/sql"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"github.com/gin-gonic/gin"
	"github.com/gin-contrib/sessions"
	"log"
	"os"
	"strconv"
	"runtime"
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
	cpus := runtime.NumCPU()
	runtime.GOMAXPROCS(cpus)

	r := gin.Default()

	store := sessions.NewCookieStore([]byte("secret-isucon"))
	r.Use(sessions.Sessions("isucon_go_session", store))

	//r.Static("/images", "../public/images")
	//r.Static("/stylesheets", "../public/stylesheets")
	r.LoadHTMLGlob("templates/*")

/*
	r.GET("/", func(c *gin.Context) {
		session := sessions.Default(c)
		c.HTML(200, "index.tmpl", gin.H{"Flash": getFlash(session, "notice")})
		session.Delete("notice")
		session.Save()
	})
*/

	r.POST("/login", func(c *gin.Context) {
		user, err := attemptLogin(c.Request)
		session := sessions.Default(c)
		if err != nil || user == nil {
			c.Redirect(302, "/")
			//notice := ""
			switch err {
			case ErrBannedIP:
				c.SetCookie("t", "b", 0, "", "", false, false)
				//notice = "You're banned."
			case ErrLockedUser:
				c.SetCookie("t", "l", 0, "", "", false, false)
				//notice = "This account is locked."
			default:
				c.SetCookie("t", "w", 0, "", "", false, false)
				//notice = "Wrong username or password"
			}

			//session.Set("notice", notice)
			//session.Save()
			//c.Redirect(302, "/")
			return
		}

		c.Redirect(302, "/mypage")
		session.Set("user_id", strconv.Itoa(user.ID))
		session.Save()
		//c.Redirect(302, "/mypage")
	})

	r.GET("/mypage", func(c *gin.Context) {
		session := sessions.Default(c)
		currentUser := getCurrentUser(session.Get("user_id"))

		if currentUser == nil {
			c.Redirect(302, "/")
			session.Set("notice", "You must be logged in")
			session.Save()
			//c.Redirect(302, "/")
			return
		}

		currentUser.getLastLogin()
		c.HTML(200, "mypage.tmpl", currentUser)
	})

	r.GET("/report", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"banned_ips":   bannedIPs(),
			"locked_users": lockedUsers(),
		})
	})

	var socket_path = "/tmp/golang-webapp.sock"
	os.Remove(socket_path)
	r.RunUnix(socket_path)
}
