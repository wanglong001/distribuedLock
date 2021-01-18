package distribuedLock

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	uuid "github.com/satori/go.uuid"
)

type IMutex interface {
	Lock(lockName string) error
	UnLock(lockName string) error
	RenewLock(lockName string) bool
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

// RedisMutex
type RedisMutex struct {
	RedisClient   *redis.Client
	Ctx           context.Context
	interval      time.Duration
	timeLock      time.Duration
	verifyCodeMap sync.Map
}

func (l *RedisMutex) Init(Addr, Password string, DB int, timeLock, interval time.Duration) error {
	l.RedisClient = redis.NewClient(&redis.Options{
		Addr:     Addr,
		Password: Password, // no password set
		DB:       DB,       // use default DB
	})
	l.Ctx = context.Background()
	l.timeLock = timeLock
	l.interval = interval
	return nil
}

func (l *RedisMutex) Lock(lockName string) error {
	fmt.Println("lock......")
	n, err := l.RedisClient.Exists(l.Ctx, lockName).Result()
	if err != nil {
		return err
	}
	if n > 0 {
		return errors.New("Failed to grab lock")
	}
	verifyCode := uuid.NewV4().String()
	ok, err := l.RedisClient.SetNX(l.Ctx, lockName, verifyCode, l.timeLock).Result()
	if err != nil {
		if err == redis.Nil {
			return errors.New("Failed to grab lock")
		}
		return err
	}
	if !ok {
		return errors.New("Failed to grab lock")
	}
	l.verifyCodeMap.Store(lockName, verifyCode)
	return nil
}

func (l *RedisMutex) UnLock(lockName string) error {
	fmt.Println("unlock......")
	v, ok := l.verifyCodeMap.Load(lockName)
	if !ok {
		return fmt.Errorf("lock: %s is expire1", lockName)
	}
	redisV, err := l.RedisClient.Get(l.Ctx, lockName).Result()
	if err != nil {
		return fmt.Errorf("lock: %s is expire2, %s", lockName, err)
	}
	if v != redisV {
		l.verifyCodeMap.Delete(lockName)
		return fmt.Errorf("lock: %s is expire3, %s, %s", lockName, v, redisV)
	}
	_, err = l.RedisClient.Del(l.Ctx, lockName).Result()
	l.verifyCodeMap.Delete(lockName)
	if err != nil {
		return err
	}
	return nil
}

func (l *RedisMutex) RenewLock(lockName string) bool {
	fmt.Println("renew lock......")
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
	l.RedisClient.SetEX(l.Ctx, lockName, redisV, l.timeLock).Result()
	return true
}

func (l *RedisMutex) GetInterval() time.Duration {
	fmt.Println("GetInterval......")
	return l.interval
}

//TODO RedLock EtcdMutex

// TODO EtcdMutex
// type EtcdMutex struct {
// }

// func (l *EtcdMutex) Lock(lockName string) error {
// 	fmt.Println("lock......")
// 	return nil
// }

// func (l *EtcdMutex) UnLock(lockName string) error {
// 	fmt.Println("unlock......")
// 	return nil
// }

// func (l *EtcdMutex) RenewLock(lockName string) bool {
// 	fmt.Println("unlock......")
// 	return nil
// }

// func (l *EtcdMutex) GetInterval() time.Duration {
// 	fmt.Println("GetInterval......")
// 	return time.Second * 1
// }
