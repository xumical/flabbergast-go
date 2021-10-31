package main

import (
	"io"
	"log"
	"strings"

	"golang.org/x/net/html"
)

type PacketAttr map[string]interface{}

type Packet struct {
	tag   string
	attr  PacketAttr
	order []string
}

type PacketHandler func(c *Client, p *Packet)

func (p *Packet) addOrder(o ...string) {
	p.order = append(p.order, o...)
}

func (p *Packet) hasAttrib(key string) (ok bool) {
	if _, ok = p.attr[key]; ok {
		return
	}
	return
}

func (p *Packet) getAttrib(key string) string {
	return p.attr[key].(string)
}

func parse(bytes []byte) (packets []*Packet) {
	packets = []*Packet{}

	reader := strings.NewReader(string(bytes))
	tokenizer := html.NewTokenizer(reader)
	for {
		tt := tokenizer.Next()
		if tt == html.ErrorToken {
			if tokenizer.Err() == io.EOF {
				return
			}
			log.Printf("Error: %v\n", tokenizer.Err())
			return
		}
		tag, hasAttr := tokenizer.TagName()
		//fmt.Printf("Tag: %v\n", string(tag))
		p := &Packet{
			tag:  string(tag),
			attr: make(PacketAttr),
		}
		if hasAttr {
			for {
				attrKey, attrValue, moreAttr := tokenizer.TagAttr()
				p.attr[string(attrKey)] = string(attrValue)
				//fmt.Printf("\t %v = %v\n", string(attrKey), string(attrValue))
				if !moreAttr {
					break
				}
			}
		}
		packets = append(packets, p)
	}
}
