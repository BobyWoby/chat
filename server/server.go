package main

import (
	"fmt"
	"net/http"

	"github.com/gorilla/websocket"
)

type Room struct {
	name       string           // the name/ID of the room
	clients    map[*Client]bool // a map of all of the clients in the room
	broadcast  chan []byte      // a buffer of the messages that need to be broadcasted
	register   chan *Client     // buffers the clients that need to be registered
	unregister chan *Client     // buffers the clients that need to be unregistered
	active     bool             // if the room is currently running
}

type Client struct {
	name string
	room Room
	conn *websocket.Conn
	send chan []byte
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
		// return r.Header.Get("name") != ""
	},
}

// read from the client
func readPump(c *Client) {
	for {
		if m_type, msg, err := c.conn.ReadMessage(); err == nil {
			if m_type != websocket.TextMessage {
				fmt.Println("Read Data isn't a string")
				break
			} else {
				fmt.Printf("Read: %s\n", msg)
				str := string(msg)
				if str == "quit" {
					c.room.unregister <- c
					break
				} else if len(str) > 10 && str[:10] == "switch to " {
					if room, ok := rooms[str[10:]]; ok {
						c.room.unregister <- c
						room.register <- c
						c.room = *room
					}
				} else {
					c.room.broadcast <- fmt.Appendf(nil, "%s: %s", c.name, string(msg))
				}
			}
		} else {
			fmt.Println("Read Error: ", err)
			c.room.unregister <- c
			break
		}
	}
}

// write to the client
// to avoid any deadlock errors, or cases where there are multiple accesses of the writer
// exclusively write through this function
func writePump(c *Client) {
	defer func() {
		//close the connection if somethign goes wrong
		c.conn.Close()
	}()
	//send the room data to the client
	keys := ""
	for room := range rooms {
		keys += fmt.Sprintln(room)
	}
	c.conn.WriteMessage(websocket.BinaryMessage, []byte(keys))
	for {
		w, err := c.conn.NextWriter(websocket.TextMessage)
		if err != nil {
			fmt.Println("Error creating Writer!")
			break
		}

		// go through the backlog of messages that need to be sent to the client
		msg := <-c.send
		w.Write(msg)
		n := len(c.send)
		for range n {
			msg = <-c.send
			w.Write([]byte("\n"))
			w.Write(msg)
		}
		err = w.Close()
		if err != nil {
			fmt.Println("There was an error flushing the writer! ", err)
			break
		}
	}
}

func handleWs(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println("Error upgrading connection!")
		return
	}
	name := r.Header.Get("name")
	roomName := r.Header.Get("room")
	if room, ok := rooms[roomName]; ok {
		fmt.Println(name, "has connected")
		client := &Client{name: name, room: *rooms[roomName], conn: conn, send: make(chan []byte, 256)}
		room.register <- client
		go readPump(client)
		go writePump(client)
	} else {
		conn.WriteMessage(websocket.TextMessage, []byte("Not a valid Room!"))
		return
	}

}
func (room *Room) run() {
	room.active = true
	for {
		select {
		// if theres a client that needs to be registered, register them
		case client := <-room.unregister:
			msg := fmt.Sprintf("%s has left the room!", client.name)
			fmt.Println(msg)
			room.broadcast <- []byte(msg)
			delete(room.clients, client)

		// if theres a client that needs to be unregistered, unregister them
		case client := <-room.register:
			msg := fmt.Sprintf("%s has joined room %s", client.name, room.name)
			fmt.Println(msg)
			room.broadcast <- []byte(msg)
			room.clients[client] = true

		// broadcast any messages that have been buffered
		case msg := <-room.broadcast:
			for client := range room.clients {
				select {
				case client.send <- msg:
					fmt.Println("Broadcasting: ", string(msg))
				default:
					// if the client cant buffer the message, unregister them
					room.unregister <- client
				}
			}
		}

	}
}
func newRoom(roomName string) *Room {
	return &Room{
		name:       roomName,
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		active:     false,
	}
}

var rooms map[string]*Room

func main() {
	rooms = make(map[string]*Room)
	defaultRoom := newRoom("1")
	room2 := newRoom("2")
	rooms["1"] = defaultRoom
	rooms[room2.name] = room2

	for _, room := range rooms {
		go room.run()
	}
	// go defaultRoom.run()

	http.HandleFunc("/ws", handleWs)
	fmt.Println("Websocket Server started on :8080")
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		fmt.Println("There was an error starting the server", err)
	}

}
