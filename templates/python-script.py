#!/usr/bin/env python3
# python3 script.py --input-file "/path/to/file.txt" --mode "process" --verbose

import argparse
import sys
import time
import os

# ==============================================================================

def run_task(args):
    is_verbose = str(args.verbose).lower() == 'true'

    loginfo("Script execution started.")

    if is_verbose:
        logerror("DEBUG: Verbose mode enabled.")

    loginfo("Parameters received:")
    loginfo(f"  - Input File: {args.input_file}")
    loginfo(f"  - Mode: {args.mode}")
    loginfo(f"  - Verbose: {is_verbose}")
    print() # Add a newline for readability

    # Simulate some work...
    loginfo("Starting task...")
    time.sleep(1)
    loginfo("Processing step 1 of 2...")
    time.sleep(1)

    # Example of validation and exiting with an error
    if not os.path.exists(args.input_file):
        logerror(f"ERROR: The file '{args.input_file}' does not exist.")
        sys.exit(1) # Exit with a non-zero status code for failure.

    loginfo(f"Processing step 2 of 2... (reading from {args.input_file})")
    time.sleep(1)
    loginfo("Task finished.")

# ==============================================================================

def loginfo(*args, **kwargs):
    print(*args, file=sys.stdout, **kwargs)

def logerror(*args, **kwargs):
    print(*args, file=sys.stderr, **kwargs)

def setup_arg_parser():
    parser = argparse.ArgumentParser(description="A sample Python script for Kaname.")
    parser.add_argument(
        "--input-file",
        dest="input_file",
        type=str,
        required=True,
        help="Path to the input file to process."
    )
    parser.add_argument(
        "--mode",
        dest="mode",
        type=str,
        choices=["preview", "process"],
        default="preview",
        help="The processing mode."
    )
    parser.add_argument(
        "--verbose",
        dest="verbose",
        action="store_true", # makes it a flag
        help="Enable verbose logging to STDERR."
    )    
    return parser

# COMMANDS.JSON Entry
# {
#   "id": "python-task",
#   "name": "A Python Task",
#   "description": "A sample task",
#   "script_path": "/app/scripts/script.py",
#   "script_type": "python",
#   "icon": "fa-python",
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

if __name__ == "__main__":
    arg_parser = setup_arg_parser()
    parsed_args = arg_parser.parse_args()
    run_task(parsed_args)
