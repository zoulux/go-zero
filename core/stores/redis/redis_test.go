package redis

import (
	"crypto/tls"
	"errors"
	"io"
	"strconv"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	red "github.com/go-redis/redis"
	"github.com/stretchr/testify/assert"
)

func TestRedis_Exists(t *testing.T) {
	runOnRedis(t, func(client *Redis) {
		ok, err := client.Exists("a")
		assert.Nil(t, err)
		assert.False(t, ok)
		assert.Nil(t, client.Set("a", "b"))
		ok, err = client.Exists("a")
		assert.Nil(t, err)
		assert.True(t, ok)
	})
}

func TestRedisTLS_Exists(t *testing.T) {
	runOnRedisTLS(t, func(client *Redis) {
		_, err := New(client.Addr, WithTLS()).Exists("a")
		assert.NotNil(t, err)
		ok, err := client.Exists("a")
		assert.NotNil(t, err)
		assert.False(t, ok)
		assert.NotNil(t, client.Set("a", "b"))
		ok, err = client.Exists("a")
		assert.NotNil(t, err)
		assert.False(t, ok)
	})
}

func TestRedis_Eval(t *testing.T) {
	runOnRedis(t, func(client *Redis) {
		_, err := client.Eval(`redis.call("EXISTS", KEYS[1])`, []string{"notexist"})
		assert.Equal(t, Nil, err)
		err = client.Set("key1", "value1")
		assert.Nil(t, err)
		_, err = client.Eval(`redis.call("EXISTS", KEYS[1])`, []string{"key1"})
		assert.Equal(t, Nil, err)
		val, err := client.Eval(`return redis.call("EXISTS", KEYS[1])`, []string{"key1"})
		assert.Nil(t, err)
		assert.Equal(t, int64(1), val)
	})
}

func TestRedis_GeoHash(t *testing.T) {
	runOnRedis(t, func(client *Redis) {
		_, err := client.GeoHash("parent", "child1", "child2")
		assert.NotNil(t, err)
	})
}

func TestRedis_Hgetall(t *testing.T) {
	runOnRedis(t, func(client *Redis) {
		assert.Nil(t, client.Hset("a", "aa", "aaa"))
		assert.Nil(t, client.Hset("a", "bb", "bbb"))
		vals, err := client.Hgetall("a")
		assert.Nil(t, err)
		assert.EqualValues(t, map[string]string{
			"aa": "aaa",
			"bb": "bbb",
		}, vals)
	})
}

func TestRedis_Hvals(t *testing.T) {
	runOnRedis(t, func(client *Redis) {
		assert.Nil(t, client.Hset("a", "aa", "aaa"))
		assert.Nil(t, client.Hset("a", "bb", "bbb"))
		vals, err := client.Hvals("a")
		assert.Nil(t, err)
		assert.ElementsMatch(t, []string{"aaa", "bbb"}, vals)
	})
}

func TestRedis_Hsetnx(t *testing.T) {
	runOnRedis(t, func(client *Redis) {
		assert.Nil(t, client.Hset("a", "aa", "aaa"))
		assert.Nil(t, client.Hset("a", "bb", "bbb"))
		ok, err := client.Hsetnx("a", "bb", "ccc")
		assert.Nil(t, err)
		assert.False(t, ok)
		ok, err = client.Hsetnx("a", "dd", "ddd")
		assert.Nil(t, err)
		assert.True(t, ok)
		vals, err := client.Hvals("a")
		assert.Nil(t, err)
		assert.ElementsMatch(t, []string{"aaa", "bbb", "ddd"}, vals)
	})
}

func TestRedis_HdelHlen(t *testing.T) {
	runOnRedis(t, func(client *Redis) {
		assert.Nil(t, client.Hset("a", "aa", "aaa"))
		assert.Nil(t, client.Hset("a", "bb", "bbb"))
		num, err := client.Hlen("a")
		assert.Nil(t, err)
		assert.Equal(t, 2, num)
		val, err := client.Hdel("a", "aa")
		assert.Nil(t, err)
		assert.True(t, val)
		vals, err := client.Hvals("a")
		assert.Nil(t, err)
		assert.ElementsMatch(t, []string{"bbb"}, vals)
	})
}

