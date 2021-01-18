# 分布式锁
redis 是 AP 模型， 需要定一致性的场景最好使用 etcd

* [x] redis setnx
* [ ] redis redlock
* [ ] etcd

## redis

```golang


addr := "localhost:6379"
password := ""
db := 0 
timeLock := 2*time.Second
interval := 1*time.Second

lockName := "redis"

redisLock := &RedisMutex{}
redisLock.Init(addr, password, db, timeLock, interval)
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


```