package main

import (
	"time"
)

type User struct {
	ID           int
	Login        string
	PasswordHash string
	Salt         string

	LastLogin *LastLogin
}

type LastLogin struct {
	Login     string
	IP        string
	CreatedAt time.Time
}

func (u *User) getLastLogin() *LastLogin {
	LastLoginDBIndexUserIDMutex.RLock()
	lastLogin0 := LastLoginDBIndexUserID[u.ID][0]
	lastLogin1 := LastLoginDBIndexUserID[u.ID][1]
	LastLoginDBIndexUserIDMutex.RUnlock()
	if lastLogin0 == nil {
		u.LastLogin = lastLogin1
		return lastLogin1
	}
	u.LastLogin = lastLogin0
	return lastLogin0
	/*
		rows, err := db.Query(
			"SELECT login, ip, created_at FROM login_log WHERE succeeded = 1 AND user_id = ? ORDER BY id DESC LIMIT 2",
			u.ID,
		)

		if err != nil {
			return nil
		}

		defer rows.Close()
		for rows.Next() {
			u.LastLogin = &LastLogin{}
			err = rows.Scan(&u.LastLogin.Login, &u.LastLogin.IP, &u.LastLogin.CreatedAt)
			if err != nil {
				u.LastLogin = nil
				return nil
			}
		}

		return u.LastLogin
	*/
}
