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
// VenvPath is removed as per requirements; a hardcoded path will be used.
type CommandDefinition struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	ScriptPath  string         `json:"script_path"`
	ScriptType  string         `json:"script_type"`
	Parameters  []CommandParam `json:"parameters"`
	Icon        string         `json:"icon"` // Added for frontend styling
}

// ExecutionRequest is the structure for a request to run a command.
type ExecutionRequest struct {
	ID     string            `json:"id"`
	Params map[string]string `json:"params"`
}

var (
	// In-memory store for command definitions, loaded from commands.json.
	commands     map[string]CommandDefinition
	commandsLock sync.RWMutex
)

const (
	// Hardcoded paths as per requirements.
	commandsConfigPath = "/app/scripts/commands.json"
	pythonVenvPath     = "/app/scripts/venv"
	coldStartScript    = "/app/scripts/cold-start.sh"
)

func main() {
	log.Println("Starting Kaname...")

	// 1. Execute the cold-start script before doing anything else.
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

	// 2. Load command definitions from the JSON file.
	if err := loadCommands(); err != nil {
		log.Fatalf("FATAL: Could not load commands from %s: %v", commandsConfigPath, err)
	}

	// 3. Set up HTTP server and handlers.
	mux := http.NewServeMux()

	mux.HandleFunc("/api/commands", commandsHandler)
	mux.HandleFunc("/api/run", runHandler)

	// Serve the embedded frontend assets.
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

// loadCommands reads and parses the commands.json file into the in-memory store.
func loadCommands() error {
	commandsLock.Lock()
	defer commandsLock.Unlock()

	log.Printf("Loading command definitions from %s", commandsConfigPath)
	data, err := os.ReadFile(commandsConfigPath)
	if err != nil {
		// Create a dummy file if it doesn't exist to allow the app to start.
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
			// We don't write it back, just use the dummy data for this session.
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

// commandsHandler serves the list of loaded commands as JSON.
func commandsHandler(w http.ResponseWriter, r *http.Request) {
	commandsLock.RLock()
	defer commandsLock.RUnlock()

	// Create a slice from the map to ensure a consistent JSON array.
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

// runHandler executes a script and streams its stdout/stderr back to the client.
func runHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	// 1. Decode the request body.
	var req ExecutionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// 2. Find the command definition.
	commandsLock.RLock()
	cmdDef, ok := commands[req.ID]
	commandsLock.RUnlock()
	if !ok {
		http.Error(w, "Command not found", http.StatusNotFound)
		return
	}

	// 3. Construct the command arguments.
	var executable string
	args := []string{cmdDef.ScriptPath}

	// TODO: process params; generally, this does it.
	// but need to handle multiple input array, and true/false (frontend sends bool as string)
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

	// 4. Set up the command execution.
	cmd := exec.Command(executable, args...)
	cmd.Env = os.Environ()

	// 5. Pipe stdout and stderr.
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

	// 6. Start the command.
	if err := cmd.Start(); err != nil {
		http.Error(w, "Failed to start command", http.StatusInternalServerError)
		return
	}

	// 7. Stream the output back to the client.
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff") // Important for security
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Use a channel to merge stdout and stderr to prevent garbled output.
	outputChan := make(chan string)
	var wg sync.WaitGroup
	wg.Add(2)

	// Goroutine to read and stream stdout.
	go streamPipe(stdout, "STDOUT", outputChan, &wg)
	// Goroutine to read and stream stderr.
	go streamPipe(stderr, "STDERR", outputChan, &wg)

	// Goroutine to close the channel once both pipes are done.
	go func() {
		wg.Wait()
		close(outputChan)
	}()

	// Write the initial execution message.
	fmt.Fprintf(w, "EXEC: Starting command: %s\n", cmdDef.Name)
	flusher.Flush()

	// Read from the merged channel and write to the HTTP response.
	for line := range outputChan {
		fmt.Fprintln(w, line)
		flusher.Flush()
	}

	// 8. Wait for the command to finish and send final status.
	err = cmd.Wait()
	if err != nil {
		fmt.Fprintf(w, "FAIL: Command finished with error: %v\n", err)
		log.Printf("ERROR: Command '%s' failed: %v", cmdDef.ID, err)
	} else {
		fmt.Fprintln(w, "SUCCESS: Command completed successfully.")
		log.Printf("INFO: Command '%s' completed successfully.", cmdDef.ID)
	}
	flusher.Flush()
}

// streamPipe reads from an io.Reader (a pipe), prefixes each line,
// and sends it to a channel.
func streamPipe(pipe io.Reader, prefix string, c chan<- string, wg *sync.WaitGroup) {
	defer wg.Done()
	scanner := bufio.NewScanner(pipe)
	for scanner.Scan() {
		c <- fmt.Sprintf("%s: %s", prefix, scanner.Text())
	}
}
