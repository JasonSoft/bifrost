package main

import (
	"bytes"
	"compress/gzip"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"math"
	"net"
	"time"
)

const (
	defaultConnection      = "lan"
	defaultMaxChunkSizeWan = 1420
	defaultMaxChunkSizeLan = 8154
)

type gelfMessage struct {
	Version      string
	Host         string
	Level        int
	ShortMessage string
	FullMessage  string
	Timestamp    float64
	Facility     string
	LoggerName   string
	CustomFields map[string]interface{}
}

func newGelfMessage(host string, appName string, loggerName string, level int) *gelfMessage {
	return &gelfMessage{
		Version:      "1.1",
		LoggerName:   loggerName,
		Host:         host,
		Facility:     appName,
		Timestamp:    float64(time.Now().UnixNano()) / float64(time.Second),
		Level:        level,
		CustomFields: make(map[string]interface{}),
	}
}

func (m *gelfMessage) toByte() []byte {
	items := make(map[string]interface{})
	items["version"] = m.Version
	items["host"] = m.Host
	items["level"] = m.Level
	items["short_message"] = m.ShortMessage
	items["full_message"] = m.FullMessage
	items["timestamp"] = m.Timestamp
	items["_app_id"] = m.Facility
	items["_logger_name"] = m.LoggerName

	for k, v := range m.CustomFields {
		items["_"+k] = v
	}

	payload, _ := json.Marshal(items)
	return payload
}

type gelfConfig struct {
	ConnectionString string
	Connection       string
	MaxChunkSizeWan  int
	MaxChunkSizeLan  int
}

type gelf struct {
	conn   net.Conn
	writer *gzip.Writer
	gelfConfig
}

func newGelf(config gelfConfig) *gelf {
	gz, err := gzip.NewWriterLevel(ioutil.Discard, gzip.BestSpeed)
	if err != nil {
		panic(err)
	}

	if len(config.ConnectionString) == 0 {
		config.ConnectionString = "127.0.0.1:12201"
	}
	if config.Connection == "" {
		config.Connection = defaultConnection
	}
	if config.MaxChunkSizeWan == 0 {
		config.MaxChunkSizeWan = defaultMaxChunkSizeWan
	}
	if config.MaxChunkSizeLan == 0 {
		config.MaxChunkSizeLan = defaultMaxChunkSizeLan
	}

	udpConn, err := net.Dial("udp", config.ConnectionString)
	if err != nil {
		panic(err)
	}

	g := &gelf{
		conn:       udpConn,
		writer:     gz,
		gelfConfig: config,
	}

	return g
}

func (g *gelf) log(data []byte) {
	/*
		msgJson := g.parseJson(message)

		err := g.testForForbiddenValues(msgJson)
		if err != nil {
			log.Printf("Uh oh! %s", err)
			return
		}
	*/
	compressed := g.compress(data)
	/*
		compressed := []byte(message)
		_logger.debug(compressed)
		g.send(compressed)
	*/
	chunksize := g.gelfConfig.MaxChunkSizeLan
	length := compressed.Len()

	if length > chunksize {

		chunkCountInt := int(math.Ceil(float64(length) / float64(chunksize)))

		id := make([]byte, 8)
		rand.Read(id)

		for i, index := 0, 0; i < length; i, index = i+chunksize, index+1 {
			packet := g.createChunkedMessage(index, chunkCountInt, id, &compressed)
			g.send(packet.Bytes())
		}

	} else {
		g.send(compressed.Bytes())
	}

}

func (g *gelf) createChunkedMessage(index int, chunkCountInt int, id []byte, compressed *bytes.Buffer) bytes.Buffer {
	var packet bytes.Buffer

	chunksize := g.getChunksize()

	packet.Write(g.intToBytes(30))
	packet.Write(g.intToBytes(15))
	packet.Write(id)

	packet.Write(g.intToBytes(index))
	packet.Write(g.intToBytes(chunkCountInt))

	packet.Write(compressed.Next(chunksize))

	return packet
}

func (g *gelf) getChunksize() int {

	if g.gelfConfig.Connection == "wan" {
		return g.gelfConfig.MaxChunkSizeWan
	}

	if g.gelfConfig.Connection == "lan" {
		return g.gelfConfig.MaxChunkSizeLan
	}

	return g.gelfConfig.MaxChunkSizeWan
}

func (g *gelf) intToBytes(i int) []byte {
	buf := new(bytes.Buffer)

	err := binary.Write(buf, binary.LittleEndian, int8(i))
	if err != nil {
		log.Printf("Uh oh! %s", err)
	}
	return buf.Bytes()
}

func (g *gelf) compress(b []byte) bytes.Buffer {
	var buf bytes.Buffer
	comp := gzip.NewWriter(&buf)

	comp.Write(b)
	comp.Close()

	return buf
}

func (g *gelf) parseJson(msg string) map[string]interface{} {
	var i map[string]interface{}
	c := []byte(msg)

	json.Unmarshal(c, &i)

	return i
}

func (g *gelf) testForForbiddenValues(gmap map[string]interface{}) error {
	if _, err := gmap["_id"]; err {
		return errors.New("Key _id is forbidden")
	}

	return nil
}

func (g *gelf) send(b []byte) {
	//time.Sleep(1 * time.Second)
	g.conn.Write(b)
}
