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
	"syscall"
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

// CancelRequest is the structure for a request to cancel a command.
type CancelRequest struct {
	ID string `json:"id"`
}

// StreamMessage is the structure for streaming output to the frontend.
type StreamMessage struct {
	Stream string `json:"stream"` // "stdout", "stderr", or "system"
	Data   string `json:"data"`
}

var (
	commandList          []CommandDefinition
	commandMap           map[string]CommandDefinition
	commandsLock         sync.RWMutex
	runningProcesses     = make(map[string]*exec.Cmd)
	runningProcessesLock sync.Mutex
	envVars              = make(map[string]string)
	envVarsLock          sync.RWMutex
)

const (
	commandsConfigPath = "../scripts-dump/commands.json"
	secretsFilePath    = "../scripts-dump/.env"
	pythonVenvPath     = "../scripts-dump/venv"
	coldStartScript    = "../scripts-dump/cold-start.sh"
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

	if err := loadEnvVars(); err != nil {
		log.Printf("WARN: Could not load environment variables: %v", err)
	}

	if err := loadCommands(); err != nil {
		log.Fatalf("FATAL: Could not load commands from %s: %v", commandsConfigPath, err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/commands", commandsHandler)
	mux.HandleFunc("/api/run", runHandler)
	mux.HandleFunc("/api/cancel", cancelHandler)
	mux.HandleFunc("/api/env", envHandler)
	mux.HandleFunc("/api/refresh", refreshHandler)

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

func refreshHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}
	log.Println("INFO: Received refresh request. Reloading commands.")
	if err := loadCommands(); err != nil {
		log.Printf("ERROR: Failed to reload commands: %v", err)
		http.Error(w, fmt.Sprintf("Failed to reload commands: %v", err), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "Command list refreshed successfully.")
}

func loadEnvVars() error {
	envVarsLock.Lock()
	defer envVarsLock.Unlock()

	envVars = make(map[string]string)

	file, err := os.Open(secretsFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("INFO: %s not found. Creating an empty file.", secretsFilePath)
			if _, createErr := os.Create(secretsFilePath); createErr != nil {
				return fmt.Errorf("failed to create secrets file: %w", createErr)
			}
			return nil
		}
		return fmt.Errorf("failed to open secrets file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	count := 0
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if parts := strings.SplitN(line, "=", 2); len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])

			if strings.HasPrefix(value, `"`) && strings.HasSuffix(value, `"`) {
				value = strings.Trim(value, `"`)
			}

			envVars[key] = value
			count++
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading secrets file: %w", err)
	}

	log.Printf("Successfully loaded %d environment variable(s).", count)
	return nil
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
			os.WriteFile(commandsConfigPath, data, 0644)
		} else {
			return fmt.Errorf("failed to read commands file: %w", err)
		}
	}

	if err := json.Unmarshal(data, &commandList); err != nil {
		return fmt.Errorf("failed to unmarshal commands JSON: %w", err)
	}

	commandMap = make(map[string]CommandDefinition)
	for _, cmd := range commandList {
		commandMap[cmd.ID] = cmd
	}
	log.Printf("Successfully loaded %d command(s).", len(commandList))
	return nil
}

func commandsHandler(w http.ResponseWriter, r *http.Request) {
	commandsLock.RLock()
	defer commandsLock.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(commandList); err != nil {
		log.Printf("ERROR: Failed to encode commands: %v", err)
		http.Error(w, "Failed to encode commands", http.StatusInternalServerError)
	}
}

func envHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		getEnvHandler(w, r)
	case http.MethodPost:
		updateEnvHandler(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func getEnvHandler(w http.ResponseWriter, _ *http.Request) {
	content, err := os.ReadFile(secretsFilePath)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("ERROR: Failed to read secrets file: %v", err)
			http.Error(w, "Failed to read secrets file", http.StatusInternalServerError)
			return
		}
	}
	w.Header().Set("Content-Type", "text/plain")
	w.Write(content)
}

func updateEnvHandler(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("ERROR: Failed to read request body for env update: %v", err)
		http.Error(w, "Failed to read request body", http.StatusInternalServerError)
		return
	}

	if err := os.WriteFile(secretsFilePath, body, 0644); err != nil {
		log.Printf("ERROR: Failed to write to secrets file: %v", err)
		http.Error(w, "Failed to write to secrets file", http.StatusInternalServerError)
		return
	}

	if err := loadEnvVars(); err != nil {
		log.Printf("ERROR: Failed to reload env vars after update: %v", err)
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "Environment variables updated successfully.")
}

func cancelHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CancelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	runningProcessesLock.Lock()
	cmd, ok := runningProcesses[req.ID]
	runningProcessesLock.Unlock()

	if !ok {
		log.Printf("WARN: Cancel request for command '%s', but it was not found running.", req.ID)
		http.Error(w, "Command not found or already stopped", http.StatusNotFound)
		return
	}

	if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGINT); err != nil {
		log.Printf("ERROR: Failed to send SIGINT to process group for command '%s' (PID: %d): %v", req.ID, cmd.Process.Pid, err)
		if errProc := cmd.Process.Signal(os.Interrupt); errProc != nil {
			log.Printf("ERROR: Fallback signal to process for command '%s' also failed: %v", req.ID, errProc)
			http.Error(w, "Failed to interrupt command", http.StatusInternalServerError)
			return
		}
	}

	log.Printf("INFO: Sent interrupt signal to command '%s' (PID: %d)", req.ID, cmd.Process.Pid)
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "Interrupt signal sent.")
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
	cmdDef, ok := commandMap[req.ID]
	commandsLock.RUnlock()
	if !ok {
		http.Error(w, "Command not found", http.StatusNotFound)
		return
	}

	runningProcessesLock.Lock()
	if _, exists := runningProcesses[req.ID]; exists {
		runningProcessesLock.Unlock()
		http.Error(w, "Task is already running", http.StatusConflict)
		return
	}
	runningProcessesLock.Unlock()

	var executable string
	var args []string

	switch cmdDef.ScriptType {
	case "bash":
		executable = "/bin/bash"
		args = append(args, cmdDef.ScriptPath)
	case "python":
		executable = filepath.Join(pythonVenvPath, "bin", "python")
		args = append(args, cmdDef.ScriptPath)
	default:
		http.Error(w, fmt.Sprintf("Unsupported script type: %s", cmdDef.ScriptType), http.StatusBadRequest)
		return
	}

	for _, p := range cmdDef.Parameters {
		if val, ok := req.Params[p.Name]; ok {
			envVarsLock.RLock()
			if after, ok0 := strings.CutPrefix(val, "$"); ok0 {
				varName := after
				if secretVal, found := envVars[varName]; found {
					val = secretVal
				}
			}
			envVarsLock.RUnlock()

			if p.Type == "checkbox" {
				if val == "true" {
					args = append(args, p.Name)
				}
			} else if val != "" {
				args = append(args, p.Name)
				if p.Type == "list" {
					var vals []string
					for t := range strings.SplitSeq(val, ",") {
						vals = append(vals, strings.TrimSpace(t))
					}
					args = append(args, vals...)
				} else {
					args = append(args, val)
				}
			}
		}
	}

	log.Printf("Executing command '%s': %s %v", cmdDef.ID, executable, args)

	cmd := exec.Command(executable, args...)
	cmd.Env = os.Environ()
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	runningProcessesLock.Lock()
	runningProcesses[req.ID] = cmd
	runningProcessesLock.Unlock()
	defer func() {
		runningProcessesLock.Lock()
		delete(runningProcesses, req.ID)
		runningProcessesLock.Unlock()
		log.Printf("Cleaned up process map for command '%s'", req.ID)
	}()

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

	sendMessage := func(msg StreamMessage) {
		if err := json.NewEncoder(w).Encode(msg); err != nil {
			log.Printf("ERROR: Failed to write stream message: %v", err)
		}
		if flusher != nil {
			flusher.Flush()
		}
	}

	sendMessage(StreamMessage{Stream: "system", Data: fmt.Sprintf("Starting command: %s (PID: %d)", cmdDef.Name, cmd.Process.Pid)})

	for {
		select {
		case msg, ok := <-outputChan:
			if !ok {
				err = cmd.Wait()
				if err != nil {
					if exitErr, ok := err.(*exec.ExitError); ok {
						if status, ok := exitErr.Sys().(syscall.WaitStatus); ok && status.Signaled() && status.Signal() == syscall.SIGINT {
							sendMessage(StreamMessage{Stream: "system", Data: "CANCELLED: Command was cancelled by user."})
							log.Printf("INFO: Command '%s' was cancelled by user.", cmdDef.ID)
						} else {
							sendMessage(StreamMessage{Stream: "system", Data: fmt.Sprintf("FAIL: Command finished with error: %v", err)})
							log.Printf("ERROR: Command '%s' failed: %v", cmdDef.ID, err)
						}
					} else {
						sendMessage(StreamMessage{Stream: "system", Data: fmt.Sprintf("FAIL: Command finished with non-exit error: %v", err)})
						log.Printf("ERROR: Command '%s' failed with non-exit error: %v", cmdDef.ID, err)
					}
				} else {
					sendMessage(StreamMessage{Stream: "system", Data: "SUCCESS: Command completed successfully."})
					log.Printf("INFO: Command '%s' completed successfully.", cmdDef.ID)
				}
				return
			}
			sendMessage(msg)
		case <-r.Context().Done():
			log.Printf("WARN: Client disconnected for command '%s'. Sending interrupt.", cmdDef.ID)
			if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGINT); err != nil {
				log.Printf("ERROR: Failed to kill process for disconnected client: %v", err)
			}
			_ = cmd.Wait()
			return
		}
	}
}

func streamPipe(pipe io.Reader, streamType string, c chan<- StreamMessage, wg *sync.WaitGroup) {
	defer wg.Done()
	scanner := bufio.NewScanner(pipe)
	for scanner.Scan() {
		c <- StreamMessage{Stream: streamType, Data: scanner.Text()}
	}
}
