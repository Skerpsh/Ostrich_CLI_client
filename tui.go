package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gorilla/websocket"
)

type loginSuccessMsg struct {
	result *LoginResponse
}

type loginErrorMsg struct {
	err error
}

type registerSuccessMsg struct {
	result *LoginResponse
}

type registerErrorMsg struct {
	err error
}

type chatsLoadedMsg struct {
	chats []Chat
}

type chatsErrorMsg struct {
	err error
}

type chatOpenedMsg struct {
	chat     Chat
	messages []Message
	conn     *websocket.Conn
}

type chatOpenErrorMsg struct {
	err error
}

type wsMessageMsg struct {
	message WSMessage
}

type wsErrorMsg struct {
	err error
}

type tuiStage int

const (
	stageLogin tuiStage = iota
	stageChats
	stageChat
)

type authMode int

const (
	authLogin authMode = iota
	authRegister
)

type tuiModel struct {
	width  int
	height int
	stage  tuiStage

	authMode authMode

	username        textinput.Model
	password        textinput.Model
	confirmPassword textinput.Model
	focus           int

	loading bool
	err     error

	user *LoginResponse

	chats    []Chat
	selected int

	currentChat  Chat
	messages     []Message
	messageInput textinput.Model

	conn *websocket.Conn
}

var (
	logoStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205")).
			MarginBottom(1)

	inputStyle = lipgloss.NewStyle().
			Width(40).
			Padding(0, 1)

	labelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("9"))

	successStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("10"))

	hintStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	selectedChatStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("205"))

	chatBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(1, 2)

	messageBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(1, 2)

	ownMessageStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("10"))

	otherMessageStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("205"))
)

func newLoginModel() tuiModel {
	username := textinput.New()
	username.Placeholder = "Username"
	username.CharLimit = 32
	username.Width = 40
	username.Focus()

	password := textinput.New()
	password.Placeholder = "Password"
	password.CharLimit = 128
	password.Width = 40
	password.EchoMode = textinput.EchoPassword
	password.EchoCharacter = '•'

	confirmPassword := textinput.New()
	confirmPassword.Placeholder = "Confirm password"
	confirmPassword.CharLimit = 128
	confirmPassword.Width = 40
	confirmPassword.EchoMode = textinput.EchoPassword
	confirmPassword.EchoCharacter = '•'

	messageInput := textinput.New()
	messageInput.Placeholder = "Type a message..."
	messageInput.CharLimit = 4096
	messageInput.Width = 60

	return tuiModel{
		stage:           stageLogin,
		authMode:        authLogin,
		username:        username,
		password:        password,
		confirmPassword: confirmPassword,
		focus:           0,
		messageInput:    messageInput,
	}
}

func loadChatsCmd(token string) tea.Cmd {
	return func() tea.Msg {
		chats, err := getChats(token)

		if err != nil {
			return chatsErrorMsg{err: err}
		}

		return chatsLoadedMsg{chats: chats}
	}
}

func openChatCmd(token string, chat Chat) tea.Cmd {
	return func() tea.Msg {
		messages, err := getMessages(token, chat.ID)
		if err != nil {
			return chatOpenErrorMsg{err: err}
		}

		conn, err := connectTUIWebSocket(token, chat.ID)
		if err != nil {
			return chatOpenErrorMsg{err: err}
		}

		return chatOpenedMsg{
			chat:     chat,
			messages: messages,
			conn:     conn,
		}
	}
}

