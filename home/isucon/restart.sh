> /var/log/mysql-slow.sql
> /var/log/nginx/access.log
> /var/log/nginx/error.log 

service nginx restart
service mysqld restart

/home/isucon/benchmarker bench

mysqldumpslow -s t /var/log/mysql-slow.sql > /home/isucon/log/mysqlslowdump.log