func TestRedis_HIncrBy(t *testing.T) {
	runOnRedis(t, func(client *Redis) {
		val, err := client.Hincrby("key", "field", 2)
		assert.Nil(t, err)
		assert.Equal(t, 2, val)
		val, err = client.Hincrby("key", "field", 3)
		assert.Nil(t, err)
		assert.Equal(t, 5, val)
	})
}

func TestRedis_Hkeys(t *testing.T) {
	runOnRedis(t, func(client *Redis) {
		assert.Nil(t, client.Hset("a", "aa", "aaa"))
		assert.Nil(t, client.Hset("a", "bb", "bbb"))
		vals, err := client.Hkeys("a")
		assert.Nil(t, err)
		assert.ElementsMatch(t, []string{"aa", "bb"}, vals)
	})
}

func TestRedis_Hmget(t *testing.T) {
	runOnRedis(t, func(client *Redis) {
		assert.Nil(t, client.Hset("a", "aa", "aaa"))
		assert.Nil(t, client.Hset("a", "bb", "bbb"))
		vals, err := client.Hmget("a", "aa", "bb")
		assert.Nil(t, err)
		assert.EqualValues(t, []string{"aaa", "bbb"}, vals)
		vals, err = client.Hmget("a", "aa", "no", "bb")
		assert.Nil(t, err)
		assert.EqualValues(t, []string{"aaa", "", "bbb"}, vals)
	})
}

func TestRedis_Hmset(t *testing.T) {
	runOnRedis(t, func(client *Redis) {
		assert.NotNil(t, NewRedis(client.Addr, "").Hmset("a", nil))
		assert.Nil(t, client.Hmset("a", map[string]string{
			"aa": "aaa",
			"bb": "bbb",
		}))
		vals, err := client.Hmget("a", "aa", "bb")
		assert.Nil(t, err)
		assert.EqualValues(t, []string{"aaa", "bbb"}, vals)
	})
}

func TestRedis_Hscan(t *testing.T) {
	runOnRedis(t, func(client *Redis) {
		key := "hash:test"
		fieldsAndValues := make(map[string]string)
		for i := 0; i < 1550; i++ {
			fieldsAndValues["filed_"+strconv.Itoa(i)] = randomStr(i)
		}
		err := client.Hmset(key, fieldsAndValues)
		assert.Nil(t, err)

		var cursor uint64 = 0
		sum := 0
		for {
			reMap, next, err := client.Hscan(key, cursor, "*", 100)
			assert.Nil(t, err)
			sum += len(reMap)
			if next == 0 {
				break
			}
			cursor = next
		}

		assert.Equal(t, sum, 3100)
		_, err = client.Del(key)
		assert.Nil(t, err)
	})
}

func TestRedis_Incr(t *testing.T) {
	runOnRedis(t, func(client *Redis) {
		val, err := client.Incr("a")
		assert.Nil(t, err)
		assert.Equal(t, int64(1), val)
		val, err = client.Incr("a")
		assert.Nil(t, err)
		assert.Equal(t, int64(2), val)
	})
}

func TestRedis_IncrBy(t *testing.T) {
	runOnRedis(t, func(client *Redis) {
		val, err := client.Incrby("a", 2)
		assert.Nil(t, err)
		assert.Equal(t, int64(2), val)
		val, err = client.Incrby("a", 3)
		assert.Nil(t, err)
		assert.Equal(t, int64(5), val)
	})
}

func TestRedis_Keys(t *testing.T) {
	runOnRedis(t, func(client *Redis) {
		err := client.Set("key1", "value1")
		assert.Nil(t, err)
		err = client.Set("key2", "value2")
		assert.Nil(t, err)
		keys, err := client.Keys("*")
		assert.Nil(t, err)
		assert.ElementsMatch(t, []string{"key1", "key2"}, keys)
	})
}

func TestRedis_HyperLogLog(t *testing.T) {
	runOnRedis(t, func(client *Redis) {
		client.Ping()
		r := NewRedis(client.Addr, "")
		_, err := r.Pfadd("key1")
		assert.NotNil(t, err)
		_, err = r.Pfcount("*")
		assert.NotNil(t, err)
		err = r.Pfmerge("*")
		assert.NotNil(t, err)
	})
}

