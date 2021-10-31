package main

import (
	"bufio"
	"bytes"
	"fmt"
	"html"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/joho/godotenv"

	"github.com/davecgh/go-spew/spew"

	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 4096
)

var (
	newline = []byte("\n")
	space   = []byte{' '}
	u       = url.URL{Scheme: "wss", Host: "wss.xatbox.com", Path: "/v2"}
)

var (
	defaultHandlers map[string]PacketHandler
)

func DoDefaultHandlers() {
	defaultHandlers["y"] = func(c *Client, p *Packet) {
		//in: <y I="32688" i="1582150546" c="1628987876" cb="147" k="32688" t="10025" s="70"  />
		//out: <j2 cb="1628984378" Y="2" l5="per" l4="600" l3="505" l2="0" y="1582150546" k="3"
		//         k3="3" z="m1.55.4,3" p="0" c="164014162" f="0" u="1518734109" d0="5216"
		//         d2="3" d3="6363220" dx="294" dt="1628601033" N="flabbergast" n="Idiot######"
		//         a="https://i.imgur.com/a23pPTE.png" h="" v="0" />
		if p.hasAttrib("C") {
			log.Println("captcha")
			c.conn.Close()
			return
		}

		loginAuth := &Packet{
			tag: "j2",
			attr: PacketAttr{
				"cb": time.Now().Unix(),
				"Y":  2,
				"l5": "per",
				"l4": 600,
				"l3": 505,
				"l2": 0,

				"z": "m1.55.4,3",

				"y": p.getAttrib("i"),

				"p": 0,
				"c": c.chatId,
				"f": 2,
			},
		}
		for k, v := range c.hub.botInfo {
			switch k {
			case "k1":
				k = "k"
			case "i":
				k = "u"
			case "n", "username":
				k = "N"
			case "tag", "k2", "password":
				continue
			}
			loginAuth.attr[k] = v
		}
		loginAuth.attr["n"] = "flab·ber·gast"
		loginAuth.attr["a"] = "1306"
		loginAuth.attr["h"] = "https://xat.com/PGO"
		loginAuth.attr["v"] = 0
		loginAuth.addOrder("cb", "Y", "l5", "l4", "l3", "l2", "y", "k", "k3", "d1", "z", "p", "c", "f", "u", "d0", "d2", "d3", "d20", "dx", "dt", "N", "n", "a", "h", "v")

		if loginAuth.hasAttrib("d1") && p.getAttrib("c") >= loginAuth.getAttrib("d1") {
			delete(loginAuth.attr, "d1")
			delete(loginAuth.attr, "d20")
		}

		c.sendPacket(loginAuth)
	}

	msgHandler := func(c *Client, p *Packet) {
		if !c.isDone() || !p.hasAttrib("u") {
			return
		}

		if p.tag == "m" && strings.HasPrefix(p.getAttrib("u"), "23232323") {
			exp := regexp.MustCompile(`\[.*?] https?://xat.com/(.*?) - (.*?) \| (\d+) seconds \| ([\d.]+)% chance`)
			var match []string
			if exp.MatchString(p.getAttrib("t")) {
				match = exp.FindAllStringSubmatch(p.getAttrib("t"), -1)[0][1:]
			} else {
				exp = regexp.MustCompile(`A wild (.*?) has appeared! It will run away in (.*?) seconds. Use ”!pgo catch” to catch it before it runs! \(Chance to catch: (.*?)%\)`)
				if !exp.MatchString(p.getAttrib("t")) {
					//log.Println("wtf?", p.getAttrib("t"))
					return
				}
				match = exp.FindAllStringSubmatch(p.getAttrib("t"), -1)[0]
				match[0] = c.chatName
			}

			if match == nil || len(match) <= 1 {
				return
			}

			//if i, err := strconv.Atoi(match[len(match)-1]); (err != nil && i < 10) || strings.Contains(match[0], "(dmd)Shiny") {
			//	// bump alert
			//}

			if m, ok := c.hub.latestSpawns[c.chatId]; ok && m == match[1] {
				return
			}

			//t := time.Now()

			for c, b := range c.hub.clients {
				if strings.EqualFold(c.chatName, match[0]) {
					if b {
						c.hub.mutex.Lock()
						c.hub.latestSpawns[c.chatId] = match[1]
						c.hub.mutex.Unlock()
						c.sendPC(23232323, "!pgc")
					}
					return
				}
			}

			log.Println("spawning", match[0])
			c2 := newBot(match[0], c.hub)
			if c2 == nil {
				return
			}
			c2.catchOnJoin = true
		} else if p.tag == "p" && p.hasAttrib("s") && p.getAttrib("s") == "2" && strings.Contains(p.getAttrib("t"), c.hub.botInfo["username"]) {
			log.Println("caught:", p.getAttrib("t"))

			if strings.Contains(p.getAttrib("t"), "PokemonGO is not allowed in this chat, sorry.") ||
				strings.Contains(p.getAttrib("t"), "You are not high enough rank to use this command, the minimum rank is") ||
				strings.Contains(p.getAttrib("t"), "You are not high enough rank to use the bot.") ||
				strings.Contains(p.getAttrib("t"), "PokemonGO nie jest dozwolone w tym czacie, niestety") ||
				strings.HasPrefix(p.getAttrib("t"), "PokemonGO") {
				// not a pgo chat
				c.conn.Close()
				return
			}
			if strings.Contains(p.getAttrib("t"), "Respond to this message within") {
				c.sendPC(23232323, "pls no")
			}

		} else if p.tag == "p" && strings.HasPrefix(p.getAttrib("u"), "23232323") {
			if strings.Contains(p.getAttrib("t"), "This chat is already activated with a lure!") {
				log.Println("private:", p.getAttrib("t"))
			}

			if c.isMaster() {
				log.Println("pc:", p.getAttrib("t"))
				return
			}
		}
	}
	defaultHandlers["m"] = msgHandler
	defaultHandlers["p"] = msgHandler

	loginStatus := func(c *Client, p *Packet) {
		defer func() {
			c.conn.Close()
		}()
		spew.Dump(p.attr)
		if p.hasAttrib("e") {
			c.hub.botInfo = login(os.Getenv("username"), os.Getenv("password"))
			spew.Dump(c.hub.botInfo)
			err := godotenv.Write(c.hub.botInfo, "./.env")
			if err != nil {
				log.Fatal("could not save .env file")
			}
			return
		}
		env, err := godotenv.Read()
		if err != nil {
			log.Fatal("could not load .env file")
		}

		for k, v := range p.attr {
			env[k] = v.(string)
		}
		err = godotenv.Write(env, "./.env")
		if err != nil {
			log.Fatal("could not save .env file")
		}
		c.hub.botInfo = fetchGlobalBotInfo()

		spew.Dump(p)
		//log.Fatal("logged out")
		_ = bufio.NewWriter(os.Stdout).Flush()
	}
	defaultHandlers["v"] = loginStatus
	defaultHandlers["logout"] = loginStatus

	defaultHandlers["w"] = func(c *Client, p *Packet) {
		if !strings.HasPrefix(p.getAttrib("v"), "0") {
			c.sendPacket(&Packet{
				tag: "w0",
			})
		}
	}

	defaultHandlers["dup"] = func(c *Client, p *Packet) {
		c.conn.Close()
	}

	defaultHandlers["u"] = func(c *Client, p *Packet) {
		if strings.HasPrefix(p.getAttrib("u"), "23232323") {
			c.hasBot = true
		}
	}

	defaultHandlers["done"] = func(c *Client, p *Packet) {
		c.done = true
		if !c.hasBot && !c.isMaster() {
			log.Println("no bot")
			c.conn.Close()
		} else if c.catchOnJoin {
			c.sendPC(23232323, "!pgc")
		}
	}

	defaultHandlers["ldone"] = func(c *Client, p *Packet) {
		c.conn.Close()
	}

	defaultHandlers["z"] = func(c *Client, p *Packet) {
		c.sendPacket(&Packet{
			tag: "z",
			attr: PacketAttr{
				"d": strings.Split(p.getAttrib("u"), "_")[0],
				"u": c.hub.botInfo["i"],
				"t": "/a_not added you as a friend",
			},
			order: []string{"d", "u", "t"},
		})
	}
}

