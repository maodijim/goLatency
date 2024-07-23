package main

import (
	"flag"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"golang.org/x/net/context"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

var (
	targets      []string
	esServers    []string
	pingCount    int
	pingInterval int
)

type pingResult struct {
	MsgType icmp.Type
	Latency time.Duration
}

func parseArgs() {
	var targetStr string
	var esServersStr string
	flag.StringVar(&targetStr, "targets", "8.8.8.8", "comma separated list of ip or hostname of the server to ping")
	flag.StringVar(&esServersStr, "esServers", "http://localhost:9200", "Comma separated Elasticsearch servers")
	flag.IntVar(&pingCount, "pingCount", 3, "number of times to ping the server")
	flag.IntVar(&pingInterval, "pingInterval", 60, "interval between each ping round in seconds")
	flag.Parse()
	targets = strings.Split(targetStr, ",")
	esServers = strings.Split(esServersStr, ",")
}

func pingOnce(ctx context.Context, dest string) pingResult {
	// ping the server
	ch := make(chan pingResult)

	go func() {
		c, err := icmp.ListenPacket("ip4:icmp", "0.0.0.0")
		if err != nil {
			log.Fatalf("failed to listen err: %s", err)
		}
		defer c.Close()

		wm := icmp.Message{
			Type: ipv4.ICMPTypeEcho,
			Code: 0,
			Body: &icmp.Echo{
				ID:   os.Getegid() & 0xffff,
				Seq:  1,
				Data: []byte("HELLO-R-U-THERE"),
			},
		}
		wb, err := wm.Marshal(nil)
		if err != nil {
			log.Fatal(err)
		}

		startTime := time.Now()
		if _, err := c.WriteTo(wb, &net.IPAddr{IP: net.ParseIP(dest)}); err != nil {
			log.Printf("WriteTo err: %s", err)
			return // return to avoid deadlock
		}

		rb := make([]byte, 1500)
		n, peer, err := c.ReadFrom(rb)
		if err != nil {
			log.Print(err)
			return // return to avoid deadlock
		}
		endTime := time.Now()

		rm, err := icmp.ParseMessage(ipv4.ICMPTypeEchoReply.Protocol(), rb[:n])
		if err != nil {
			log.Print(err)
			return // return to avoid deadlock
		}

		switch rm.Type {
		case ipv4.ICMPTypeEchoReply:
			log.Printf("got reply from %v", peer)
		case ipv4.ICMPTypeDestinationUnreachable:
			log.Printf("destination unreachable")
		case ipv4.ICMPTypeTimeExceeded:
			log.Printf("timed out")
		default:
			log.Printf("got %+v; want echo reply", rm)
		}
		ch <- pingResult{
			MsgType: rm.Type,
			Latency: endTime.Sub(startTime),
		}
	}()

	select {
	case <-ctx.Done():
		log.Print("ping timeout")
		return pingResult{
			MsgType: ipv4.ICMPTypeTimeExceeded,
			Latency: time.Duration(0),
		}
	case res := <-ch:
		return res
	}
}

func pingX(dest string, repeat int) []pingResult {
	results := []pingResult{}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*time.Duration(3*repeat))
	defer cancel()
	for i := 0; i < repeat; i++ {
		result := pingOnce(ctx, dest)
		results = append(results, result)
	}
	return results
}

func oneRound() {
	for _, target := range targets {
		log.Printf("pinging %s", target)
		res := pingX(target, pingCount)
		log.Printf("ping result: %v", res)
		log.Printf("pinged %s %d times", target, pingCount)
		sendPingEs(target, res)
		maxLatency := time.Duration(0)
		minLatency := time.Duration(1000000000)
		totalLatency := time.Duration(0)
		for _, r := range res {
			if r.Latency > maxLatency {
				maxLatency = r.Latency
			}
			if r.Latency < minLatency {
				minLatency = r.Latency
			}
			totalLatency += r.Latency
		}
		if pingCount == 0 {
			minLatency = time.Duration(0)
		}
		log.Printf("max latency: %v", maxLatency)
		log.Printf("min latency: %v", minLatency)
		log.Printf("average latency: %v", totalLatency/time.Duration(pingCount))
	}
}

func main() {
	parseArgs()
	ticker := time.NewTicker(time.Second * time.Duration(pingInterval))
	for {
		select {
		case <-ticker.C:
			oneRound()
		}
	}
}
