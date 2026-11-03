// This fixture never dials, resolves a host, forwards CONNECT or starts a core.
package main

import (
	"encoding/json"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func main() {
	if strings.Contains(filepath.Base(os.Args[0]), "no-read") {
		time.Sleep(30 * time.Second)
		return
	}
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		key = strings.ToUpper(key)
		if strings.HasPrefix(key, "CLASH_") || strings.HasPrefix(key, "XRAY_") || key == "HTTP_PROXY" || key == "HTTPS_PROXY" || key == "ALL_PROXY" {
			os.Exit(4)
		}
	}
	data, err := io.ReadAll(io.LimitReader(os.Stdin, 16385))
	if err != nil || len(data) > 16384 {
		os.Exit(2)
	}
	var cfg map[string]json.RawMessage
	if json.Unmarshal(data, &cfg) != nil {
		os.Exit(2)
	}
	port := 0
	if len(os.Args) == 5 && os.Args[1] == "run" && os.Args[2] == "-format=json" && os.Args[3] == "-config" && os.Args[4] == "stdin:" {
		var in []struct {
			Listen string
			Port   int
		}
		if json.Unmarshal(cfg["inbounds"], &in) != nil || len(in) != 1 || in[0].Listen != "127.0.0.1" {
			os.Exit(2)
		}
		port = in[0].Port
		var out []struct {
			Protocol string
			Settings struct {
				Servers []struct{ Users []struct{ User, Pass string } }
			}
		}
		if json.Unmarshal(cfg["outbounds"], &out) != nil || len(out) != 1 || len(out[0].Settings.Servers) != 1 || len(out[0].Settings.Servers[0].Users) != 1 || out[0].Settings.Servers[0].Users[0].Pass == "" {
			os.Exit(2)
		}
	} else if len(os.Args) == 5 && os.Args[1] == "-d" && os.Args[3] == "-f" && os.Args[4] == "-" {
		if json.Unmarshal(cfg["socks-port"], &port) != nil {
			os.Exit(2)
		}
		var node []struct{ Username, Password string }
		if json.Unmarshal(cfg["proxies"], &node) != nil || len(node) != 1 || node[0].Password == "" {
			os.Exit(2)
		}
	} else {
		os.Exit(2)
	}
	if strings.Contains(filepath.Base(os.Args[0]), "early-exit") {
		os.Exit(3)
	}
	listener, err := net.Listen("tcp4", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		os.Exit(2)
	}
	defer listener.Close()
	for {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		go func() {
			defer conn.Close()
			_ = conn.SetDeadline(time.Now().Add(time.Second))
			greeting := [3]byte{}
			if _, err := io.ReadFull(conn, greeting[:]); err == nil && greeting == [3]byte{5, 1, 0} {
				_, _ = conn.Write([]byte{5, 0})
			}
		}()
	}
}
