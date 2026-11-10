package ranking

import (
	"bufio"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

// RedisBurstCounter is a distributed BurstCounter over Redis sorted sets
// (ZADD + ZREMRANGEBYSCORE + ZCARD), so burst verdicts hold across replicas
// (C-6). It speaks RESP directly over TCP with no third-party dependency.
//
// Failure policy is fail-open: when Redis is unreachable the key is allowed,
// because the counter is a soft abuse signal behind the gateway's PostgreSQL
// rate limits — a Redis blip must not take search down.
type RedisBurstCounter struct {
	addr    string
	timeout time.Duration
	prefix  string
	seq     atomic.Uint64
}

// NewRedisBurstCounter creates a Redis-backed counter. addr is host:port;
// timeout bounds each command round-trip.
func NewRedisBurstCounter(addr string, timeout time.Duration) *RedisBurstCounter {
	if timeout <= 0 {
		timeout = 200 * time.Millisecond
	}
	return &RedisBurstCounter{addr: addr, timeout: timeout, prefix: "openportal:burst:"}
}

// Allow implements BurstCounter.
func (c *RedisBurstCounter) Allow(key string, window time.Duration, max int) bool {
	count, err := c.countSliding(key, window)
	if err != nil {
		return true // fail open (see policy above)
	}
	return count <= max
}

func (c *RedisBurstCounter) countSliding(key string, window time.Duration) (int, error) {
	now := time.Now()
	nowMs := now.UnixMilli()
	minMs := now.Add(-window).UnixMilli()
	conn, err := net.DialTimeout("tcp", c.addr, c.timeout)
	if err != nil {
		return 0, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(now.Add(c.timeout))
	rw := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))
	redisKey := c.prefix + key
	// Unique member per event: same-millisecond events must not collapse.
	member := fmt.Sprintf("%d-%d", nowMs, c.seq.Add(1))
	// ZADD key nowMs member (score must be numeric; member is unique)
	if err := writeCommand(rw, "ZADD", redisKey, fmt.Sprintf("%d", nowMs), member); err != nil {
		return 0, err
	}
	if _, err := readReply(rw); err != nil {
		return 0, err
	}
	// ZREMRANGEBYSCORE key -inf (now-window)
	if err := writeCommand(rw, "ZREMRANGEBYSCORE", redisKey, "0", fmt.Sprintf("%d", minMs)); err != nil {
		return 0, err
	}
	if _, err := readReply(rw); err != nil {
		return 0, err
	}
	// ZCARD key
	if err := writeCommand(rw, "ZCARD", redisKey); err != nil {
		return 0, err
	}
	count, err := readInteger(rw)
	if err != nil {
		return 0, err
	}
	// EXPIRE key 2*window (best-effort cleanup)
	ttl := int(window.Seconds() * 2)
	if ttl < 1 {
		ttl = 1
	}
	_ = writeCommand(rw, "EXPIRE", redisKey, fmt.Sprintf("%d", ttl))
	_, _ = readReply(rw)
	return count, nil
}

func writeCommand(rw *bufio.ReadWriter, args ...string) error {
	var sb strings.Builder
	sb.WriteString("*" + strconv.Itoa(len(args)) + "\r\n")
	for _, arg := range args {
		sb.WriteString("$" + strconv.Itoa(len(arg)) + "\r\n" + arg + "\r\n")
	}
	if _, err := rw.WriteString(sb.String()); err != nil {
		return err
	}
	return rw.Flush()
}

// readReply consumes one RESP reply, returning integer values and accepting
// simple-string/OK replies. Errors propagate.
func readReply(rw *bufio.ReadWriter) (int, error) {
	line, err := rw.ReadString('\n')
	if err != nil {
		return 0, err
	}
	if len(line) < 2 {
		return 0, fmt.Errorf("short redis reply")
	}
	kind, payload := line[0], strings.TrimSuffix(line[1:], "\r\n")
	switch kind {
	case '+':
		return 0, nil
	case ':':
		return strconv.Atoi(payload)
	case '-':
		return 0, fmt.Errorf("redis error: %s", payload)
	default:
		return 0, fmt.Errorf("unexpected redis reply: %q", line)
	}
}

func readInteger(rw *bufio.ReadWriter) (int, error) {
	return readReply(rw)
}
