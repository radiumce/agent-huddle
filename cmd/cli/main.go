package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

var (
	serverURL string
)

type Config struct {
	ServerURL string `json:"server_url"`
}

func getConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".agent-huddle", "config.json")
}

func loadConfig() string {
	path := getConfigPath()
	if path == "" {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return ""
	}
	return cfg.ServerURL
}

func saveConfig(url string) {
	path := getConfigPath()
	if path == "" {
		return
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		fmt.Printf("Warning: failed to create config directory: %v\n", err)
		return
	}
	cfg := Config{ServerURL: url}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		fmt.Printf("Warning: failed to encode config: %v\n", err)
		return
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		fmt.Printf("Warning: failed to write config file: %v\n", err)
	}
}

func main() {
	defaultURL := "http://localhost:8881"
	if saved := loadConfig(); saved != "" {
		defaultURL = saved
	}

	// Parse global flags before subcommands
	flag.StringVar(&serverURL, "server", defaultURL, "URL of the Agent Huddle HTTP API")
	flag.Usage = printUsage
	flag.Parse()

	// Check if --server was explicitly provided
	serverFlagProvided := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "server" {
			serverFlagProvided = true
		}
	})

	if len(flag.Args()) < 1 {
		// If no subcommand is provided, or if --server is explicitly passed without a command
		saveConfig(serverURL)
		fmt.Printf("✅ Agent Huddle CLI configuration saved.\n")
		fmt.Printf("   Current Server URL: %s\n", serverURL)

		// Quick health check
		client := http.Client{Timeout: 2 * time.Second}
		resp, err := client.Get(serverURL + "/api/rooms/list")
		if err == nil && resp.StatusCode == http.StatusOK {
			fmt.Printf("   Server Status: ✅ Running\n")
			resp.Body.Close()
		} else {
			fmt.Printf("   Server Status: ❌ Unreachable (Could not connect or bad response)\n")
			if err == nil {
				resp.Body.Close()
			}
		}

		os.Exit(0)
	}

	// If there ARE subcommands, explicitly fail if --server was passed
	if serverFlagProvided {
		fmt.Println("Error: the '--server' flag can only be used alone to configure the CLI (e.g., 'huddle-cli --server <url>').")
		fmt.Println("It cannot be used with subcommands.")
		os.Exit(1)
	}

	cmd := flag.Arg(0)
	args := flag.Args()[1:]

	switch cmd {
	case "list":
		handleList(args)
	case "create":
		handleCreate(args)
	case "post":
		handlePost(args)
	case "wait":
		handleWait(args)
	case "context":
		handleContext(args)
	case "close":
		handleClose(args)
	case "leave":
		handleLeave(args)
	default:
		fmt.Printf("Unknown command: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage: huddle-cli <command> [command options]")
	fmt.Println("\nConfiguration Command:")
	fmt.Println("  huddle-cli --server <url>   Configure the local Agent Huddle HTTP API server URL")
	fmt.Println("\nCommands:")
	fmt.Println("  list        List all active meeting rooms")
	fmt.Println("  create      Create a room, optionally post init message, and wait for reply")
	fmt.Println("  post        Post a message and wait for new messages (use --force to skip conflict checking)")
	fmt.Println("  wait        Wait for new messages in the room")
	fmt.Println("  context     Get message history from a specific ID")
	fmt.Println("  close       Close a meeting room")
	fmt.Println("  leave       Leave a meeting room")
	fmt.Println("\nRun 'huddle-cli <command> -h' for more options on a specific command.")
}

