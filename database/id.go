package database

import (
	"fmt"
	"os"
	"sync"
	"time"
)

const (
	snowflakeEpochMs int64 = 1735689600000 // 2025-01-01T00:00:00Z
	nodeBits               = 5
	sequenceBits           = 7
	maxNodeID        int64 = (1 << nodeBits) - 1
	maxSequence      int64 = (1 << sequenceBits) - 1
	nodeShift              = sequenceBits
	timeShift              = nodeBits + sequenceBits
)

var globalIDGenerator = newSnowflakeGenerator()

// NextID 生成 53-bit 雪花 ID，兼容 PostgreSQL BIGINT 和浏览器 Number 精度。
func NextID() int64 {
	return globalIDGenerator.Next()
}

type snowflakeGenerator struct {
	mu       sync.Mutex
	lastTS   int64
	sequence int64
	nodeID   int64
}

func newSnowflakeGenerator() *snowflakeGenerator {
	return &snowflakeGenerator{
		nodeID: int64(os.Getpid()) & maxNodeID,
	}
}

func (g *snowflakeGenerator) Next() int64 {
	g.mu.Lock()
	defer g.mu.Unlock()

	now := currentMs()
	if now < g.lastTS {
		now = g.lastTS
	}

	if now == g.lastTS {
		g.sequence = (g.sequence + 1) & maxSequence
		if g.sequence == 0 {
			now = g.waitNextMs(now)
		}
	} else {
		g.sequence = 0
	}

	g.lastTS = now
	id := ((now - snowflakeEpochMs) << timeShift) | (g.nodeID << nodeShift) | g.sequence
	if id <= 0 {
		panic(fmt.Sprintf("生成雪花 ID 失败: now=%d node=%d seq=%d", now, g.nodeID, g.sequence))
	}
	return id
}

func (g *snowflakeGenerator) waitNextMs(last int64) int64 {
	now := currentMs()
	for now <= last {
		time.Sleep(time.Millisecond)
		now = currentMs()
	}
	g.sequence = 0
	return now
}

func currentMs() int64 {
	return time.Now().UnixMilli()
}
