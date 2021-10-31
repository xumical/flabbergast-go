package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"sync"
)

// Hub maintains the set of active clients and broadcasts messages to the
// clients.
type Hub struct {
	// Registered clients.
	clients map[*Client]bool

	// Inbound messages from the clients.
	broadcast chan []byte

	// Register requests from the clients.
	register chan *Client

	// Unregister requests from clients.
	unregister   chan *Client
	latestSpawns map[int]string
	botInfo      map[string]string
	chatCache    map[string]int
	debug        bool
	mutex        *sync.Mutex
}

func newHub() *Hub {
	return &Hub{
		broadcast:    make(chan []byte),
		register:     make(chan *Client),
		unregister:   make(chan *Client),
		clients:      make(map[*Client]bool),
		latestSpawns: make(map[int]string),
		botInfo:      make(map[string]string),
		chatCache:    make(map[string]int),
		mutex:        &sync.Mutex{},
	}
}

func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
			log.Printf("[NEW] %v (%v) | Tot: %v\n", client.chatName, client.chatId, len(h.clients))
		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
				close(client.parsed)
				log.Printf("[DEAD] %v (%v) | Tot: %v\n ", client.chatName, client.chatId, len(h.clients))
				if client.isMaster() {
					go func() {
						MasterBot = newBot(client.chatName, h)
						MasterBot.setMaster(true)
					}()
				}
			}
		case message := <-h.broadcast:
			log.Println("msg:", string(message))
			for client := range h.clients {
				if !client.isDone() {
					continue
				}
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
		}
	}
}

func (h *Hub) restore() {
	path := ".cache/"
	fn := "chat"
	f, err := os.Open(path + fn)
	if err != nil {
		log.Println("no open:", err)
		return
	}
	defer f.Close()
	err = json.NewDecoder(f).Decode(&h.chatCache)
	if err != nil {
		log.Println("decode:", err)
	}
}

func (h *Hub) save() {
	// rename main file -> .bk
	// write data to new file
	// if err, delete new file, rename .bk to old

	// backup
	//f, err := os.OpenFile(".cache/chat", os.O_RDWR|os.O_CREATE, 0644)
	//if err != nil {
	//	log.Fatal(err)
	//}
	//bck, err := os.Create(".cache/chat.bk")
	//if err != nil {
	//	log.Fatal(err)
	//}
	//_, err = io.Copy(bck, f)
	//if err != nil {
	//	log.Fatal(err)
	//}
	//err = bck.Sync()
	//if err != nil {
	//	log.Fatal(err)
	//}
	//err = f.Truncate(0)
	//if err != nil {
	//	log.Fatal(err)
	//}
	//bv, _ := json.Marshal(h.chatCache)
	//write, err := f.Write(bv)
	//if write != len(bv) {
	//	log.Printf("could not write all data")
	//}
	//if err != nil {
	//	log.Fatal(err)
	//}
	//if err != nil {
	//	log.Printf("could not save, reverting\n", err)
	//	_, err = bck.Seek(0, io.SeekStart)
	//	if err != nil {
	//		log.Fatal("jinkies, its doomed")
	//	}
	//	_, err = io.Copy(f, bck)
	//	if err != nil {
	//		log.Fatal(err)
	//	}
	//}

	fn := ".cache/chat"
	bx := ".bk"
	// attempt rename
	if err := os.Rename(fn, fn+bx); err != nil {
		log.Println(err)
		return
	}
	// create new file
	bv, _ := json.Marshal(h.chatCache)
	err := ioutil.WriteFile(fn, bv, 0644)
	if err != nil {
		log.Println("could not save, reverting:", err)
		err = os.Remove(fn)
		if err != nil {
			log.Fatal(err)
		}
		err = os.Rename(fn+bx, fn)
		if err != nil {
			log.Fatal(err)
		}
	}
	_ = os.Remove(fn + bx)
}