func connectTUIWebSocket(token, chatID string) (*websocket.Conn, error) {
	conn, _, err := websocket.DefaultDialer.Dial(
		websocketURL+"?token="+token,
		nil,
	)

	if err != nil {
		return nil, err
	}

	var connected WSMessage

	if err := conn.ReadJSON(&connected); err != nil {
		conn.Close()
		return nil, err
	}

	if connected.Type != "connected" {
		conn.Close()

		return nil, fmt.Errorf(
			"unexpected websocket response: %s",
			connected.Type,
		)
	}

	if err := conn.WriteJSON(map[string]string{
		"type":   "join",
		"chatId": chatID,
	}); err != nil {
		conn.Close()
		return nil, err
	}

	var joined WSMessage

	if err := conn.ReadJSON(&joined); err != nil {
		conn.Close()
		return nil, err
	}

	if joined.Type != "joined" {
		conn.Close()
		return nil, fmt.Errorf("failed to join chat")
	}

	return conn, nil
}

func listenTUIWebSocket(conn *websocket.Conn) tea.Cmd {
	return func() tea.Msg {
		var message WSMessage

		if err := conn.ReadJSON(&message); err != nil {
			return wsErrorMsg{err: err}
		}

		return wsMessageMsg{message: message}
	}
}

func (m tuiModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case loginSuccessMsg:
		m.loading = true
		m.user = msg.result
		m.err = nil

		return m, loadChatsCmd(msg.result.Token)

	case loginErrorMsg:
		m.loading = false
		m.err = msg.err

		return m, nil

	case registerSuccessMsg:
		m.loading = true
		m.user = msg.result
		m.err = nil

		return m, loadChatsCmd(msg.result.Token)

	case registerErrorMsg:
		m.loading = false
		m.err = msg.err

		return m, nil

	case chatsLoadedMsg:
		m.loading = false
		m.chats = msg.chats
		m.stage = stageChats
		m.err = nil
		m.selected = 0

		return m, nil

	case chatsErrorMsg:
		m.loading = false
		m.err = msg.err

		return m, nil

	case chatOpenedMsg:
		m.loading = false
		m.currentChat = msg.chat
		m.messages = msg.messages
		m.conn = msg.conn
		m.stage = stageChat
		m.err = nil

		m.messageInput.SetValue("")
		m.messageInput.Focus()

		return m, listenTUIWebSocket(m.conn)

	case chatOpenErrorMsg:
		m.loading = false
		m.err = msg.err

		if m.conn != nil {
			m.conn.Close()
			m.conn = nil
		}

		return m, nil

	case wsMessageMsg:
		switch msg.message.Type {

		case "message":
			if msg.message.Message != nil {
				m.messages = append(
					m.messages,
					*msg.message.Message,
				)
			}

		case "error":
			m.err = fmt.Errorf(
				"server error: %s",
				msg.message.Error,
			)
		}

		if m.conn != nil {
			return m, listenTUIWebSocket(m.conn)
		}

		return m, nil

	case wsErrorMsg:
		m.err = fmt.Errorf(
			"websocket disconnected: %v",
			msg.err,
		)

		return m, nil

	case tea.KeyMsg:

		if msg.String() == "ctrl+c" {
			if m.conn != nil {
				m.conn.Close()
			}

			return m, tea.Quit
		}

		if m.stage == stageLogin {
			return m.updateLogin(msg)
		}

		if m.stage == stageChats {
			return m.updateChats(msg)
		}

		if m.stage == stageChat {
			return m.updateChat(msg)
		}
	}

	return m, nil
}

