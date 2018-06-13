package redis

import (
	"fmt"
	"log"
	"math/big"
	"strconv"
	"strings"
	"time"

	//"github.com/ethereum/go-ethereum/common"
	//"github.com/ethereumproject/go-ethereum/common"
	"gopkg.in/redis.v3"

	"../ethhelp"
	"../util"
)

func (r *RedisClient) WriteNode(ip, port string) error {
	tx := r.client.Multi()
	defer tx.Close()
	//map eth:nodes:ip port 1024 lastBeat 1111111
	//set ip port 可以联通的
	now := util.MakeTimestamp() / 1000

	_, err := tx.Exec(func() error {
		tx.HSet(r.formatKey("nodes"), join(ip, "port"), port)

		tx.HSet(r.formatKey("nodes"), join(ip, "lastBeat"), strconv.FormatInt(now, 10))
		return nil
	})
	return err
}
func (r *RedisClient) Exist(ip string) bool {
	tx := r.client.Multi()
	defer tx.Close()
	//map eth:nodes:ip port 1024 lastBeat 1111111
	//set ip port 可以联通的
	_, err := tx.Exec(func() error {
		_, keys, _ := r.client.Scan(c, r.formatKey("hashrate", "*"), now).Result()
		return len(keys) != 0
	})
	return false
}
func (r *RedisClient) GetPort(ip string) int {
	tx := r.client.Multi()
	defer tx.Close()
	//map eth:nodes:ip port 1024 lastBeat 1111111
	//set ip port 可以联通的
	cmds, err := tx.Exec(func() error {
		tx.HGet(r.formatKey("nodes", ip), port)
	})
	if err != nil && err != redis.Nil {
		return 0
	} else {
		result, _ := cmds[0].(*redis.StringStringMapCmd).Result()
		ret, _ := strconv.Atoi(result)
		return ret
	}
}
func (r *RedisClient) WriteGoodPort(iport string) {
	tx := r.client.Multi()
	defer tx.Close()
	//map eth:nodes:ip port 1024 lastBeat 1111111
	//set ip port 可以联通的
	tx.Exec(func() error {
		tx.Sadd(r.formatKey("goodport"), iport)
	})
}
