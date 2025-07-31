package main

import (
	"bufio"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

//go:embed frontend
var embeddedFrontend embed.FS

// CommandParam defines a single parameter for a command.
type CommandParam struct {
	Name     string   `json:"name"`
	Label    string   `json:"label"`
	Type     string   `json:"type"`
	Required bool     `json:"required"`
	Options  []string `json:"options,omitempty"`
	Default  any      `json:"default,omitempty"`
}

// CommandDefinition defines a script that can be executed.
type CommandDefinition struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	ScriptPath  string         `json:"script_path"`
	ScriptType  string         `json:"script_type"`
	Parameters  []CommandParam `json:"parameters"`
	Icon        string         `json:"icon"`
}

// ExecutionRequest is the structure for a request to run a command.
type ExecutionRequest struct {
	ID     string            `json:"id"`
	Params map[string]string `json:"params"`
}

// StreamMessage is the structure for streaming output to the frontend.
type StreamMessage struct {
	Stream string `json:"stream"` // "stdout", "stderr", or "system"
	Data   string `json:"data"`
}

var (
	commands     map[string]CommandDefinition
	commandsLock sync.RWMutex
)

const (
	commandsConfigPath = "/app/scripts/commands.json"
	pythonVenvPath     = "/app/scripts/venv"
	coldStartScript    = "/app/scripts/cold-start.sh"
)

func main() {
	log.Println("Starting Kaname...")

	if _, err := os.Stat(coldStartScript); err == nil {
		log.Printf("Executing cold-start script at %s...", coldStartScript)
		cmd := exec.Command("/bin/bash", coldStartScript)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			log.Fatalf("FATAL: Cold-start script failed: %v. Exiting.", err)
		}
		log.Println("Cold-start script completed successfully.")
	}

	if err := loadCommands(); err != nil {
		log.Fatalf("FATAL: Could not load commands from %s: %v", commandsConfigPath, err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/commands", commandsHandler)
	mux.HandleFunc("/api/run", runHandler)

	frontendFS, err := fs.Sub(embeddedFrontend, "frontend")
	if err != nil {
		log.Fatalf("FATAL: Failed to create frontend subtree: %v", err)
	}
	mux.Handle("/", http.FileServer(http.FS(frontendFS)))

	log.Println("Server starting on :8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatalf("FATAL: Server failed to start: %v", err)
	}
}

func loadCommands() error {
	commandsLock.Lock()
	defer commandsLock.Unlock()

	log.Printf("Loading command definitions from %s", commandsConfigPath)
	data, err := os.ReadFile(commandsConfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("WARN: %s not found. Creating a dummy file. Please configure it.", commandsConfigPath)
			dummyCommands := []CommandDefinition{
				{
					ID:          "placeholder",
					Name:        "Placeholder Command",
					Description: "This is a placeholder. Please configure your commands.json.",
					ScriptPath:  "/bin/echo",
					ScriptType:  "bash",
					Icon:        "fa-question-circle",
				},
			}
			data, _ = json.MarshalIndent(dummyCommands, "", "  ")
		} else {
			return fmt.Errorf("failed to read commands file: %w", err)
		}
	}

	var loadedCommands []CommandDefinition
	if err := json.Unmarshal(data, &loadedCommands); err != nil {
		return fmt.Errorf("failed to unmarshal commands JSON: %w", err)
	}

	commands = make(map[string]CommandDefinition)
	for _, cmd := range loadedCommands {
		commands[cmd.ID] = cmd
	}
	log.Printf("Successfully loaded %d command(s).", len(commands))
	return nil
}

func commandsHandler(w http.ResponseWriter, r *http.Request) {
	commandsLock.RLock()
	defer commandsLock.RUnlock()

	var cmdList []CommandDefinition
	for _, cmd := range commands {
		cmdList = append(cmdList, cmd)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(cmdList); err != nil {
		log.Printf("ERROR: Failed to encode commands: %v", err)
		http.Error(w, "Failed to encode commands", http.StatusInternalServerError)
	}
}

func runHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ExecutionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	commandsLock.RLock()
	cmdDef, ok := commands[req.ID]
	commandsLock.RUnlock()
	if !ok {
		http.Error(w, "Command not found", http.StatusNotFound)
		return
	}

	var executable string
	args := []string{cmdDef.ScriptPath}
	for _, p := range cmdDef.Parameters {
		if val, ok := req.Params[p.Name]; ok {
			if p.Type == "list" {
				args = append(args, p.Name)
				values := strings.Split(val, ",")
				for i, v := range values {
					values[i] = strings.TrimSpace(v)
				}
				args = append(args, values...)
			} else {
				args = append(args, p.Name, val)
			}
		}
	}

	switch cmdDef.ScriptType {
	case "bash":
		executable = "/bin/bash"
	case "python":
		executable = filepath.Join(pythonVenvPath, "bin", "python")
	default:
		http.Error(w, fmt.Sprintf("Unsupported script type: %s", cmdDef.ScriptType), http.StatusBadRequest)
		return
	}

	log.Printf("Executing command '%s': %s %v", cmdDef.ID, executable, args)

	cmd := exec.Command(executable, args...)
	cmd.Env = os.Environ()

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		http.Error(w, "Failed to create stdout pipe", http.StatusInternalServerError)
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		http.Error(w, "Failed to create stderr pipe", http.StatusInternalServerError)
		return
	}

	if err := cmd.Start(); err != nil {
		http.Error(w, "Failed to start command", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/x-json-stream")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	outputChan := make(chan StreamMessage)
	var wg sync.WaitGroup
	wg.Add(2)

	go streamPipe(stdout, "stdout", outputChan, &wg)
	go streamPipe(stderr, "stderr", outputChan, &wg)

	go func() {
		wg.Wait()
		close(outputChan)
	}()

	// Helper to send a JSON message and flush.
	sendMessage := func(msg StreamMessage) {
		if err := json.NewEncoder(w).Encode(msg); err != nil {
			log.Printf("ERROR: Failed to write stream message: %v", err)
		}
		flusher.Flush()
	}

	sendMessage(StreamMessage{Stream: "system", Data: fmt.Sprintf("Starting command: %s", cmdDef.Name)})

	for msg := range outputChan {
		sendMessage(msg)
	}

	err = cmd.Wait()
	if err != nil {
		sendMessage(StreamMessage{Stream: "system", Data: fmt.Sprintf("FAIL: Command finished with error: %v", err)})
		log.Printf("ERROR: Command '%s' failed: %v", cmdDef.ID, err)
	} else {
		sendMessage(StreamMessage{Stream: "system", Data: "SUCCESS: Command completed successfully."})
		log.Printf("INFO: Command '%s' completed successfully.", cmdDef.ID)
	}
}

// streamPipe reads from an io.Reader, wraps each line in a StreamMessage,
// and sends it to a channel.
func streamPipe(pipe io.Reader, streamType string, c chan<- StreamMessage, wg *sync.WaitGroup) {
	defer wg.Done()
	scanner := bufio.NewScanner(pipe)
	for scanner.Scan() {
		c <- StreamMessage{Stream: streamType, Data: scanner.Text()}
	}
}
