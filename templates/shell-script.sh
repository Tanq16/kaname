#!/bin/bash
# ./script.sh --input-file "/path/to/file.txt" --mode "process" --verbose "true"

# ==============================================================================

run_task() {
    loginfo "Script execution started."

    if [[ -z "$INPUT_FILE" ]]; then
        logerror "ERROR: Input file was not provided."
        exit 1
    fi

    loginfo "Parameters received:"
    loginfo "  - Input File: $INPUT_FILE"
    loginfo "  - Mode: $MODE"
    loginfo "  - Verbose: $VERBOSE"
    echo "" # Add a newline for readability

    if [ "$VERBOSE" = true ]; then
        logerror "DEBUG: Verbose mode enabled."
    fi

    loginfo "Starting task..."
    sleep 1
    loginfo "Processing step 1 of 2..."
    sleep 1

    if [[ ! -f "$INPUT_FILE" ]]; then
        logerror "ERROR: The file '$INPUT_FILE' does not exist."
        exit 1
    fi

    loginfo "Processing step 2 of 2... (reading from $INPUT_FILE)"
    sleep 1
    loginfo "Task finished."
}

# ==============================================================================

loginfo() {
    echo "$@"
}

logerror() {
    echo "$@" >&2
}

parse_arguments() {
    # Defaults
    INPUT_FILE=""
    MODE="preview"
    VERBOSE=false

    while [[ $# -gt 0 ]]; do
        key="$1"
        case $key in
            --input-file)
            INPUT_FILE="$2"
            shift 2
            ;;
            --mode)
            MODE="$2"
            shift 2
            ;;
            --verbose)
            # Kaname sends 'true' for a checked box.
            if [[ "$2" == "true" ]]; then
                VERBOSE=true
            fi
            shift 2
            ;;
            *)
            logerror "Unknown parameter passed: $1"
            shift
            ;;
        esac
    done
}

# COMMANDS.JSON Entry
# {
#   "id": "shell-task",
#   "name": "A Shell Task",
#   "description": "A sample task",
#   "script_path": "/app/scripts/script.sh",
#   "script_type": "bash",
#   "icon": "fa-terminal",
#   "parameters": [
#     {
#       "name": "input-file",
#       "label": "Input File Path",
#       "type": "text",
#       "required": true,
#       "default": "/app/data/default.txt"
#     },
#     {
#       "name": "mode",
#       "label": "Processing Mode",
#       "type": "select",
#       "required": false,
#       "default": "preview",
#       "options": ["preview", "process"]
#     },
#     {
#       "name": "verbose",
#       "label": "Enable Verbose Logging",
#       "type": "checkbox",
#       "required": false,
#       "default": false
#     }
#   ]
# }

parse_arguments "$@"
run_task
