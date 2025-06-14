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
	"github.com/charmbracelet/bubbles/viewport"
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
	msgViewStyle = lipgloss.NewStyle().
			Align(lipgloss.Left).
			Background(lipgloss.Color("#000"))

	textInputStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			Align(lipgloss.Left, lipgloss.Center).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("69"))
)

type mainModel struct {
	textInput textinput.Model
	msgLog    viewport.Model
	focus     focusState

	currentRoom string
	messages    chan string
	conn        *websocket.Conn
}

type socketMsg string
type socketErr struct {
	msg string
	err error
}

var msgMutex sync.Mutex
var messages []string

func initialModel(conn *websocket.Conn, room string) mainModel {
	// create the text input
	ti := textinput.New()
	ti.Prompt = ">"
	ti.Placeholder = "Send a message ..."
	w, h, err := term.GetSize(int(os.Stdout.Fd()))
	ti.Width = 30
	if err == nil {
		ti.Width = w - 5
	}

	ti.Cursor.Style = ti.Cursor.Style.UnsetBackground()
	ti.Focus()

	// create the viewport
	vp := viewport.New(w, h-ti.Cursor.Style.GetHeight()-10)

	vp.SetContent(lipgloss.NewStyle().Width(vp.Width).Render(fmt.Sprintf("Welcome to Aphasia!\nYou are currently connected to room %s, type a message and hit enter to chat", room)))
	model := mainModel{
		textInput:   ti,
		focus:       inputFocus,
		messages:    make(chan string, 256),
		conn:        conn,
		currentRoom: room,
		msgLog:      vp,
	}

	go getBroadcast(conn, model)
	return model
}

func (m mainModel) Init() tea.Cmd {
	// return tea.Batch(getBroadcast(m.conn, m), textarea.Blink)
	return tea.Batch(textarea.Blink, tea.SetWindowTitle("Aphasia"))
}

func (m mainModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var tiCmd tea.Cmd
	var vpCmd tea.Cmd
	m.textInput, tiCmd = m.textInput.Update(msg)
	m.msgLog, vpCmd = m.msgLog.Update(msg)

	var cmds []tea.Cmd
	cmds = append(cmds, tiCmd)
	cmds = append(cmds, vpCmd)

	switch msgData := msg.(type) {
	case tea.KeyMsg:
		switch msgData.Type {
		case tea.KeyCtrlJ:
			m.focus = inputFocus
			m.textInput.Focus()
		case tea.KeyCtrlK:
			m.focus = msgFocus
			m.textInput.Blur()
		case tea.KeyEnter:
			outBoundMsg := m.textInput.Value()
			m.textInput.Reset()
			if outBoundMsg == "quit" {
				mut.Lock()
				quit = true
				mut.Unlock()
				return m, tea.Quit
			} else {
				if (len(outBoundMsg) > 10 && outBoundMsg[:10] == "switch to ") ||
					(outBoundMsg == "quit") || (outBoundMsg == "clear") {
					msgOut = ""
				}
				if outBoundMsg != "clear" {
					m.conn.WriteMessage(websocket.TextMessage, []byte(outBoundMsg))
				}
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
	output := " Aphasia: The chatting service for people who can't (seriously, go touch grass)\n"

	for range len(m.messages) {
		msgOut += fmt.Sprintln(<-m.messages)
	}
	m.msgLog.SetContent(lipgloss.NewStyle().Width(m.msgLog.Width).Render(msgOut))
	m.msgLog.GotoBottom()
	if m.focus == inputFocus {
		// m.textInput.TextStyle = m.textInput.TextStyle.Background(lipgloss.Color("#000"))
		// textInputStyle = textInputStyle.Background(lipgloss.Color("#000"))

		m.msgLog.Style = m.msgLog.Style.UnsetBackground()
		msgViewStyle = msgViewStyle.UnsetBackground()

		output += lipgloss.JoinVertical(lipgloss.Center, msgViewStyle.Render(m.msgLog.View()), textInputStyle.Render(m.textInput.View()))
		// output += lipgloss.JoinVertical(lipgloss.Center, msgViewStyle.Render(msgOut), textInputStyle.Render(m.textInput.View()))
	} else {
		// m.textInput.TextStyle = m.textInput.TextStyle.UnsetBackground()
		// textInputStyle = textInputStyle.UnsetBackground()

		m.msgLog.Style = m.msgLog.Style.Background(lipgloss.Color("#000"))
		msgViewStyle = msgViewStyle.Background(lipgloss.Color("#000"))

		output += lipgloss.JoinVertical(lipgloss.Center, msgViewStyle.Render(m.msgLog.View()), textInputStyle.Render(m.textInput.View()))
		// output += lipgloss.JoinVertical(lipgloss.Center, msgViewStyle.Render(msgOut), textInputStyle.Render(m.textInput.View()))
	}
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
	textInputStyle = textInputStyle.Width(w - 3).Height(1)
	msgViewStyle = msgViewStyle.Width(w - 3).Height(h - 4)
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

	model := initialModel(c, header.Get("room"))
	program := tea.NewProgram(model)

	if _, err := program.Run(); err != nil {
		fmt.Println("There was an error starting the client: ", err)
	}
}
