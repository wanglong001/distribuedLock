# 分布式锁
redis 是 AP 模型， 需要定一致性的场景最好使用 etcd

* [x] redis setnx
* [ ] redis redlock
* [x] etcd

## 环境
go 1.5

## Example

```golang


// REDIS
addr := "localhost:6379"
password := ""
db := 0 
ttl := 2*time.Second
interval := 1*time.Second

lockName := "redis"

redisLock := &RedisMutex{}
redisLock.Init(addr, password, db, ttl, interval)
managerLock := &ManagerMutex{IMutex: redisLock}

err := managerLock.Lock(lockName)
if err != nil {
    ....
}
managerLock.RenewLock(lockName) // if need to renew lock
do_your_work()

err := managerLock.UnLock(lockName)
if err != nil {
    ...
    // network error, lock expire
    // rollback
}

// ETCD
etcdMutex := &EtcdMutex{}
etcdMutex.Init([]string{"0.0.0.0:2379"}, 2*time.Second)
managerLock := &ManagerMutex{IMutex: etcdMutex}

err := managerLock.Lock(lockName)
if err != nil {
    ....
}
do_your_work()

err := managerLock.UnLock(lockName)
if err != nil {
    ...
    // network error, lock expire
    // rollback
}

```