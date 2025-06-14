package main

import (
	"bufio"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gorilla/websocket"
)

var (
	focusedStyle = lipgloss.NewStyle()
)

type socketMsg string
type socketErr error

func getBroadcast(c *websocket.Conn) tea.Cmd {
	for {
		mut.Lock()
		if quit {
			fmt.Println("Exiting client")
			return func() tea.Msg {
				return socketMsg("Exiting client")
			}
		}
		mut.Unlock()
		m_type, data, err := c.ReadMessage()
		if err != nil {
			fmt.Println("Smt went wrong trying to read: ", err)
			return func() tea.Msg {
				return socketMsg(fmt.Sprintf("Smt went wrong trying to read: %s\n", err))
			}
		} else if m_type != websocket.TextMessage {
			fmt.Println("Not a text message")
			return
		} else {
			fmt.Printf("\n%s\n> ", data)
		}
	}

}
func fillHeader(header *http.Header) {
	scanner := bufio.NewScanner(os.Stdin)
	headerNames := []string{"name", "room"}
	count := 0
	fmt.Printf("Enter a %s >", headerNames[count])
	for scanner.Scan() {
		val := scanner.Text()
		if val == "" {
			switch count {
			case 0:
				val = fmt.Sprintf("anon%d", rand.Int63())
			case 1:
				val = "1"
			}
		}
		header.Set(headerNames[count], val)
		count += 1
		if count >= len(headerNames) {
			break
		}
		fmt.Printf("Enter a %s >", headerNames[count])
	}
}

var quit bool
var mut sync.Mutex

func main() {
	u := url.URL{Scheme: "ws", Host: "localhost:8080", Path: "ws"}
	fmt.Println("Connecting to: ", u.String())
	header := http.Header{}
	scanner := bufio.NewScanner(os.Stdin)
	fillHeader(&header)

	c, _, err := websocket.DefaultDialer.Dial(u.String(), header)

	if err != nil {
		fmt.Println("There was an error connecting to the URL: ", err)
	}
	defer c.Close()

	readBuf := make(chan string, 200)

	//process inputs and read the output from the server here
	go getBroadcast(c)
	var msg string = ""
	mut = sync.Mutex{}
	for {
		fmt.Print("> ")
		for scanner.Scan() {
			msg = scanner.Text()
			readBuf <- msg
			c.WriteMessage(websocket.TextMessage, []byte(msg))
			c.WriteMessage(websocket.PingMessage, []byte(""))
			if msg == "quit" {
				mut.Lock()
				quit = true
				mut.Unlock()
				return
			}
		}
	}
}
