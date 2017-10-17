package main

import (
	"errors"
	"net/http"
	"strconv"
	"sync"
	"time"
	"golang.org/x/sync/errgroup"
)

var (
	ErrBannedIP      = errors.New("Banned IP")
	ErrLockedUser    = errors.New("Locked user")
	ErrUserNotFound  = errors.New("Not found user")
	ErrWrongPassword = errors.New("Wrong password")
)

var (
	LoginLogDBMutex             sync.RWMutex
	LoginLogDB                  = make([]int, 200010)
	LoginLogDBIndexIPMutex      sync.RWMutex
	LoginLogDBIndexIP           = make(map[string]int, 80000)
	UserNameDB                  = make([]string, 200010)
	UserDBIndexID               = make([]*User, 200010)
	UserDBIndexlogin            = make(map[string]*User, 200010)
	LastLoginDBIndexUserID      = make([][2]*LastLogin, 200010)
	LastLoginDBIndexUserIDMutex sync.RWMutex
)

func initializeInmemmoryDB() error {
	rows, err := db.Query("SELECT user_id, succeeded FROM login_log ORDER BY id")
	if err != nil {
		return err
	}
	for rows.Next() {
		var id, succeed int
		rows.Scan(&id, &succeed)
		LoginLogDBMutex.Lock()
		if succeed == 1 {
			LoginLogDB[id] = 0
		} else {
			LoginLogDB[id]++
		}
		LoginLogDBMutex.Unlock()
	}
	rows, err = db.Query(
		"SELECT ip, t0.cnt FROM " +
			"(SELECT ip, MAX(succeeded) as max_succeeded, COUNT(1) as cnt FROM login_log GROUP BY ip) " +
			"AS t0 WHERE t0.max_succeeded = 0")
	if err != nil {
		return err
	}
	for rows.Next() {
		var count int
		var ip string
		rows.Scan(&ip, &count)
		LoginLogDBIndexIPMutex.Lock()
		LoginLogDBIndexIP[ip] = count
		LoginLogDBIndexIPMutex.Unlock()
	}

	rows, err = db.Query(
		"SELECT id, login, password_hash, salt FROM users")
	for rows.Next() {
		u := User{}
		rows.Scan(&u.ID, &u.Login, &u.PasswordHash, &u.Salt)
		UserNameDB[u.ID] = u.Login

		UserDBIndexID[u.ID] = &u
		UserDBIndexlogin[u.Login] = &u
	}

	rows, err = db.Query(
		"SELECT login, ip, created_at, user_id FROM login_log WHERE succeeded = 1 ORDER BY id ASC",
	)

	if err != nil {
		return nil
	}

	defer rows.Close()
	for rows.Next() {
		l := &LastLogin{}
		var userId int
		err = rows.Scan(&l.Login, &l.IP, &l.CreatedAt, &userId)
		if err != nil {
			l = nil
		}
		LastLoginDBIndexUserID[userId][0], LastLoginDBIndexUserID[userId][1], l = LastLoginDBIndexUserID[userId][1], l, LastLoginDBIndexUserID[userId][0]
	}

	return nil
}

func createLoginLog(succeeded bool, remoteAddr, login string, user *User) error {
	if succeeded {
		go func(){
			LastLoginDBIndexUserIDMutex.Lock()
			LastLoginDBIndexUserID[user.ID][0], LastLoginDBIndexUserID[user.ID][1] = LastLoginDBIndexUserID[user.ID][1], &LastLogin{login, remoteAddr, time.Now()}
			LastLoginDBIndexUserIDMutex.Unlock()
		}()
		go func(){
			LoginLogDBMutex.Lock()
			LoginLogDB[user.ID] = 0
			LoginLogDBMutex.Unlock()
		}()
		go func(){
			LoginLogDBIndexIPMutex.Lock()
			LoginLogDBIndexIP[remoteAddr] = 0
			LoginLogDBIndexIPMutex.Unlock()
		}()
	}else{
		if user != nil {
			go func(){
				LoginLogDBMutex.Lock()
				LoginLogDB[user.ID]++
				LoginLogDBMutex.Unlock()
			}()
		}
		go func(){
			LoginLogDBIndexIPMutex.Lock()
			LoginLogDBIndexIP[remoteAddr]++
			LoginLogDBIndexIPMutex.Unlock()
		}()
	}

/*
	if succeeded {
		LastLoginDBIndexUserIDMutex.Lock()
		LastLoginDBIndexUserID[user.ID][0], LastLoginDBIndexUserID[user.ID][1] = LastLoginDBIndexUserID[user.ID][1], &LastLogin{login, remoteAddr, time.Now()}
		LastLoginDBIndexUserIDMutex.Unlock()
	}

	LoginLogDBMutex.Lock()
	LoginLogDBIndexIPMutex.Lock()
	if succeeded {
		LoginLogDB[user.ID] = 0
		LoginLogDBIndexIP[remoteAddr] = 0
	} else {
		if user != nil {
			LoginLogDB[user.ID]++
		}
		LoginLogDBIndexIP[remoteAddr]++
	}
	LoginLogDBMutex.Unlock()
	LoginLogDBIndexIPMutex.Unlock()
*/
	return nil
}

func isLockedUser(user *User) (bool, error) {
	if user == nil {
		return false, nil
	}
	LoginLogDBMutex.RLock()
	defer LoginLogDBMutex.RUnlock()
	return UserLockThreshold <= LoginLogDB[user.ID], nil
}

func isBannedIP(ip string) (bool, error) {
	LoginLogDBIndexIPMutex.RLock()
	v, ok := LoginLogDBIndexIP[ip]
	LoginLogDBIndexIPMutex.RUnlock()
	return ok && IPBanThreshold <= v, nil
}

func attemptLogin(req *http.Request) (*User, error) {
	//succeeded := false
	user := &User{}

	loginName := req.PostFormValue("login")
	password := req.PostFormValue("password")

	remoteAddr := req.RemoteAddr
	if xForwardedFor := req.Header.Get("X-Forwarded-For"); len(xForwardedFor) > 0 {
		remoteAddr = xForwardedFor
	}
/*
	defer func() {
		createLoginLog(succeeded, remoteAddr, loginName, user)
	}()
*/
	user = UserDBIndexlogin[loginName]

	var g errgroup.Group

	g.Go(func() error {
		if banned, _ := isBannedIP(remoteAddr); banned {
			return ErrBannedIP
		}
		return nil
	})

	g.Go(func() error {
		if locked, _ := isLockedUser(user); locked {
			return ErrLockedUser
		}
		return nil
	})

	g.Go(func() error {
		if user == nil {
			return ErrUserNotFound
		}
		return nil
	})

	g.Go(func() error {
		if user.PasswordHash != calcPassHash(password, user.Salt) {
			return ErrWrongPassword
		}
		return nil
	})

	if err := g.Wait(); err != nil {
		go createLoginLog(false, remoteAddr, loginName, user)
		return nil, err
	}

	//succeeded = true
	go createLoginLog(true, remoteAddr, loginName, user)
	return user, nil
}

func getCurrentUser(userId interface{}) *User {
	v, _ := userId.(string)
	id, _ := strconv.Atoi(v)
	return UserDBIndexID[id]
}

func bannedIPs() []string {
	ips := []string{}
	for ip, count := range LoginLogDBIndexIP {
		if IPBanThreshold <= count {
			ips = append(ips, ip)
		}
	}

	return ips
}

func lockedUsers() []string {
	userIds := []string{}
	for id, count := range LoginLogDB {
		if UserLockThreshold <= count {
			userIds = append(userIds, UserNameDB[id])
		}
	}

	return userIds
}
