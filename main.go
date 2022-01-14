package main

import (
	"bufio"
	"log"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/go-co-op/gocron"
)

/*
set GOARCH=amd64
set GOOS=linux
$env:GOOS = "linux"
go build
*/
/*
host: wss://wss.xatbox.com/v2

in: <p u="1" t="/m565044444"  />
	<i b="https://i.imgur.com/FfdnCkZ.png;=;=;=English;=http://89.105.32.22:8110/;=#348aee;=;=" f="84017984" v="3" r="1" cb="57" B="23232323"  />
	<gp p="0|0|256|1074003968|4194304|0|0|0|0|0|0|0|0|0|0|0|0|0|0|0|0|0|0|0|0|" g114="{'m':'Austin','t':'Mahonies','rnk':'5','b':'Banned','v':1}"   />
	<aa b="PGNlbnRlcj48cCBzdHlsZT0ibWFyZ2luOiA3cHggMCAxMHB4IDAiPjxiPk5ldyB4YXRzcGFjZSAoeGF0Lm1lKSBvdXQgbm93ITwvYj48L3A+PGltZyBzcmM9Imh0dHBzOi8veGF0Lndpa2kvdy9pbWFnZXMvOC84OS9OZXd4cy5wbmciIHdpZHRoPSIyNDAiPjxkaXYgc3R5bGU9Im1hcmdpbjogMjBweCAwIiA+PGEgaHJlZj0iaHR0cHM6Ly91LnhhdC5jb20vemJ4IiB0YXJnZXQ9Il9ibGFuayIgc3R5bGU9InBhZGRpbmc6IDNweCAxMHB4OyBiYWNrZ3JvdW5kLWNvbG9yOiAjMDA3YmZmOyBib3JkZXItcmFkaXVzOiAzcHg7IGNvbG9yOiAjZmZmOyB0ZXh0LWFsaWduOiBjZW50ZXI7IHRleHQtZGVjb3JhdGlvbjogbm9uZTsiPlJlYWQgbW9yZTwvYT48L2Rpdj48cCBzdHlsZT0iY29sb3I6ICM3MDcwNzA7IGZvbnQtc2l6ZTogMTRweDsiPlNlbnQgYnkgeGF0LmNvbTwvcD48L2NlbnRlcj4="  />
	<m t="/s" d="0"  />
	<w v="0 0 1 2"  />
	<done  />
out: <m t="msg" o="emre0zf8f" u="638877683" />
	 <z d="23232323" u="638877683_0" t="/l" /> (tickle)
	 <z d="23232323" u="638877683_0" t="!pc stuff" o="ixbian4gg" s="2" />
												   o=unique random, Math.random().toString(36).substr(2, 9)
*/

const (
	MasterChat = "PGO"
	Debug      = false
)

var MasterBot *Client

func newBot(chatName string, hub *Hub) *Client {
	client := newClient(hub, chatName)
	if client == nil {
		return nil
	}
	go client.writePump()
	go client.readPump()
	go client.handlePump()
	client.doJoin()
	return client
}

func main() {
	log.SetFlags(0)

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	hub := newHub()
	hub.restore()
	hub.debug = Debug
	hub.botInfo = fetchGlobalBotInfo()
	go hub.run()

	MasterBot = newBot(MasterChat, hub)
	MasterBot.setMaster(true)

	for n := range hub.chatCache {
		if !strings.EqualFold(n, MasterChat) {
			newBot(n, hub)
		}
	}

	// cmd command
	var cmd = make(chan string)
	go func() {
		reader := bufio.NewScanner(os.Stdin)
		for {
			for reader.Scan() {
				l := reader.Text()
				if len(l) == 0 {
					break
				}
				cmd <- l
			}
		}
	}()

	s := gocron.NewScheduler(time.UTC)

	j, _ := s.Every(3610).Seconds().StartAt(time.Now().Add(-time.Hour)).Do(lure, hub)
	_, _ = s.Every(5).Minutes().Do(updateChats, hub)
	s.StartAsync()

	for {
		select {
		case s := <-interrupt:
			log.Println("Got signal", s)
			hub.save()
			os.Exit(1)
		case t := <-cmd:
			if t == "" {
				break
			}
			log.Println("received console cmd:", t)
			if t[0] == '/' {
				t = t[1:]
				if strings.HasPrefix(t, "lure") {
					lure(hub)
					_, _ = s.Job(j).StartAt(time.Now().Add(time.Hour).Add(time.Second * 10)).Update()
				} else if strings.HasPrefix(t, "update") {
					updateChats(hub)
				} else if strings.HasPrefix(t, "join") {
					if !strings.Contains(t, " ") {
						break
					}
					if s := strings.Split(t, " "); len(s) >= 2 {
						newBot(s[1], hub)
					}
				}
			} else {
				MasterBot.sendPC(23232323, t)
			}
		}
	}
}

func updateChats(hub *Hub) {
	hub.mutex.Lock()
	hub.chatCache = make(map[string]int)
	for c := range hub.clients {
		if c.isDone() {
			hub.chatCache[c.chatName] = c.chatId
		}
	}
	hub.save()
	hub.mutex.Unlock()
	MasterBot.sendPC(23232323, "Keep alive")
}

func lure(hub *Hub) {
	hub.broadcast <- []byte(buildPacket(&Packet{
		tag: "p",
		attr: PacketAttr{
			"u": 23232323,
			"t": "!pgo use lure",
			"s": 2,
			"d": hub.botInfo["i"],
		},
		order: []string{"u", "t", "s", "d"},
	}))
}