func doRequest(endpoint string, payload interface{}) {
	var bodyReader io.Reader
	var data []byte
	var err error
	if payload != nil {
		data, err = json.Marshal(payload)
		if err != nil {
			fmt.Println("Error encoding request mapping:", err)
			os.Exit(1)
		}
		bodyReader = bytes.NewReader(data)
	}

	url := serverURL + endpoint
	req, err := http.NewRequest("POST", url, bodyReader)
	if err != nil {
		fmt.Println("Error creating request:", err)
		os.Exit(1)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Error sending request to %s: %v\n", url, err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading response:", err)
		os.Exit(1)
	}

	printReadableResponse(body)

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("\n[Server responded with status: %d]\n", resp.StatusCode)
		os.Exit(1)
	}
}

func handleList(args []string) {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	fs.Parse(args)

	doRequest("/api/rooms/list", map[string]string{})
}

func handleCreate(args []string) {
	fs := flag.NewFlagSet("create", flag.ExitOnError)
	roomID := fs.String("room-id", "", "(Required) Room ID")
	name := fs.String("name", "", "Room name")
	host := fs.String("host", "", "(Required) Host name")
	initMsg := fs.String("init-message", "", "Initial message to post")
	timeout := fs.Int("timeout", 300, "Timeout in seconds (0 means no wait for new messages)")
	fs.Parse(args)

	if *roomID == "" || *host == "" {
		fmt.Println("Error: --room-id and --host are required")
		fs.PrintDefaults()
		os.Exit(1)
	}

	payload := map[string]interface{}{
		"room_id":      *roomID,
		"name":         *name,
		"host":         *host,
		"init_message": *initMsg,
		"timeout_sec":  *timeout,
	}

	if *timeout > 0 {
		fmt.Printf("[Waiting for new messages for %d seconds...]\n", *timeout)
	}
	doRequest("/api/rooms/create_and_wait", payload)
}

func handlePost(args []string) {
	fs := flag.NewFlagSet("post", flag.ExitOnError)
	roomID := fs.String("room-id", "", "(Required) Room ID")
	sender := fs.String("sender", "", "(Required) Sender name")
	content := fs.String("content", "", "(Required) Message content")
	recipient := fs.String("recipient", "", "Recipient name")
	lastID := fs.Int64("last-id", 0, "Last message ID seen")
	timeout := fs.Int("timeout", 300, "Timeout in seconds (0 means no wait for new messages)")
	force := fs.Bool("force", false, "Force post a message without conflict checking")
	fs.Parse(args)

	if *roomID == "" || *sender == "" || *content == "" {
		fmt.Println("Error: --room-id, --sender, and --content are required")
		fs.PrintDefaults()
		os.Exit(1)
	}

	payload := map[string]interface{}{
		"room_id":     *roomID,
		"sender":      *sender,
		"content":     *content,
		"recipient":   *recipient,
		"last_id":     *lastID,
		"timeout_sec": *timeout,
		"force":       *force,
	}

	endpoint := "/api/rooms/post_message_and_wait"
	if *force {
		endpoint = "/api/rooms/force_post_message_and_wait"
	}

	if *timeout > 0 {
		fmt.Printf("[Waiting for new messages for %d seconds...]\n", *timeout)
	}
	doRequest(endpoint, payload)
}

func handleWait(args []string) {
	fs := flag.NewFlagSet("wait", flag.ExitOnError)
	roomID := fs.String("room-id", "", "(Required) Room ID")
	member := fs.String("member", "", "(Required) Member name")
	lastID := fs.Int64("last-id", 0, "Last message ID received")
	timeout := fs.Int("timeout", 300, "Timeout in seconds (0 means no wait for new messages)")
	fs.Parse(args)

	if *roomID == "" || *member == "" {
		fmt.Println("Error: --room-id and --member are required")
		fs.PrintDefaults()
		os.Exit(1)
	}

	payload := map[string]interface{}{
		"room_id":     *roomID,
		"member_name": *member,
		"last_id":     *lastID,
		"timeout_sec": *timeout,
	}

	if *timeout > 0 {
		fmt.Printf("[Waiting for new messages for %d seconds...]\n", *timeout)
	}
	doRequest("/api/rooms/wait_for_message", payload)
}

