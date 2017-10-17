cp /var/log/mysql-slow.sql /var/log.mysql-slow.sql.old
cp /var/log/nginx/access.log /var/log/nginx/access.log.old
cp /var/log/nginx/error.log /var/log/nginx/error.log.old
> /var/log/mysql-slow.sql
> /var/log/nginx/access.log
> /var/log/nginx/error.log 

service nginx restart
service mysqld restart

output=`/home/isucon/benchmarker bench`

mysqldumpslow -s t /var/log/mysql-slow.sql > /home/isucon/log/mysqlslowdump.log
cat /var/log/nginx/access.log | kataribe -conf /opt/kataribe.toml > /home/isucon/log/kataribe.log

git reset
git add /home/isucon/log
git commit -m 'Run benchmark' -m "$output"
