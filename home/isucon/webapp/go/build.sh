#!/bin/bash
sudo supervisorctl stop isucon_go

go get github.com/go-martini/martini
go get github.com/go-sql-driver/mysql
go get github.com/martini-contrib/render
go get github.com/martini-contrib/sessions
go build -o golang-webapp .

sudo supervisorctl start isucon_go
