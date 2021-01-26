package distribuedLock

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/go-redis/redis/v8"
	uuid "github.com/satori/go.uuid"
	"go.etcd.io/etcd/clientv3/concurrency"
)

var ErrLocked = errors.New("mutex: Locked by another session")
var ErrSessionExpired = errors.New("mutex: session is expired")

type IMutex interface {
	Lock(lockName string) error
	UnLock(lockName string) error
	RenewLock(lockName string) bool
	Close()
	GetInterval() time.Duration // how long to renew lock
}

// ManagerMutex
type ManagerMutex struct {
	IMutex IMutex
}

func (l *ManagerMutex) UnLock(lockName string) error {
	return l.IMutex.UnLock(lockName)
}

func (l *ManagerMutex) Lock(lockName string) error {
	return l.IMutex.Lock(lockName)
}

func (l *ManagerMutex) RenewLock(lockName string) error {
	go func() {
		for {
			isContinue := l.IMutex.RenewLock(lockName)
			if !isContinue {
				time.Sleep(l.IMutex.GetInterval())
				break
			}
			time.Sleep(l.IMutex.GetInterval())
		}
	}()
	return nil
}

func (l *ManagerMutex) Close() {
	l.IMutex.Close()
}

// RedisMutex
type RedisMutex struct {
	RedisClient   *redis.Client
	Ctx           context.Context
	interval      time.Duration
	ttl           time.Duration
	verifyCodeMap sync.Map
}

func (l *RedisMutex) Init(Addr, Password string, DB int, ttl time.Duration) error {
	l.RedisClient = redis.NewClient(&redis.Options{
		Addr:     Addr,
		Password: Password, // no password set
		DB:       DB,       // use default DB
	})
	l.Ctx = context.Background()
	l.ttl = ttl
	l.interval = time.Duration(ttl/4*10) * time.Second
	return nil
}

func (l *RedisMutex) Lock(lockName string) error {
	n, err := l.RedisClient.Exists(l.Ctx, lockName).Result()
	if err != nil {
		return err
	}
	if n > 0 {
		return ErrLocked
	}
	verifyCode := uuid.NewV4().String()
	ok, err := l.RedisClient.SetNX(l.Ctx, lockName, verifyCode, l.ttl).Result()
	if err != nil {
		if err == redis.Nil {
			return ErrLocked
		}
		return err
	}
	if !ok {
		return ErrLocked
	}
	l.verifyCodeMap.Store(lockName, verifyCode)
	return nil
}

func (l *RedisMutex) UnLock(lockName string) error {
	v, ok := l.verifyCodeMap.Load(lockName)
	if !ok {
		return ErrSessionExpired
	}
	redisV, err := l.RedisClient.Get(l.Ctx, lockName).Result()
	if err != nil {
		return ErrSessionExpired
	}
	if v != redisV {
		l.verifyCodeMap.Delete(lockName)
		return ErrSessionExpired
	}
	_, err = l.RedisClient.Del(l.Ctx, lockName).Result()
	l.verifyCodeMap.Delete(lockName)
	if err != nil {
		return err
	}
	return nil
}

func (l *RedisMutex) RenewLock(lockName string) bool {
	v, ok := l.verifyCodeMap.Load(lockName)
	if !ok {
		return false
	}
	redisV, err := l.RedisClient.Get(l.Ctx, lockName).Result()
	if err != nil {
		return false
	}
	if v != redisV {
		return false
	}
	l.RedisClient.SetEX(l.Ctx, lockName, redisV, l.ttl).Result()
	return true
}

func (l *RedisMutex) GetInterval() time.Duration {
	return l.interval
}

func (l *RedisMutex) Close() {
	l.RedisClient.Close()
}

//TODO RedLock EtcdMutex

// TODO EtcdMutex
type EtcdMutex struct {
	EtcdClient *clientv3.Client
	mutexMap   sync.Map
	interval   time.Duration
	ttl        time.Duration
}

func (l *EtcdMutex) Init(endpoints []string, ttl time.Duration) error {
	var err error
	l.EtcdClient, err = clientv3.New(clientv3.Config{Endpoints: endpoints})
	if err != nil {
		return err
	}
	l.ttl = ttl
	l.interval = time.Duration(ttl/4*10) * time.Second
	return nil
}

func (l *EtcdMutex) Close() {
	l.EtcdClient.Close()
}

func (l *EtcdMutex) Lock(lockName string) error {
	sessionOption := concurrency.WithTTL(int(l.ttl.Seconds()))
	s, _ := concurrency.NewSession(l.EtcdClient, sessionOption)
	mutex := concurrency.NewMutex(s, fmt.Sprintf("/distributedLock/%s", lockName))
	timeout, cancelFunc := context.WithTimeout(context.Background(), l.ttl)
	defer cancelFunc()
	err := mutex.Lock(timeout)
	if err != nil {
		s.Close()
		return ErrLocked
	}
	l.mutexMap.Store(lockName, mutex)
	return nil
}

func (l *EtcdMutex) UnLock(lockName string) error {
	mutex, ok := l.mutexMap.LoadAndDelete(lockName)
	if !ok {
		return ErrSessionExpired
	}
	timeout, cancelFunc := context.WithTimeout(context.Background(), l.ttl)
	defer cancelFunc()
	if err := mutex.(*concurrency.Mutex).Unlock(timeout); err != nil {
		return err
	}
	return nil
}

func (l *EtcdMutex) RenewLock(lockName string) bool {
	// etcd mutex, they has do renew:
	//		 https://github.com/etcd-io/etcd/blob/92458228e1268e9b78e11abeeb255824b44d0b2f/client/v3/concurrency/session.go#L64
	return false
}

func (l *EtcdMutex) GetInterval() time.Duration {
	return l.interval
}
