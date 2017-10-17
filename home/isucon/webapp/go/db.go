package main

import (
//	"database/sql"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"sync"
	"time"
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
	//rows, err = db.Query("SELECT ip, count(1) FROM login_log GROUP BY ip")
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
		//LastLoginDBIndexUserID[userId][1], LastLoginDBIndexUserID[userId][0], l = LastLoginDBIndexUserID[userId][0], l, LastLoginDBIndexUserID[userId][1]
		LastLoginDBIndexUserID[userId][0], LastLoginDBIndexUserID[userId][1], l = LastLoginDBIndexUserID[userId][1], l, LastLoginDBIndexUserID[userId][0]
	}

	return nil
}

func createLoginLog(succeeded bool, remoteAddr, login string, user *User) error {
	/*
		succ := 0
		if succeeded {
			succ = 1
		}
	*/
	/*
	var userId sql.NullInt64
	if user != nil {
		userId.Int64 = int64(user.ID)
		userId.Valid = true
	}

		_, err := db.Exec(
			"INSERT INTO login_log (`created_at`, `user_id`, `login`, `ip`, `succeeded`) "+
				"VALUES (?,?,?,?,?)",
			time.Now(), userId, login, remoteAddr, succ,
		)
		_=err
	*/
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

	return nil
}

func isLockedUser(user *User) (bool, error) {
	if user == nil {
		return false, nil
	}

	/*
		var ni sql.NullInt64
		row := db.QueryRow(
			"SELECT COUNT(1) AS failures FROM login_log WHERE "+
				"user_id = ? AND id > IFNULL((select id from login_log where user_id = ? AND "+
				"succeeded = 1 ORDER BY id DESC LIMIT 1), 0);",
			user.ID, user.ID,
		)
		err := row.Scan(&ni)

		switch {
		case err == sql.ErrNoRows:
			return false, nil
		case err != nil:
			return false, err
		}
	*/
	LoginLogDBMutex.RLock()
	defer LoginLogDBMutex.RUnlock()
	return UserLockThreshold <= LoginLogDB[user.ID], nil

	//return UserLockThreshold <= int(ni.Int64), nil
}

func isBannedIP(ip string) (bool, error) {
	/*
		var ni sql.NullInt64
		row := db.QueryRow(
			"SELECT COUNT(1) AS failures FROM login_log WHERE "+
				"ip = ? AND id > IFNULL((select id from login_log where ip = ? AND "+
				"succeeded = 1 ORDER BY id DESC LIMIT 1), 0);",
			ip, ip,
		)
		err := row.Scan(&ni)

		switch {
		case err == sql.ErrNoRows:
			return false, nil
		case err != nil:
			return false, err
		}
		return IPBanThreshold <= int(ni.Int64), nil
	*/
	LoginLogDBIndexIPMutex.RLock()
	defer LoginLogDBIndexIPMutex.RUnlock()
	v, ok := LoginLogDBIndexIP[ip]
	return ok && IPBanThreshold <= v, nil
}

func attemptLogin(req *http.Request) (*User, error) {
	succeeded := false
	user := &User{}

	loginName := req.PostFormValue("login")
	password := req.PostFormValue("password")

	remoteAddr := req.RemoteAddr
	if xForwardedFor := req.Header.Get("X-Forwarded-For"); len(xForwardedFor) > 0 {
		remoteAddr = xForwardedFor
	}

	defer func() {
		createLoginLog(succeeded, remoteAddr, loginName, user)
	}()

	/*
		row := db.QueryRow(
			"SELECT id, login, password_hash, salt FROM users WHERE login = ?",
			loginName,
		)
		err := row.Scan(&user.ID, &user.Login, &user.PasswordHash, &user.Salt)
		switch {
		case err == sql.ErrNoRows:
			user = nil
		case err != nil:
			return nil, err
		}
	*/
	user = UserDBIndexlogin[loginName]

	if banned, _ := isBannedIP(remoteAddr); banned {
		return nil, ErrBannedIP
	}

	if locked, _ := isLockedUser(user); locked {
		return nil, ErrLockedUser
	}

	if user == nil {
		return nil, ErrUserNotFound
	}

	if user.PasswordHash != calcPassHash(password, user.Salt) {
		return nil, ErrWrongPassword
	}

	succeeded = true
	return user, nil
}

