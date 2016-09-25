// Copyright 2013 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
    "bytes"
	"encoding/json"
	"log"
	"net/http"
	"time"
	"strconv"

	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 2 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 512
)

var (
	newline = []byte{'\n'}
	space   = []byte{' '}
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// Client is an middleman between the websocket connection and the hub.
type Client struct {
	hub *Hub

	// The websocket connection.
	conn *websocket.Conn

	// Buffered channel of outbound messages.
	send chan []byte
}

// readPump pumps messages from the websocket connection to the hub.
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway) {
				log.Printf("error: %v", err)
			}
			break
		}
		message = bytes.TrimSpace(bytes.Replace(message, newline, space, -1))
        // do processing here!

        var socketData SocketData
		json.Unmarshal([]byte(message), &socketData)
		var data []byte
        var broadcast = true
		switch socketData.Action {
			case "update ui":
				data, _ = json.Marshal(SocketData{Action: "update ui", Data: UpdateUi()})
                broadcast = false
			case "set volume":
				SetVolume(int(socketData.Data.(float64)))
				data, _ = json.Marshal(SocketData{Action: "update ui", Data: UpdateUi()})
			case "load playlist":
				LoadPlaylist(socketData.Data.(int))
				StartPlayer(-1)
				data, _ = json.Marshal(SocketData{Action: "update ui", Data: UpdateUi()})
			case "play playlist entry":
				var playlistMedia PlaylistMedia
				json.Unmarshal([]byte(socketData.Data.(string)), &playlistMedia)
				PlayMediaFromPlaylist(playlistMedia.MediaID, playlistMedia.PlaylistID)
				data, _ = json.Marshal(SocketData{Action: "update ui", Data: UpdateUi()})
			case "list media":
				if socketData.Data == nil {
					socketData.Data = 0
				}
				data, _ = json.Marshal(SocketData{Action: "media list", Data: GetAllMedia(socketData.Data.(int))})
                broadcast = false
			case "play media":
				var media Media
				json.Unmarshal([]byte(socketData.Data.(string)), &media)
				ClearQueue()
				AddToQueue(media.FileName)
				StartPlayer(-1)
				data, _ = json.Marshal(SocketData{Action: "update ui", Data: UpdateUi()})
			case "add to up next":
				var media Media
				json.Unmarshal([]byte(socketData.Data.(string)), &media)
				if currentPlaylist.ID != 0 {
					ClearQueue()
				}
				AddToQueue(media.FileName)
				StartPlayer(-1)
				data, _ = json.Marshal(SocketData{Action: "update ui", Data: UpdateUi()})
			case "start player":
				StartPlayer(-1)
				data, _ = json.Marshal(SocketData{Action: "update ui", Data: UpdateUi()})
			case "next":
				Next()
				data, _ = json.Marshal(SocketData{Action: "update ui", Data: UpdateUi()})
			case "prev":
				Previous()
				data, _ = json.Marshal(SocketData{Action: "update ui", Data: UpdateUi()})
			case "toggle pause":
				TogglePause()
				data, _ = json.Marshal(SocketData{Action: "update ui", Data: UpdateUi()})
            case "toggle random":
                ToggleRandom()
                data, _ = json.Marshal(SocketData{Action: "update ui", Data: UpdateUi()})
            case "set repeat":
                Repeat(int(socketData.Data.(float64)))
                data, _ = json.Marshal(SocketData{Action: "update ui", Data: UpdateUi()})
			case "play queued song":
				pos, _ := strconv.Atoi(socketData.Data.(string))
				StartPlayer(pos)
				data, _ = json.Marshal(SocketData{Action: "update ui", Data: UpdateUi()})
		}
        if broadcast {
            c.hub.broadcast <- data
        } else {
            c.send <- data
        }
	}
}

// write writes a message with the given message type and payload.
func (c *Client) write(mt int, payload []byte) error {
	c.conn.SetWriteDeadline(time.Now().Add(writeWait))
	return c.conn.WriteMessage(mt, payload)
}

// writePump pumps messages from the hub to the websocket connection.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				// The hub closed the channel.
				c.write(websocket.CloseMessage, []byte{})
				return
			}

			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued chat messages to the current websocket message.
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write(newline)
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			if err := c.write(websocket.PingMessage, []byte{}); err != nil {
				return
			}
		}
	}
}

// serveWs handles websocket requests from the peer.
func serveWs(hub *Hub, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	client := &Client{hub: hub, conn: conn, send: make(chan []byte, 256)}
	client.hub.register <- client
	go client.writePump()
	client.readPump()
}