func TestRedis_List(t *testing.T) {
	runOnRedis(t, func(client *Redis) {
		val, err := client.Lpush("key", "value1", "value2")
		assert.Nil(t, err)
		assert.Equal(t, 2, val)
		val, err = client.Rpush("key", "value3", "value4")
		assert.Nil(t, err)
		assert.Equal(t, 4, val)
		val, err = client.Llen("key")
		assert.Nil(t, err)
		assert.Equal(t, 4, val)
		vals, err := client.Lrange("key", 0, 10)
		assert.Nil(t, err)
		assert.EqualValues(t, []string{"value2", "value1", "value3", "value4"}, vals)
		v, err := client.Lpop("key")
		assert.Nil(t, err)
		assert.Equal(t, "value2", v)
		val, err = client.Lpush("key", "value1", "value2")
		assert.Nil(t, err)
		assert.Equal(t, 5, val)
		v, err = client.Rpop("key")
		assert.Nil(t, err)
		assert.Equal(t, "value4", v)
		val, err = client.Rpush("key", "value4", "value3", "value3")
		assert.Nil(t, err)
		assert.Equal(t, 7, val)
		n, err := client.Lrem("key", 2, "value1")
		assert.Nil(t, err)
		assert.Equal(t, 2, n)
		vals, err = client.Lrange("key", 0, 10)
		assert.Nil(t, err)
		assert.EqualValues(t, []string{"value2", "value3", "value4", "value3", "value3"}, vals)
		n, err = client.Lrem("key", -2, "value3")
		assert.Nil(t, err)
		assert.Equal(t, 2, n)
		vals, err = client.Lrange("key", 0, 10)
		assert.Nil(t, err)
		assert.EqualValues(t, []string{"value2", "value3", "value4"}, vals)
	})
}

func TestRedis_Mget(t *testing.T) {
	runOnRedis(t, func(client *Redis) {
		err := client.Set("key1", "value1")
		assert.Nil(t, err)
		err = client.Set("key2", "value2")
		assert.Nil(t, err)
		vals, err := client.Mget("key1", "key0", "key2", "key3")
		assert.Nil(t, err)
		assert.EqualValues(t, []string{"value1", "", "value2", ""}, vals)
	})
}

func TestRedis_SetBit(t *testing.T) {
	runOnRedis(t, func(client *Redis) {
		assert.Nil(t, client.SetBit("key", 1, 1))
	})
}

func TestRedis_GetBit(t *testing.T) {
	runOnRedis(t, func(client *Redis) {
		err := client.SetBit("key", 2, 1)
		assert.Nil(t, err)
		val, err := client.GetBit("key", 2)
		assert.Nil(t, err)
		assert.Equal(t, 1, val)
	})
}

func TestRedis_BitCount(t *testing.T) {
	runOnRedis(t, func(client *Redis) {
		for i := 0; i < 11; i++ {
			err := client.SetBit("key", int64(i), 1)
			assert.Nil(t, err)
		}

		val, err := client.BitCount("key", 0, -1)
		assert.Nil(t, err)
		assert.Equal(t, int64(11), val)

		val, err = client.BitCount("key", 0, 0)
		assert.Nil(t, err)
		assert.Equal(t, int64(8), val)

		val, err = client.BitCount("key", 1, 1)
		assert.Nil(t, err)
		assert.Equal(t, int64(3), val)

		val, err = client.BitCount("key", 0, 1)
		assert.Nil(t, err)
		assert.Equal(t, int64(11), val)

		val, err = client.BitCount("key", 2, 2)
		assert.Nil(t, err)
		assert.Equal(t, int64(0), val)

	})
}

func TestRedis_BitOpAnd(t *testing.T) {
	runOnRedis(t, func(client *Redis) {
		err := client.Set("key1", "0")
		assert.Nil(t, err)
		err = client.Set("key2", "1")
		assert.Nil(t, err)
		val, err := client.BitOpAnd("destKey", "key1", "key2")
		assert.Nil(t, err)
		assert.Equal(t, int64(1), val)
		valStr, err := client.Get("destKey")
		assert.Nil(t, err)
		assert.Equal(t, "0", valStr)
	})
}