func getCurrentUser(userId interface{}) *User {
	v, _ := userId.(string)
	id, _ := strconv.Atoi(v)
	return UserDBIndexID[id]
	/*
		user := &User{}
		row := db.QueryRow(
			"SELECT id, login, password_hash, salt FROM users WHERE id = ?",
			userId,
		)
		err := row.Scan(&user.ID, &user.Login, &user.PasswordHash, &user.Salt)

		if err != nil {
			return nil
		}

		return user
	*/
}

func bannedIPs() []string {
	//ips := []string{}
	ips := sort.StringSlice{}

	/*
		rows, err := db.Query(
			"SELECT ip FROM "+
				"(SELECT ip, MAX(succeeded) as max_succeeded, COUNT(1) as cnt FROM login_log GROUP BY ip) "+
				"AS t0 WHERE t0.max_succeeded = 0 AND t0.cnt >= ?",
			IPBanThreshold,
		)

		if err != nil {
			return ips
		}

		defer rows.Close()
		for rows.Next() {
			var ip string

			if err := rows.Scan(&ip); err != nil {
				return ips
			}
			ips = append(ips, ip)
		}
		if err := rows.Err(); err != nil {
			return ips
		}

		rowsB, err := db.Query(
			"SELECT ip, MAX(id) AS last_login_id FROM login_log WHERE succeeded = 1 GROUP by ip",
		)

		if err != nil {
			return ips
		}

		defer rowsB.Close()
		for rowsB.Next() {
			var ip string
			var lastLoginId int

			if err := rows.Scan(&ip, &lastLoginId); err != nil {
				return ips
			}

			var count int

			err = db.QueryRow(
				"SELECT COUNT(1) AS cnt FROM login_log WHERE ip = ? AND ? < id",
				ip, lastLoginId,
			).Scan(&count)

			if err != nil {
				return ips
			}

			if IPBanThreshold <= count {
				ips = append(ips, ip)
			}
		}
		if err := rowsB.Err(); err != nil {
			return ips
		}
	*/
	for ip, count := range LoginLogDBIndexIP {
		if IPBanThreshold <= count {
			ips = append(ips, ip)
		}
	}

	ips.Sort()
	return ips
}

func lockedUsers() []string {
	userIds := []string{}

	/*
		rows, err := db.Query(
			"SELECT user_id, login FROM "+
				"(SELECT user_id, login, MAX(succeeded) as max_succeeded, COUNT(1) as cnt FROM login_log GROUP BY user_id) "+
				"AS t0 WHERE t0.user_id IS NOT NULL AND t0.max_succeeded = 0 AND t0.cnt >= ?",
			UserLockThreshold,
		)

		if err != nil {
			return userIds
		}

		defer rows.Close()
		for rows.Next() {
			var userId int
			var login string

			if err := rows.Scan(&userId, &login); err != nil {
				return userIds
			}
			userIds = append(userIds, login)
		}
		if err := rows.Err(); err != nil {
			return userIds
		}

		rowsB, err := db.Query(
			"SELECT user_id, login, MAX(id) AS last_login_id FROM login_log WHERE user_id IS NOT NULL AND succeeded = 1 GROUP BY user_id",
		)

		if err != nil {
			return userIds
		}

		defer rowsB.Close()
		for rowsB.Next() {
			var userId int
			var login string
			var lastLoginId int

			if err := rowsB.Scan(&userId, &login, &lastLoginId); err != nil {
				return userIds
			}

			var count int

			err = db.QueryRow(
				"SELECT COUNT(1) AS cnt FROM login_log WHERE user_id = ? AND ? < id",
				userId, lastLoginId,
			).Scan(&count)

			if err != nil {
				return userIds
			}

			if UserLockThreshold <= count {
				userIds = append(userIds, login)
			}
		}
		if err := rowsB.Err(); err != nil {
			return userIds
		}

	*/
	for id, count := range LoginLogDB {
		if UserLockThreshold <= count {
			userIds = append(userIds, UserNameDB[id])
		}
	}

	return userIds
}
