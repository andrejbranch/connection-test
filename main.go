package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type Member struct {
	ip   net.IP
	port uint16
}

type ConnectionTest struct {
	talkingTo *Member
	myIp      string
	client    *http.Client
}

func main() {
	myIp := os.Getenv("POD_IP")
	if myIp == "" {
		log.Fatalln("POD_IP env variable must be set")
	}

	c := &ConnectionTest{
		myIp:      myIp,
		talkingTo: nil,
		client: &http.Client{
			Timeout: 500 * time.Millisecond,
		},
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	http.HandleFunc("/alive", alive)

	go c.loop()
	go http.ListenAndServe(":8080", nil)

	sig := <-sigs
	log.Printf("------------------------ SIGNAL %s sleeping 30s ------------------------", sig.String())
	time.Sleep(30 * time.Second)
	log.Println("------------------------ SHUTTING DOWN ------------------------")
}

func (c *ConnectionTest) loop() {
	heartbeatTicker := time.NewTicker(1000 * time.Millisecond)
	defer heartbeatTicker.Stop()
	for {
		select {
		case <-heartbeatTicker.C:
			if c.memberCheck() {
				c.checkMemberConnection()
			}
		}
	}
}

func (c *ConnectionTest) checkMemberConnection() {
	resp, err := c.client.Get(fmt.Sprintf("http://%s:%d/alive", c.talkingTo.ip, c.talkingTo.port))
	if err != nil {
		log.Println(err)
		c.talkingTo = nil
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Printf("Error talking to member %s:%d, status %s", c.talkingTo.ip, c.talkingTo.port, resp.Status)
		c.talkingTo = nil
		return
	}

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}
	bodyString := string(bodyBytes)
	log.Printf("member %s:%d says: %s", c.talkingTo.ip, c.talkingTo.port, bodyString)
}

func alive(w http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(w, "im alive\n")
}

func (c *ConnectionTest) memberCheck() bool {
	if c.talkingTo == nil {
		log.Println("searching for member")
		c.findMember()
		if c.talkingTo != nil {
			log.Printf("member found %s:%d", c.talkingTo.ip, c.talkingTo.port)
		}
	}
	return c.talkingTo != nil
}

func (c *ConnectionTest) findMember() {
	_, addrs, err := net.LookupSRV("", "", "cortex-connection-test")
	if err != nil {
		log.Println(err)
		return
	}
	for _, addr := range addrs {
		memberIps, mErr := net.LookupIP(addr.Target)
		if mErr != nil {
			log.Println(mErr)
			return
		}
		if len(memberIps) == 0 {
			return
		}
		memberIp := memberIps[0]
		if memberIp.String() == c.myIp {
			continue
		}
		c.talkingTo = &Member{
			ip:   memberIp,
			port: addr.Port,
		}
	}
}