func TestRedis_BitOpNot(t *testing.T) {
	runOnRedis(t, func(client *Redis) {
		err := client.Set("key1", "\u0000")
		assert.Nil(t, err)
		val, err := client.BitOpNot("destKey", "key1")
		assert.Nil(t, err)
		assert.Equal(t, int64(1), val)
		valStr, err := client.Get("destKey")
		assert.Nil(t, err)
		assert.Equal(t, "\xff", valStr)
	})
}

func TestRedis_BitOpOr(t *testing.T) {
	runOnRedis(t, func(client *Redis) {
		err := client.Set("key1", "1")
		assert.Nil(t, err)
		err = client.Set("key2", "0")
		assert.Nil(t, err)
		val, err := client.BitOpOr("destKey", "key1", "key2")
		assert.Nil(t, err)
		assert.Equal(t, int64(1), val)
		valStr, err := client.Get("destKey")
		assert.Nil(t, err)
		assert.Equal(t, "1", valStr)
	})
}

func TestRedis_BitOpXor(t *testing.T) {
	runOnRedis(t, func(client *Redis) {
		err := client.Set("key1", "\xff")
		assert.Nil(t, err)
		err = client.Set("key2", "\x0f")
		assert.Nil(t, err)
		val, err := client.BitOpXor("destKey", "key1", "key2")
		assert.Nil(t, err)
		assert.Equal(t, int64(1), val)
		valStr, err := client.Get("destKey")
		assert.Nil(t, err)
		assert.Equal(t, "\xf0", valStr)
	})
}
func TestRedis_BitPos(t *testing.T) {
	runOnRedis(t, func(client *Redis) {
		// 11111111 11110000 00000000
		err := client.Set("key", "\xff\xf0\x00")
		assert.Nil(t, err)

		val, err := client.BitPos("key", 0, 0, 2)
		assert.Nil(t, err)
		assert.Equal(t, int64(12), val)

		val, err = client.BitPos("key", 1, 0, 2)
		assert.Nil(t, err)
		assert.Equal(t, int64(0), val)

		val, err = client.BitPos("key", 0, 1, 2)
		assert.Nil(t, err)
		assert.Equal(t, int64(12), val)

		val, err = client.BitPos("key", 1, 1, 2)
		assert.Nil(t, err)
		assert.Equal(t, int64(8), val)

		val, err = client.BitPos("key", 1, 2, 2)
		assert.Nil(t, err)
		assert.Equal(t, int64(-1), val)

	})
}

func TestRedis_Persist(t *testing.T) {
	runOnRedis(t, func(client *Redis) {
		ok, err := client.Persist("key")
		assert.Nil(t, err)
		assert.False(t, ok)
		err = client.Set("key", "value")
		assert.Nil(t, err)
		ok, err = client.Persist("key")
		assert.Nil(t, err)
		assert.False(t, ok)
		err = client.Expire("key", 5)
		assert.Nil(t, err)
		ok, err = client.Persist("key")
		assert.Nil(t, err)
		assert.True(t, ok)
		err = client.Expireat("key", time.Now().Unix()+5)
		assert.Nil(t, err)
		ok, err = client.Persist("key")
		assert.Nil(t, err)
		assert.True(t, ok)
	})
}

func TestRedis_Ping(t *testing.T) {
	runOnRedis(t, func(client *Redis) {
		ok := client.Ping()
		assert.True(t, ok)
	})
}

func TestRedis_Scan(t *testing.T) {
	runOnRedis(t, func(client *Redis) {
		err := client.Set("key1", "value1")
		assert.Nil(t, err)
		err = client.Set("key2", "value2")
		assert.Nil(t, err)
		keys, _, err := client.Scan(0, "*", 100)
		assert.Nil(t, err)
		assert.ElementsMatch(t, []string{"key1", "key2"}, keys)
	})
}

