package repository

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestBuildRedisOptions(t *testing.T) {
	cfg := &config.Config{
		Redis: config.RedisConfig{
			Host:                "localhost",
			Port:                6379,
			Username:            "app-user",
			Password:            "secret",
			DB:                  2,
			DialTimeoutSeconds:  5,
			ReadTimeoutSeconds:  3,
			WriteTimeoutSeconds: 4,
			PoolSize:            100,
			MinIdleConns:        10,
		},
	}

	opts := buildRedisOptions(cfg)
	require.Equal(t, "localhost:6379", opts.Addr)
	require.Equal(t, "app-user", opts.Username)
	require.Equal(t, "secret", opts.Password)
	require.Equal(t, 2, opts.DB)
	require.Equal(t, 5*time.Second, opts.DialTimeout)
	require.Equal(t, 3*time.Second, opts.ReadTimeout)
	require.Equal(t, 4*time.Second, opts.WriteTimeout)
	require.Equal(t, 100, opts.PoolSize)
	require.Equal(t, 10, opts.MinIdleConns)
	require.Nil(t, opts.TLSConfig)

	// Test case with TLS enabled
	cfgTLS := &config.Config{
		Redis: config.RedisConfig{
			Host:      "localhost",
			EnableTLS: true,
		},
	}
	optsTLS := buildRedisOptions(cfgTLS)
	require.NotNil(t, optsTLS.TLSConfig)
	require.Equal(t, "localhost", optsTLS.TLSConfig.ServerName)
}

func TestBuildRedisOptionsHonorsCommandDeadline(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	commandReceived := make(chan struct{})
	releaseServer := make(chan struct{})
	serverDone := make(chan error, 1)
	go func() {
		reader := bufio.NewReader(serverConn)
		for {
			command, err := readRedisTestCommand(reader)
			if err != nil {
				serverDone <- err
				return
			}
			var reply string
			switch strings.ToLower(command[0]) {
			case "hello":
				reply = "-ERR unknown command 'hello'\r\n"
			case "ping":
				reply = "+PONG\r\n"
			case "get":
				close(commandReceived)
				<-releaseServer
				serverDone <- nil
				return
			default:
				serverDone <- fmt.Errorf("unexpected Redis command %q", command[0])
				return
			}
			if _, err := io.WriteString(serverConn, reply); err != nil {
				serverDone <- err
				return
			}
		}
	}()
	t.Cleanup(func() {
		close(releaseServer)
		_ = clientConn.Close()
		_ = serverConn.Close()
		require.NoError(t, <-serverDone)
	})

	opts := buildRedisOptions(&config.Config{Redis: config.RedisConfig{
		ReadTimeoutSeconds:  2,
		WriteTimeoutSeconds: 2,
		PoolSize:            1,
	}})
	opts.Dialer = func(context.Context, string, string) (net.Conn, error) {
		return clientConn, nil
	}
	opts.MaxRetries = -1
	opts.Protocol = 2
	opts.DisableIdentity = true
	client := redis.NewClient(opts)
	t.Cleanup(func() { _ = client.Close() })
	// Finish connection setup before timing a command whose response never arrives.
	require.NoError(t, client.Ping(context.Background()).Err())

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	started := time.Now()
	err := client.Get(ctx, "deadline-probe").Err()
	elapsed := time.Since(started)
	t.Logf("command deadline=100ms socket timeout=2s elapsed=%s error=%v", elapsed, err)

	select {
	case <-commandReceived:
	default:
		t.Fatal("Redis command did not reach the isolated server")
	}
	require.Error(t, err)
	var timeoutError net.Error
	require.ErrorAs(t, err, &timeoutError)
	require.True(t, timeoutError.Timeout())
	require.Less(t, elapsed, time.Second, "Redis must stop at the caller deadline before the socket timeout")
}

func readRedisTestCommand(reader *bufio.Reader) ([]string, error) {
	header, err := reader.ReadString('\n')
	if err != nil {
		return nil, err
	}
	count, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(header, "*")))
	if err != nil || count < 1 {
		return nil, fmt.Errorf("invalid Redis command array header")
	}
	command := make([]string, count)
	for i := range command {
		header, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		size, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(header, "$")))
		if err != nil || size < 0 {
			return nil, fmt.Errorf("invalid Redis command argument header")
		}
		value := make([]byte, size+2)
		if _, err := io.ReadFull(reader, value); err != nil {
			return nil, err
		}
		command[i] = string(value[:size])
	}
	return command, nil
}
