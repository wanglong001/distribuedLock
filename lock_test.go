package distribuedLock

import (
	"fmt"
	"math/rand"
	"testing"
	"time"
)

func Test_RedisLock(t *testing.T) {
	count := 0
	redisLock := &RedisMutex{}
	redisLock.Init("localhost:6379", "", 0, 2*time.Second, 1*time.Second)
	managerLock := &ManagerMutex{IMutex: redisLock}
	redisLock.RedisClient.Del(redisLock.Ctx, "redis").Result()

	for i := 0; i < 3; i++ {
		go func(i int) {
			for {
				err := managerLock.Lock("redis")
				interval := time.Duration(rand.Int31n(10)) * time.Second
				if err != nil {
					time.Sleep(interval)
					continue
				}
				managerLock.RenewLock("redis")
				fmt.Println("Sleep: ", interval)
				count++
				t.Log("goruntime id: ", i, "count: ", count)
				time.Sleep(interval)
				err = managerLock.UnLock("redis")
				time.Sleep(interval)
				if err != nil {
					continue
				}

			}
		}(i)
	}
	for count <= 100000 {

	}
}
