package main

import (
	"bufio"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"sync"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gorilla/websocket"
	"golang.org/x/term"
)

const (
	inputFocus focusState = iota
	msgFocus
)

type focusState int

// Lipgloss styles for each of the windows
var (
	modelStyle = lipgloss.NewStyle().
			Align(lipgloss.Left).
			BorderForeground(lipgloss.Color("#fff"))
	focusedStyle = lipgloss.NewStyle().
			Align(lipgloss.Left, lipgloss.Center).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("69"))
)

type mainModel struct {
	textInput textinput.Model
	focus     focusState

	messages chan string
	conn     *websocket.Conn
}

type socketMsg string
type socketErr struct {
	msg string
	err error
}

var msgMutex sync.Mutex
var messages []string

func initialModel(conn *websocket.Conn) mainModel {
	ti := textinput.New()
	ti.Prompt = ">"
	ti.Placeholder = "Send a message ..."
	w, _, err := term.GetSize(int(os.Stdout.Fd()))
	ti.Width = 30
	if err == nil {
		ti.Width = w - 5
	}
	ti.Focus()
	model := mainModel{
		textInput: ti,
		focus:     inputFocus,
		messages:  make(chan string, 256),
		conn:      conn,
	}

	go getBroadcast(conn, model)
	return model
}

func (m mainModel) Init() tea.Cmd {
	// return tea.Batch(getBroadcast(m.conn, m), textarea.Blink)
	return tea.Batch(textarea.Blink)
}

func (m mainModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var tiCmd tea.Cmd
	m.textInput, tiCmd = m.textInput.Update(msg)
	var cmds []tea.Cmd
	cmds = append(cmds, tiCmd)

	switch msgData := msg.(type) {
	case tea.KeyMsg:
		switch msgData.Type {
		// case tea.KeyCtrlJ:
		// 	m.focus = inputFocus
		// case tea.KeyCtrlK:
		// 	m.focus = msgFocus
		case tea.KeyEnter:
			outBoundMsg := m.textInput.Value()
			m.textInput.Reset()
			if outBoundMsg == "quit" {
				mut.Lock()
				quit = true
				mut.Unlock()
				return m, tea.Quit
			} else {
				m.conn.WriteMessage(websocket.TextMessage, []byte(outBoundMsg))
			}
		case tea.KeyCtrlC:
			mut.Lock()
			quit = true
			mut.Unlock()
			return m, tea.Quit
		}
	}
	return m, tea.Batch(cmds...)
}

var msgOut string

func (m mainModel) View() string {
	output := "Aphasia: The chatting service for people who can't (seriously, go touch grass)\n"

	for range len(m.messages) {
		msgOut += fmt.Sprintln(<-m.messages)
		// fmt.Println(msgOut)
	}
	// fmt.Println(modelStyle.Render(msgOut))
	output += lipgloss.JoinVertical(lipgloss.Left, modelStyle.Render(msgOut), focusedStyle.Render(m.textInput.View()))

	// if m.focus == inputFocus {
	// 	output += lipgloss.JoinVertical(lipgloss.Left, modelStyle.Render(msgOut), focusedStyle.Render(m.textInput.View()))
	// } else {
	// 	output += lipgloss.JoinVertical(lipgloss.Left, focusedStyle.Render(msgOut), modelStyle.Render(m.textInput.View()))
	// }
	return output
}

func getBroadcast(c *websocket.Conn, m mainModel) {
	defer func() {
		c.Close()
	}()
	for {
		mut.Lock()
		if quit {
			fmt.Println("Exiting client")
			return
		}
		mut.Unlock()
		m_type, data, err := c.ReadMessage()
		if err != nil {
			fmt.Println("Smt went wrong trying to read: ", err)
			return
		} else if m_type != websocket.TextMessage {
			fmt.Println("Not a text message")
			return
		} else {
			m.messages <- string(data)
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
	w, h, e := term.GetSize(int(os.Stdout.Fd()))
	if e != nil {
		return
	}
	focusedStyle = focusedStyle.Width(w - 3).Height(1)
	modelStyle = modelStyle.Width(w - 3).Height(h - 4)
	u := url.URL{Scheme: "ws", Host: "localhost:8080", Path: "ws"}
	fmt.Println("Connecting to: ", u.String())
	header := http.Header{}
	// header.Set("name", "bobywoby")
	// header.Set("room", "1")
	fillHeader(&header)

	c, _, err := websocket.DefaultDialer.Dial(u.String(), header)

	if err != nil {
		fmt.Println("There was an error connecting to the URL: ", err)
	}
	defer c.Close()

	model := initialModel(c)
	program := tea.NewProgram(model)

	if _, err := program.Run(); err != nil {
		fmt.Println("There was an error starting the client: ", err)
	}
}