// Client is a middleman between the websocket connection and the hub.
type Client struct {
	hub *Hub

	// The websocket connection.
	conn *websocket.Conn

	// Buffered channel of outbound messages.
	send chan []byte

	// Packet handlers
	handlers    map[string]PacketHandler
	parsed      chan *Packet
	chatId      int
	chatName    string
	done        bool
	hasBot      bool
	catchOnJoin bool
	master      bool
}

func newClient(hub *Hub, name string) *Client {
	chatId := findChat(hub, name)
	if chatId == -1 {
		log.Printf("Could could get chatId for \"%s\"\n", name)
		return nil
	}

	if len(defaultHandlers) == 0 {
		defaultHandlers = make(map[string]PacketHandler)
		DoDefaultHandlers()
	}

	log.Printf("connecting to %s", u.String()) // Spawning new client for chat NAME (ID)

	c, _, err := websocket.DefaultDialer.Dial(u.String(), http.Header{
		"Origin": []string{"https://xat.com"},
	})

	if err != nil {
		log.Fatal("dial:", err)
	}

	client := &Client{hub: hub, conn: c, send: make(chan []byte, 256), handlers: make(map[string]PacketHandler), parsed: make(chan *Packet, 10)}
	client.chatId = chatId
	client.chatName = name
	client.doHandlers()
	client.hub.register <- client

	return client
}

