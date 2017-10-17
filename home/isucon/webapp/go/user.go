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
	LastLoginDBIndexUserIDMutex.RUnlock()
	if lastLogin0 != nil {
		u.LastLogin = lastLogin0
		return lastLogin0
	}
	LastLoginDBIndexUserIDMutex.RLock()
	lastLogin1 := LastLoginDBIndexUserID[u.ID][1]
	LastLoginDBIndexUserIDMutex.RUnlock()
	u.LastLogin = lastLogin1
	return lastLogin1
}
