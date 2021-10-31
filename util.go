package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

func Find(a []string, x string) int {
	for i, n := range a {
		if x == n {
			return i
		}
	}
	return len(a)
}

func fetchGlobalBotInfo() map[string]string {
	_ = godotenv.Load()

	//loc, _ := time.LoadLocation("GMT")
	//if d, err := sToTime(os.Getenv("dt")); err != nil || time.Now().In(loc).Sub(d).Hours() >= 48 {
	//	log.Println("logging in", err, time.Now().Sub(d).Hours())
	if os.Getenv("dt") == "" {
		log.Println("logging in")
		return login(os.Getenv("username"), os.Getenv("password"))
	}
	env, err := godotenv.Read()
	if err != nil {
		log.Fatal("Could not load .env file", err)
	}
	return env
}

type LoginRes struct {
	V string `json:"v"`
}

func login(username string, password string) map[string]string {
	body := strings.NewReader(fmt.Sprintf(`json={"M": "0", "P": "", "d": "4450ee855cb4a3230106be1eb0b241e2", "n": "%v", "nfy": "", "oi": "", "p": "%v", "pt": "3", "t": ""}`, username, password))
	req, err := http.NewRequest("POST", "https://xat.com/web_gear/chat/mlogin2.php?v=1.55.4&m=7&", body)
	if err != nil {
		// handle err
		log.Fatal("http err:", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/92.0.4515.131 Safari/537.36")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "https://xat.com")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		// handle err
		log.Fatal("could not login:", err)
	}
	defer resp.Body.Close()

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal("could not read login response")
	}

	var loginResJSON LoginRes
	err = json.Unmarshal(content, &loginResJSON)
	if err != nil {
		log.Fatal("could not unmarshall login response:", err)
	}

	env, err := godotenv.Read()
	if err != nil {
		log.Fatal("could not load .env file")
	}

	loginResPacket := parse([]byte(loginResJSON.V))[0]
	for k, v := range loginResPacket.attr {
		env[k] = v.(string)
	}
	err = godotenv.Write(env, "./.env")
	if err != nil {
		log.Fatal("could not save .env file")
	}
	return env
}

func findChat(hub *Hub, name string) int {
	for n, id := range hub.chatCache {
		if strings.EqualFold(n, name) {
			return id
		}
	}
	req, err := http.NewRequest("GET", "https://xat.com/web_gear/chat/roomid.php?v2&d="+name, nil)
	if err != nil {
		// handle err
		return -1
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/92.0.4515.131 Safari/537.36")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "https://xat.com")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		// handle err
		return -1
	}
	defer resp.Body.Close()
	res, err := io.ReadAll(resp.Body)
	if err != nil {
		return -1
	}
	if strings.HasPrefix(string(res), "0") {
		log.Println("error getting chatid:", name)
		return -1
	}
	var cl chatLookup
	err = json.Unmarshal(res, &cl)
	if err != nil {
		return -1
	}
	//spew.Dump(cl)
	atoi, err := strconv.Atoi(cl.ID)
	if err != nil {
		log.Println("couldnt convert chatid to int:", cl.ID, err)
		return -1
	}
	hub.mutex.Lock()
	hub.chatCache[name] = atoi
	hub.save()
	hub.mutex.Unlock()
	return atoi
}

type chatLookup struct {
	ID  string `json:"id"`
	D   string `json:"d"`
	G   string `json:"g"`
	A   string `json:"a"`
	Bot int    `json:"bot"`
}