func TestRedis_Sscan(t *testing.T) {
	runOnRedis(t, func(client *Redis) {
		key := "list"
		var list []string
		for i := 0; i < 1550; i++ {
			list = append(list, randomStr(i))
		}
		lens, err := client.Sadd(key, list)
		assert.Nil(t, err)
		assert.Equal(t, lens, 1550)

		var cursor uint64 = 0
		sum := 0
		for {
			keys, next, err := client.Sscan(key, cursor, "", 100)
			assert.Nil(t, err)
			sum += len(keys)
			if next == 0 {
				break
			}
			cursor = next
		}

		assert.Equal(t, sum, 1550)
		_, err = client.Del(key)
		assert.Nil(t, err)
	})
}

func TestRedis_Set(t *testing.T) {
	runOnRedis(t, func(client *Redis) {
		num, err := client.Sadd("key", 1, 2, 3, 4)
		assert.Nil(t, err)
		assert.Equal(t, 4, num)
		val, err := client.Scard("key")
		assert.Nil(t, err)
		assert.Equal(t, int64(4), val)
		ok, err := client.Sismember("key", 2)
		assert.Nil(t, err)
		assert.True(t, ok)
		num, err = client.Srem("key", 3, 4)
		assert.Nil(t, err)
		assert.Equal(t, 2, num)
		vals, err := client.Smembers("key")
		assert.Nil(t, err)
		assert.ElementsMatch(t, []string{"1", "2"}, vals)
		members, err := client.Srandmember("key", 1)
		assert.Nil(t, err)
		assert.Len(t, members, 1)
		assert.Contains(t, []string{"1", "2"}, members[0])
		member, err := client.Spop("key")
		assert.Nil(t, err)
		assert.Contains(t, []string{"1", "2"}, member)
		vals, err = client.Smembers("key")
		assert.Nil(t, err)
		assert.NotContains(t, vals, member)
		num, err = client.Sadd("key1", 1, 2, 3, 4)
		assert.Nil(t, err)
		assert.Equal(t, 4, num)
		num, err = client.Sadd("key2", 2, 3, 4, 5)
		assert.Nil(t, err)
		assert.Equal(t, 4, num)
		vals, err = client.Sunion("key1", "key2")
		assert.Nil(t, err)
		assert.ElementsMatch(t, []string{"1", "2", "3", "4", "5"}, vals)
		num, err = client.Sunionstore("key3", "key1", "key2")
		assert.Nil(t, err)
		assert.Equal(t, 5, num)
		vals, err = client.Sdiff("key1", "key2")
		assert.Nil(t, err)
		assert.EqualValues(t, []string{"1"}, vals)
		num, err = client.Sdiffstore("key4", "key1", "key2")
		assert.Nil(t, err)
		assert.Equal(t, 1, num)
	})
}

func TestRedis_SetGetDel(t *testing.T) {
	runOnRedis(t, func(client *Redis) {
		err := client.Set("hello", "world")
		assert.Nil(t, err)
		val, err := client.Get("hello")
		assert.Nil(t, err)
		assert.Equal(t, "world", val)
		ret, err := client.Del("hello")
		assert.Nil(t, err)
		assert.Equal(t, 1, ret)
	})
}

func TestRedis_SetExNx(t *testing.T) {
	runOnRedis(t, func(client *Redis) {
		err := client.Setex("hello", "world", 5)
		assert.Nil(t, err)
		ok, err := client.Setnx("hello", "newworld")
		assert.Nil(t, err)
		assert.False(t, ok)
		ok, err = client.Setnx("newhello", "newworld")
		assert.Nil(t, err)
		assert.True(t, ok)
		val, err := client.Get("hello")
		assert.Nil(t, err)
		assert.Equal(t, "world", val)
		val, err = client.Get("newhello")
		assert.Nil(t, err)
		assert.Equal(t, "newworld", val)
		ttl, err := client.Ttl("hello")
		assert.Nil(t, err)
		assert.True(t, ttl > 0)
		ok, err = client.SetnxEx("newhello", "newworld", 5)
		assert.Nil(t, err)
		assert.False(t, ok)
		num, err := client.Del("newhello")
		assert.Nil(t, err)
		assert.Equal(t, 1, num)
		ok, err = client.SetnxEx("newhello", "newworld", 5)
		assert.Nil(t, err)
		assert.True(t, ok)
		val, err = client.Get("newhello")
		assert.Nil(t, err)
		assert.Equal(t, "newworld", val)
	})
}