func (m tuiModel) updateLogin(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.loading {
		return m, nil
	}

	key := msg.String()

	switch key {

	case "q":
		return m, tea.Quit

	case "esc":
		if m.authMode == authRegister {
			m.authMode = authLogin
			m.focus = 0
			m.err = nil

			m.password.Blur()
			m.confirmPassword.Blur()
			m.username.Focus()

			return m, nil
		}

		return m, tea.Quit

	// TAB переключает Login <-> Register
	case "tab":
		if m.authMode == authLogin {
			m.authMode = authRegister
		} else {
			m.authMode = authLogin
		}

		m.focus = 0
		m.err = nil

		m.username.Focus()
		m.password.Blur()
		m.confirmPassword.Blur()

		return m, nil

	// SHIFT+TAB тоже переключает Login <-> Register назад
	case "shift+tab":
		if m.authMode == authLogin {
			m.authMode = authRegister
		} else {
			m.authMode = authLogin
		}

		m.focus = 0
		m.err = nil

		m.username.Focus()
		m.password.Blur()
		m.confirmPassword.Blur()

		return m, nil

	// Стрелки переключают поля
	case "up":
		maxFocus := 1

		if m.authMode == authRegister {
			maxFocus = 2
		}

		m.focus--

		if m.focus < 0 {
			m.focus = maxFocus
		}

		m.username.Blur()
		m.password.Blur()
		m.confirmPassword.Blur()

		switch m.focus {
		case 0:
			m.username.Focus()

		case 1:
			m.password.Focus()

		case 2:
			m.confirmPassword.Focus()
		}

		return m, nil

	case "down":
		maxFocus := 1

		if m.authMode == authRegister {
			maxFocus = 2
		}

		m.focus++

		if m.focus > maxFocus {
			m.focus = 0
		}

		m.username.Blur()
		m.password.Blur()
		m.confirmPassword.Blur()

		switch m.focus {
		case 0:
			m.username.Focus()

		case 1:
			m.password.Focus()

		case 2:
			m.confirmPassword.Focus()
		}

		return m, nil

	case "enter":
		username := strings.TrimSpace(m.username.Value())
		password := m.password.Value()

		if username == "" {
			m.err = fmt.Errorf("username is required")
			return m, nil
		}

		if password == "" {
			m.err = fmt.Errorf("password is required")
			return m, nil
		}

		if m.authMode == authRegister {

			confirmPassword := m.confirmPassword.Value()

			if confirmPassword == "" {
				m.err = fmt.Errorf("please confirm your password")
				return m, nil
			}

			if password != confirmPassword {
				m.err = fmt.Errorf("passwords do not match")
				return m, nil
			}

			m.loading = true
			m.err = nil

			return m, func() tea.Msg {
				result, err := registerWithCredentials(
					username,
					password,
				)

				if err != nil {
					return registerErrorMsg{err: err}
				}

				return registerSuccessMsg{
					result: result,
				}
			}
		}

		m.loading = true
		m.err = nil

		return m, func() tea.Msg {
			result, err := loginWithCredentials(
				username,
				password,
			)

			if err != nil {
				return loginErrorMsg{err: err}
			}

			return loginSuccessMsg{
				result: result,
			}
		}
	}

	var cmd tea.Cmd

	switch m.focus {

	case 0:
		m.username, cmd = m.username.Update(msg)

	case 1:
		m.password, cmd = m.password.Update(msg)

	case 2:
		m.confirmPassword, cmd = m.confirmPassword.Update(msg)
	}

	return m, cmd
}

func (m tuiModel) updateChats(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.loading {
		return m, nil
	}

	if len(m.chats) == 0 {
		return m, nil
	}

	switch msg.String() {

	case "q":
		return m, tea.Quit

	case "up", "k":
		if m.selected > 0 {
			m.selected--
		}

	case "down", "j":
		if m.selected < len(m.chats)-1 {
			m.selected++
		}

	case "enter":
		m.loading = true
		m.err = nil

		selectedChat := m.chats[m.selected]

		return m, openChatCmd(
			m.user.Token,
			selectedChat,
		)
	}

	return m, nil
}

func (m tuiModel) updateChat(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {

	case "esc":
		if m.conn != nil {
			m.conn.Close()
			m.conn = nil
		}

		m.messageInput.Blur()
		m.stage = stageChats
		m.err = nil

		return m, nil

	case "enter":
		content := strings.TrimSpace(
			m.messageInput.Value(),
		)

		if content == "" {
			return m, nil
		}

		if m.conn == nil {
			m.err = fmt.Errorf("websocket is not connected")
			return m, nil
		}

		err := m.conn.WriteJSON(map[string]string{
			"type":    "message",
			"chatId":  m.currentChat.ID,
			"content": content,
		})

		if err != nil {
			m.err = err
			return m, nil
		}

		m.messageInput.SetValue("")

		return m, nil
	}

	var cmd tea.Cmd

	m.messageInput, cmd = m.messageInput.Update(msg)

	return m, cmd
}

