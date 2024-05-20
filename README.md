go-redisutils
==============

Go Redis utils.

[![GoDoc](https://godoc.org/github.com/koofr/go-redisutils?status.png)](https://godoc.org/github.com/koofr/go-redisutils)

## Install

```sh
go get github.com/koofr/go-redisutils
```

## Testing

```sh
docker run --rm -it -p 26379:6379 redis
```

```sh
export REDIS_HOST=localhost
export REDIS_PORT=26379

go test
```