func handleContext(args []string) {
	fs := flag.NewFlagSet("context", flag.ExitOnError)
	roomID := fs.String("room-id", "", "(Required) Room ID")
	member := fs.String("member", "", "(Required) Member name")
	lastID := fs.Int64("last-id", 0, "Start fetching after this message ID (default 0)")
	fs.Parse(args)

	if *roomID == "" || *member == "" {
		fmt.Println("Error: --room-id and --member are required")
		fs.PrintDefaults()
		os.Exit(1)
	}

	payload := map[string]interface{}{
		"room_id":     *roomID,
		"member_name": *member,
		"last_id":     *lastID,
	}
	doRequest("/api/rooms/context", payload)
}

func handleClose(args []string) {
	fs := flag.NewFlagSet("close", flag.ExitOnError)
	roomID := fs.String("room-id", "", "(Required) Room ID")
	fs.Parse(args)

	if *roomID == "" {
		fmt.Println("Error: --room-id is required")
		fs.PrintDefaults()
		os.Exit(1)
	}

	doRequest("/api/rooms/close", map[string]string{"room_id": *roomID})
}

func handleLeave(args []string) {
	fs := flag.NewFlagSet("leave", flag.ExitOnError)
	roomID := fs.String("room-id", "", "(Required) Room ID")
	member := fs.String("member", "", "(Required) Member name")
	fs.Parse(args)

	if *roomID == "" || *member == "" {
		fmt.Println("Error: --room-id and --member are required")
		fs.PrintDefaults()
		os.Exit(1)
	}

	doRequest("/api/rooms/leave", map[string]string{"room_id": *roomID, "member_name": *member})
}

func printReadableResponse(body []byte) {
	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		// Fallback to raw output if not valid JSON
		fmt.Println(string(body))
		return
	}

	if result, ok := payload["result"]; ok {
		fmt.Printf("✔ Result: %v\n", result)
	}
	if errorMsg, ok := payload["error"]; ok {
		fmt.Printf("✖ Error: %v\n", errorMsg)
	}
	if roomID, ok := payload["room_id"]; ok {
		fmt.Printf("Room ID: %v\n", roomID)
	}
	if lastID, ok := payload["last_msg_id"]; ok {
		if fmt.Sprintf("%v", lastID) != "0" {
			fmt.Printf("Last Msg ID: %v\n", lastID)
		}
	}

	printMessages(payload, "messages", "New Messages")
	printMessages(payload, "pre_existing_msgs", "Pre-existing Messages")

	if rooms, ok := payload["rooms"].([]interface{}); ok {
		if len(rooms) == 0 {
			fmt.Println("No active rooms found.")
		} else {
			fmt.Printf("\n[Active Rooms (%d)]\n", len(rooms))
			for _, rInter := range rooms {
				if r, isMap := rInter.(map[string]interface{}); isMap {
					id := r["ID"]
					name := r["Name"]
					if name == "" {
						name = "<None>"
					}

					hostName := "<Unknown>"
					if hostMap, ok := r["Host"].(map[string]interface{}); ok {
						if hName, ok2 := hostMap["Name"].(string); ok2 {
							hostName = hName
						}
					}

					memCount := 0
					if mems, mok := r["Members"].(map[string]interface{}); mok {
						memCount = len(mems)
					}

					fmt.Printf("- %v (Name: %v) | Host: %v | Members Active: %d\n", id, name, hostName, memCount)
				}
			}
		}
	}
}

func printMessages(payload map[string]interface{}, key, title string) {
	if msgs, ok := payload[key].([]interface{}); ok && len(msgs) > 0 {
		fmt.Printf("\n[%s]\n", title)
		for _, mInter := range msgs {
			if m, isMap := mInter.(map[string]interface{}); isMap {
				id := m["id"]
				sender := m["sender"]
				content := m["content"]
				fmt.Printf("  [%v] %v: %v\n", id, sender, content)
			}
		}
	}
}