func (m tuiModel) View() string {
	switch m.stage {

	case stageLogin:
		return m.loginView()

	case stageChats:
		return m.chatsView()

	case stageChat:
		return m.chatView()
	}

	return ""
}

func (m tuiModel) loginView() string {
	var b strings.Builder

	b.WriteString("\n")

	b.WriteString(
		logoStyle.Render(
			"╔════════════════════╗\n" +
				"║    OSTRICH CLI     ║\n" +
				"╚════════════════════╝",
		),
	)

	b.WriteString("\n\n")

	if m.loading && m.user != nil {
		action := "Logged in"

		if m.authMode == authRegister {
			action = "Account created"
		}

		b.WriteString(
			successStyle.Render(
				"✓ " + action + " as " + m.user.User.Username,
			),
		)

		b.WriteString("\n\n")
		b.WriteString("Loading chats...")
		b.WriteString("\n")

		return b.String()
	}

	/*
		Authentication mode selector.
	*/

	loginLabel := "Login"
	registerLabel := "Register"

	if m.authMode == authLogin {
		loginLabel = "▶ Login"
	} else {
		registerLabel = "▶ Register"
	}

	b.WriteString(
		selectedChatStyle.Render(loginLabel),
	)

	b.WriteString("    ")

	if m.authMode == authRegister {
		b.WriteString(
			selectedChatStyle.Render(registerLabel),
		)
	} else {
		b.WriteString(registerLabel)
	}

	b.WriteString("\n\n")

	b.WriteString(labelStyle.Render("Username"))
	b.WriteString("\n")
	b.WriteString(inputStyle.Render(m.username.View()))
	b.WriteString("\n\n")

	b.WriteString(labelStyle.Render("Password"))
	b.WriteString("\n")
	b.WriteString(inputStyle.Render(m.password.View()))
	b.WriteString("\n")

	if m.authMode == authRegister {
		b.WriteString("\n")

		b.WriteString(
			labelStyle.Render("Confirm password"),
		)

		b.WriteString("\n")

		b.WriteString(
			inputStyle.Render(
				m.confirmPassword.View(),
			),
		)

		b.WriteString("\n")
	}

	b.WriteString("\n")

	if m.loading {
		if m.authMode == authRegister {
			b.WriteString("Creating account...\n")
		} else {
			b.WriteString("Connecting to Ostrich...\n")
		}
	} else {
		if m.authMode == authRegister {
			b.WriteString(
				hintStyle.Render(
					"Tab / ↑↓ switch   Enter register   Esc login   Ctrl+C quit",
				),
			)
		} else {
			b.WriteString(
				hintStyle.Render(
					"Tab / ↑↓ switch   Enter login   Esc quit   Ctrl+C quit",
				),
			)
		}

		b.WriteString("\n")
	}

	if m.err != nil {
		b.WriteString("\n")

		b.WriteString(
			errorStyle.Render(
				"Error: " + m.err.Error(),
			),
		)

		b.WriteString("\n")
	}

	return b.String()
}