func TestRedis_SetGetDelHashField(t *testing.T) {
	runOnRedis(t, func(client *Redis) {
		err := client.Hset("key", "field", "value")
		assert.Nil(t, err)
		val, err := client.Hget("key", "field")
		assert.Nil(t, err)
		assert.Equal(t, "value", val)
		ok, err := client.Hexists("key", "field")
		assert.Nil(t, err)
		assert.True(t, ok)
		ret, err := client.Hdel("key", "field")
		assert.Nil(t, err)
		assert.True(t, ret)
		ok, err = client.Hexists("key", "field")
		assert.Nil(t, err)
		assert.False(t, ok)
	})
}

func TestRedis_SortedSet(t *testing.T) {
	runOnRedis(t, func(client *Redis) {
		ok, err := client.Zadd("key", 1, "value1")
		assert.Nil(t, err)
		assert.True(t, ok)
		ok, err = client.Zadd("key", 2, "value1")
		assert.Nil(t, err)
		assert.False(t, ok)
		val, err := client.Zscore("key", "value1")
		assert.Nil(t, err)
		assert.Equal(t, int64(2), val)
		val, err = client.Zincrby("key", 3, "value1")
		assert.Nil(t, err)
		assert.Equal(t, int64(5), val)
		val, err = client.Zscore("key", "value1")
		assert.Nil(t, err)
		assert.Equal(t, int64(5), val)
		val, err = client.Zadds("key", Pair{
			Key:   "value2",
			Score: 6,
		}, Pair{
			Key:   "value3",
			Score: 7,
		})
		assert.Nil(t, err)
		assert.Equal(t, int64(2), val)
		pairs, err := client.ZRevRangeWithScores("key", 1, 3)
		assert.Nil(t, err)
		assert.EqualValues(t, []Pair{
			{
				Key:   "value2",
				Score: 6,
			},
			{
				Key:   "value1",
				Score: 5,
			},
		}, pairs)
		rank, err := client.Zrank("key", "value2")
		assert.Nil(t, err)
		assert.Equal(t, int64(1), rank)
		rank, err = client.Zrevrank("key", "value1")
		assert.Nil(t, err)
		assert.Equal(t, int64(2), rank)
		_, err = client.Zrank("key", "value4")
		assert.Equal(t, Nil, err)
		num, err := client.Zrem("key", "value2", "value3")
		assert.Nil(t, err)
		assert.Equal(t, 2, num)
		ok, err = client.Zadd("key", 6, "value2")
		assert.Nil(t, err)
		assert.True(t, ok)
		ok, err = client.Zadd("key", 7, "value3")
		assert.Nil(t, err)
		assert.True(t, ok)
		ok, err = client.Zadd("key", 8, "value4")
		assert.Nil(t, err)
		assert.True(t, ok)
		num, err = client.Zremrangebyscore("key", 6, 7)
		assert.Nil(t, err)
		assert.Equal(t, 2, num)
		ok, err = client.Zadd("key", 6, "value2")
		assert.Nil(t, err)
		assert.True(t, ok)
		ok, err = client.Zadd("key", 7, "value3")
		assert.Nil(t, err)
		assert.True(t, ok)
		num, err = client.Zcount("key", 6, 7)
		assert.Nil(t, err)
		assert.Equal(t, 2, num)
		num, err = client.Zremrangebyrank("key", 1, 2)
		assert.Nil(t, err)
		assert.Equal(t, 2, num)
		card, err := client.Zcard("key")
		assert.Nil(t, err)
		assert.Equal(t, 2, card)
		vals, err := client.Zrange("key", 0, -1)
		assert.Nil(t, err)
		assert.EqualValues(t, []string{"value1", "value4"}, vals)
		vals, err = client.Zrevrange("key", 0, -1)
		assert.Nil(t, err)
		assert.EqualValues(t, []string{"value4", "value1"}, vals)
		pairs, err = client.ZrangeWithScores("key", 0, -1)
		assert.Nil(t, err)
		assert.EqualValues(t, []Pair{
			{
				Key:   "value1",
				Score: 5,
			},
			{
				Key:   "value4",
				Score: 8,
			},
		}, pairs)
		pairs, err = client.ZrangebyscoreWithScores("key", 5, 8)
		assert.Nil(t, err)
		assert.EqualValues(t, []Pair{
			{
				Key:   "value1",
				Score: 5,
			},
			{
				Key:   "value4",
				Score: 8,
			},
		}, pairs)
		pairs, err = client.ZrangebyscoreWithScoresAndLimit("key", 5, 8, 1, 1)
		assert.Nil(t, err)
		assert.EqualValues(t, []Pair{
			{
				Key:   "value4",
				Score: 8,
			},
		}, pairs)
		pairs, err = client.ZrangebyscoreWithScoresAndLimit("key", 5, 8, 1, 0)
		assert.Nil(t, err)
		assert.Equal(t, 0, len(pairs))
		pairs, err = client.ZrevrangebyscoreWithScores("key", 5, 8)
		assert.Nil(t, err)
		assert.EqualValues(t, []Pair{
			{
				Key:   "value4",
				Score: 8,
			},
			{
				Key:   "value1",
				Score: 5,
			},
		}, pairs)
		pairs, err = client.ZrevrangebyscoreWithScoresAndLimit("key", 5, 8, 1, 1)
		assert.Nil(t, err)
		assert.EqualValues(t, []Pair{
			{
				Key:   "value1",
				Score: 5,
			},
		}, pairs)
		pairs, err = client.ZrevrangebyscoreWithScoresAndLimit("key", 5, 8, 1, 0)
		assert.Nil(t, err)
		assert.Equal(t, 0, len(pairs))
		client.Zadd("second", 2, "aa")
		client.Zadd("third", 3, "bbb")
		val, err = client.Zunionstore("union", ZStore{
			Weights:   []float64{1, 2},
			Aggregate: "SUM",
		}, "second", "third")
		assert.Nil(t, err)
		assert.Equal(t, int64(2), val)
		vals, err = client.Zrange("union", 0, 10000)
		assert.Nil(t, err)
		assert.EqualValues(t, []string{"aa", "bbb"}, vals)
		ival, err := client.Zcard("union")
		assert.Nil(t, err)
		assert.Equal(t, 2, ival)
	})
}