func (c *Client) doJoin() {
	if c.chatId <= 0 {
		log.Fatal("invalid chat id")
	}
	c.sendMessage(fmt.Sprintf("<y r=\"%v\" v=\"0\" u=\"%v\" z=\"8335799305056508195\" />", c.chatId, c.hub.botInfo["i"]))
}

func (c *Client) sendMessage(msg string) {
	defer func() {
		if r := recover(); r != nil {
			log.Println("recovered in sendmsg", r)
		}
	}()
	c.send <- []byte(msg)
}

func buildPacket(p *Packet) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("<%v", p.tag))
	if len(p.attr) > 0 {
		var keys []string
		for k := range p.attr {
			keys = append(keys, k)
		}
		sort.SliceStable(keys, func(i, j int) bool {
			return Find(p.order, keys[i]) < Find(p.order, keys[j])
		})
		for _, k := range keys {
			v := p.attr[k]
			b.WriteString(fmt.Sprintf(" %v=\"%v\"", k, html.EscapeString(fmt.Sprintf("%v", v))))
		}
	}
	b.WriteString(" />")
	return b.String()
}

func (c *Client) sendPacket(p *Packet) {
	ps := buildPacket(p)
	c.sendMessage(ps)
}

func (c *Client) readPump() {
	defer func() {
		log.Println("read pump dead")
		c.hub.unregister <- c
		c.conn.Close()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error { _ = c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v\n", err)
			} else {
				log.Printf("error read: %v\n", err)
			}
			break
		}
		message = bytes.TrimSpace(bytes.Replace(message, newline, space, -1))
		if c.hub.debug {
			log.Println("read:", string(message))
		}
		packs := parse(message)
		for i := 0; i < len(packs); i++ {
			c.parsed <- packs[i]
		}
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		log.Println("write pump dead")
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel.
				log.Println("hub closed channel")
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				log.Printf("error write: %v\n", err)
				return
			}
			_, _ = w.Write(message)
			if c.hub.debug {
				log.Println("send:", string(message))
			}
			// Add queued chat messages to the current websocket message.
			n := len(c.send)
			for i := 0; i < n; i++ {
				_, _ = w.Write(newline)
				message = <-c.send
				_, _ = w.Write(message)
				if c.hub.debug {
					log.Println("send:", string(message))
				}
			}

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				log.Println("client died:", err)
				return
			}
		}
	}
}

func (c *Client) handlePump() {
	defer func() {
		log.Println("handle pump dead")
		c.conn.Close()
	}()
	for {
		select {
		case p, open := <-c.parsed:
			if !open {
				return
			}
			if h, ok := c.handlers[p.tag]; ok {
				h(c, p)
			}
			break
		}
	}
}

func (c *Client) doHandlers() {
	c.handlers = defaultHandlers
}

func (c *Client) handle(tag string, f PacketHandler) {
	c.handlers[tag] = f
}

func (c *Client) isDone() bool {
	return c.done
}

func (c *Client) sendPC(i int, s string) {
	c.sendPacket(&Packet{
		tag: "p",
		attr: PacketAttr{
			"u": i,
			"t": s,
			"s": 2,
			"d": c.hub.botInfo["i"],
		},
		order: []string{"u", "t", "s", "d"},
	})
}

func (c *Client) isMaster() bool {
	return c.master
}

func (c *Client) setMaster(b bool) {
	c.master = b
}