func (m tuiModel) chatsView() string {
	var b strings.Builder

	b.WriteString("\n")

	title := "OSTRICH"

	if m.user != nil {
		title += "   @" + m.user.User.Username
	}

	b.WriteString(logoStyle.Render(title))
	b.WriteString("\n\n")

	var chatList strings.Builder

	chatList.WriteString("Chats\n\n")

	if len(m.chats) == 0 {
		chatList.WriteString("No chats.")
	} else {
		for i, chat := range m.chats {

			cursor := "  "

			if i == m.selected {
				cursor = "▶ "
			}

			line := fmt.Sprintf(
				"%s%s\n  %s\n\n",
				cursor,
				chat.Username,
				chat.LoginID,
			)

			if i == m.selected {
				chatList.WriteString(
					selectedChatStyle.Render(line),
				)
			} else {
				chatList.WriteString(line)
			}
		}
	}

	left := chatBoxStyle.Render(
		chatList.String(),
	)

	rightText := "Select a chat\n\n" +
		"↑ / ↓   Navigate\n" +
		"Enter    Open chat\n" +
		"Ctrl+C   Quit"

	if m.loading {
		rightText = "Opening chat..."
	}

	right := messageBoxStyle.Render(
		rightText,
	)

	content := lipgloss.JoinHorizontal(
		lipgloss.Top,
		left,
		right,
	)

	b.WriteString(content)
	b.WriteString("\n\n")

	b.WriteString(
		hintStyle.Render(
			"↑↓ / j k Navigate   Enter Open   Ctrl+C Quit",
		),
	)

	b.WriteString("\n")

	if m.err != nil {
		b.WriteString(
			errorStyle.Render(
				"Error: " + m.err.Error(),
			),
		)

		b.WriteString("\n")
	}

	return b.String()
}

func (m tuiModel) chatView() string {
	var b strings.Builder

	b.WriteString("\n")

	header := fmt.Sprintf(
		"OSTRICH   /   %s",
		m.currentChat.Username,
	)

	b.WriteString(logoStyle.Render(header))
	b.WriteString("\n\n")

	chatWidth := m.width - 6

	if chatWidth < 50 {
		chatWidth = 50
	}

	contentWidth := chatWidth - 8

	if contentWidth < 20 {
		contentWidth = 20
	}

	var messageList strings.Builder

	if len(m.messages) == 0 {
		messageList.WriteString(
			hintStyle.Render("No messages yet."),
		)
	} else {
		start := 0
		maxVisible := 15

		if len(m.messages) > maxVisible {
			start = len(m.messages) - maxVisible
		}

		for _, message := range m.messages[start:] {
			name := message.SenderUsername

			if name == "" && m.user != nil {
				if message.SenderID == m.user.User.ID {
					name = m.user.User.Username
				} else {
					name = m.currentChat.Username
				}
			}

			text := fmt.Sprintf(
				"%s: %s",
				name,
				message.Content,
			)

			isOwn := false

			if m.user != nil {
				isOwn = message.SenderID == m.user.User.ID
			}

			style := lipgloss.NewStyle().
				Width(contentWidth).
				MarginBottom(1)

			if isOwn {
				style = style.
					Align(lipgloss.Right).
					Foreground(lipgloss.Color("10"))
			} else {
				style = style.
					Align(lipgloss.Left).
					Foreground(lipgloss.Color("205"))
			}

			messageList.WriteString(
				style.Render(text),
			)

			messageList.WriteString("\n")
		}
	}

	messageBox := lipgloss.NewStyle().
		Width(chatWidth).
		Height(18).
		Border(lipgloss.RoundedBorder()).
		Padding(1, 2).
		Render(messageList.String())

	b.WriteString(messageBox)
	b.WriteString("\n\n")

	inputWidth := chatWidth - 4

	if inputWidth < 20 {
		inputWidth = 20
	}

	input := m.messageInput
	input.Width = inputWidth

	inputBox := lipgloss.NewStyle().
		Width(chatWidth).
		Border(lipgloss.RoundedBorder()).
		Padding(0, 1).
		Render("> " + input.View())

	b.WriteString(inputBox)
	b.WriteString("\n\n")

	b.WriteString(
		hintStyle.Render(
			"Enter Send   Esc Back   Ctrl+C Quit",
		),
	)

	b.WriteString("\n")

	if m.err != nil {
		b.WriteString(
			errorStyle.Render(
				"Error: " + m.err.Error(),
			),
		)

		b.WriteString("\n")
	}

	return b.String()
}