func TestRedis_Pipelined(t *testing.T) {
	runOnRedis(t, func(client *Redis) {
		err := client.Pipelined(
			func(pipe Pipeliner) error {
				pipe.Incr("pipelined_counter")
				pipe.Expire("pipelined_counter", time.Hour)
				pipe.ZAdd("zadd", Z{Score: 12, Member: "zadd"})
				return nil
			},
		)
		assert.Nil(t, err)
		ttl, err := client.Ttl("pipelined_counter")
		assert.Nil(t, err)
		assert.Equal(t, 3600, ttl)
		value, err := client.Get("pipelined_counter")
		assert.Nil(t, err)
		assert.Equal(t, "1", value)
		score, err := client.Zscore("zadd", "zadd")
		assert.Nil(t, err)
		assert.Equal(t, int64(12), score)
	})
}

func TestRedisString(t *testing.T) {
	runOnRedis(t, func(client *Redis) {
		client.Ping()
		_, err := getRedis(NewRedis(client.Addr, ClusterType))
		assert.Nil(t, err)
		assert.Equal(t, client.Addr, client.String())
	})
}

func TestRedisScriptLoad(t *testing.T) {
	runOnRedis(t, func(client *Redis) {
		client.Ping()
		_, err := client.ScriptLoad("foo")
		assert.NotNil(t, err)
	})
}

func TestRedisEvalSha(t *testing.T) {
	runOnRedis(t, func(client *Redis) {
		client.Ping()
		scriptHash, err := client.ScriptLoad(`return redis.call("EXISTS", KEYS[1])`)
		assert.Nil(t, err)
		result, err := client.EvalSha(scriptHash, []string{"key1"})
		assert.Nil(t, err)
		assert.Equal(t, int64(0), result)
	})
}

func TestRedisToPairs(t *testing.T) {
	pairs := toPairs([]red.Z{
		{
			Member: 1,
			Score:  1,
		},
		{
			Member: 2,
			Score:  2,
		},
	})
	assert.EqualValues(t, []Pair{
		{
			Key:   "1",
			Score: 1,
		},
		{
			Key:   "2",
			Score: 2,
		},
	}, pairs)
}

