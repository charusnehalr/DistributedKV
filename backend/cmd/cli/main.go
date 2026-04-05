package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
)

func main() {
	server := flag.String("server", "http://localhost:8080", "kvstore server base URL")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: kvctl [--server URL] <command> [args]")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Commands:")
		fmt.Fprintln(os.Stderr, "  put <key> <value>   Store a key-value pair")
		fmt.Fprintln(os.Stderr, "  get <key>           Retrieve a value")
		fmt.Fprintln(os.Stderr, "  delete <key>        Delete a key")
		fmt.Fprintln(os.Stderr, "  scan --prefix <p>   List keys with prefix")
		fmt.Fprintln(os.Stderr, "  health              Check server health")
	}
	flag.Parse()

	if flag.NArg() == 0 {
		flag.Usage()
		os.Exit(1)
	}

	c := &client{base: *server, http: &http.Client{Timeout: 10e9}}

	switch flag.Arg(0) {
	case "put":
		if flag.NArg() < 3 {
			die("usage: kvctl put <key> <value>")
		}
		c.put(flag.Arg(1), flag.Arg(2))
	case "get":
		if flag.NArg() < 2 {
			die("usage: kvctl get <key>")
		}
		c.get(flag.Arg(1))
	case "delete":
		if flag.NArg() < 2 {
			die("usage: kvctl delete <key>")
		}
		c.delete(flag.Arg(1))
	case "scan":
		fs := flag.NewFlagSet("scan", flag.ExitOnError)
		prefix := fs.String("prefix", "", "key prefix to scan")
		fs.Parse(flag.Args()[1:])
		c.scan(*prefix)
	case "health":
		c.health()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", flag.Arg(0))
		flag.Usage()
		os.Exit(1)
	}
}

type client struct {
	base string
	http *http.Client
}

func (c *client) put(key, value string) {
	body, _ := json.Marshal(map[string]string{"key": key, "value": value})
	resp, err := c.http.Post(c.base+"/api/v1/kv", "application/json", bytes.NewReader(body))
	mustHTTP(err)
	defer resp.Body.Close()
	printResponse(resp)
	if resp.StatusCode >= 400 {
		os.Exit(1)
	}
}

func (c *client) get(key string) {
	resp, err := c.http.Get(c.base + "/api/v1/kv/" + key)
	mustHTTP(err)
	defer resp.Body.Close()
	printResponse(resp)
	if resp.StatusCode >= 400 {
		os.Exit(1)
	}
}

func (c *client) delete(key string) {
	req, _ := http.NewRequest(http.MethodDelete, c.base+"/api/v1/kv/"+key, nil)
	resp, err := c.http.Do(req)
	mustHTTP(err)
	defer resp.Body.Close()
	printResponse(resp)
	if resp.StatusCode >= 400 {
		os.Exit(1)
	}
}

func (c *client) scan(prefix string) {
	resp, err := c.http.Get(c.base + "/api/v1/kv?prefix=" + prefix)
	mustHTTP(err)
	defer resp.Body.Close()
	printResponse(resp)
	if resp.StatusCode >= 400 {
		os.Exit(1)
	}
}

func (c *client) health() {
	resp, err := c.http.Get(c.base + "/api/v1/health")
	mustHTTP(err)
	defer resp.Body.Close()
	printResponse(resp)
	if resp.StatusCode >= 400 {
		os.Exit(1)
	}
}

func printResponse(resp *http.Response) {
	data, _ := io.ReadAll(resp.Body)
	var pretty bytes.Buffer
	if json.Indent(&pretty, data, "", "  ") == nil {
		fmt.Println(pretty.String())
	} else {
		fmt.Println(string(data))
	}
}

func mustHTTP(err error) {
	if err != nil {
		die("request failed: " + err.Error())
	}
}

func die(msg string) {
	fmt.Fprintln(os.Stderr, "error:", msg)
	os.Exit(1)
}
