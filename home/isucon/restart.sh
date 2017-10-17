cp /var/log/mysql-slow.sql /var/log.mysql-slow.sql.old
cp /var/log/nginx/access.log /var/log/nginx/access.log.old
cp /var/log/nginx/error.log /var/log/nginx/error.log.old
> /var/log/mysql-slow.sql
> /var/log/nginx/access.log
> /var/log/nginx/error.log 
> /tmp/isucon.go.log

service nginx restart
service mysqld restart

echo 'start benchmark'
output=`/home/isucon/benchmarker bench`
echo ${output:1000}

mysqldumpslow -s t /var/log/mysql-slow.sql > /home/isucon/log/mysqlslowdump.log
cat /var/log/nginx/access.log | /opt/kataribe -conf /opt/kataribe.toml > /home/isucon/log/kataribe.log
cp /tmp/isucon.go.log /home/isucon/log/isucon.go.log

git reset
git add /home/isucon/log
git commit -m 'Run benchmark' -m "${output:140}"