func TestRedisToStrings(t *testing.T) {
	vals := toStrings([]interface{}{1, 2})
	assert.EqualValues(t, []string{"1", "2"}, vals)
}

func TestRedisBlpop(t *testing.T) {
	runOnRedis(t, func(client *Redis) {
		client.Ping()
		var node mockedNode
		_, err := client.Blpop(nil, "foo")
		assert.NotNil(t, err)
		_, err = client.Blpop(node, "foo")
		assert.NotNil(t, err)
	})
}

func TestRedisBlpopEx(t *testing.T) {
	runOnRedis(t, func(client *Redis) {
		client.Ping()
		var node mockedNode
		_, _, err := client.BlpopEx(nil, "foo")
		assert.NotNil(t, err)
		_, _, err = client.BlpopEx(node, "foo")
		assert.NotNil(t, err)
	})
}

func TestRedisGeo(t *testing.T) {
	runOnRedis(t, func(client *Redis) {
		client.Ping()
		var geoLocation = []*GeoLocation{{Longitude: 13.361389, Latitude: 38.115556, Name: "Palermo"}, {Longitude: 15.087269, Latitude: 37.502669, Name: "Catania"}}
		v, err := client.GeoAdd("sicily", geoLocation...)
		assert.Nil(t, err)
		assert.Equal(t, int64(2), v)
		v2, err := client.GeoDist("sicily", "Palermo", "Catania", "m")
		assert.Nil(t, err)
		assert.Equal(t, 166274, int(v2))
		// GeoHash not support
		v3, err := client.GeoPos("sicily", "Palermo", "Catania")
		assert.Nil(t, err)
		assert.Equal(t, int64(v3[0].Longitude), int64(13))
		assert.Equal(t, int64(v3[0].Latitude), int64(38))
		assert.Equal(t, int64(v3[1].Longitude), int64(15))
		assert.Equal(t, int64(v3[1].Latitude), int64(37))
		v4, err := client.GeoRadius("sicily", 15, 37, &red.GeoRadiusQuery{WithDist: true, Unit: "km", Radius: 200})
		assert.Nil(t, err)
		assert.Equal(t, int64(v4[0].Dist), int64(190))
		assert.Equal(t, int64(v4[1].Dist), int64(56))
		var geoLocation2 = []*GeoLocation{{Longitude: 13.583333, Latitude: 37.316667, Name: "Agrigento"}}
		v5, err := client.GeoAdd("sicily", geoLocation2...)
		assert.Nil(t, err)
		assert.Equal(t, int64(1), v5)
		v6, err := client.GeoRadiusByMember("sicily", "Agrigento", &red.GeoRadiusQuery{Unit: "km", Radius: 100})
		assert.Nil(t, err)
		assert.Equal(t, v6[0].Name, "Agrigento")
		assert.Equal(t, v6[1].Name, "Palermo")
	})
}

func runOnRedis(t *testing.T, fn func(client *Redis)) {
	s, err := miniredis.Run()
	assert.Nil(t, err)
	defer func() {
		client, err := clientManager.GetResource(s.Addr(), func() (io.Closer, error) {
			return nil, errors.New("should already exist")
		})
		if err != nil {
			t.Error(err)
		}

		if client != nil {
			client.Close()
		}
	}()
	fn(NewRedis(s.Addr(), NodeType))

}

func runOnRedisTLS(t *testing.T, fn func(client *Redis)) {
	s, err := miniredis.RunTLS(&tls.Config{
		Certificates:       make([]tls.Certificate, 1),
		InsecureSkipVerify: true,
	})
	assert.Nil(t, err)
	defer func() {
		client, err := clientManager.GetResource(s.Addr(), func() (io.Closer, error) {
			return nil, errors.New("should already exist")
		})
		if err != nil {
			t.Error(err)
		}
		if client != nil {
			client.Close()
		}
	}()
	fn(New(s.Addr(), WithTLS()))
}

type mockedNode struct {
	RedisNode
}

func (n mockedNode) BLPop(timeout time.Duration, keys ...string) *red.StringSliceCmd {
	return red.NewStringSliceCmd("foo", "bar")
}
