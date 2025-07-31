<div align="center">
  <img src=".github/assets/logo.png" alt="Kaname Logo" width="200">
  <h1>Kaname</h1>

  <a href="https://github.com/tanq16/kaname/actions/workflows/release.yml"><img alt="Build Workflow" src="https://github.com/tanq16/kaname/actions/workflows/release.yml/badge.svg"></a>&nbsp;<a href="https://github.com/Tanq16/kaname/releases"><img alt="GitHub Release" src="https://img.shields.io/github/v/release/tanq16/kaname"></a>&nbsp;<a href="https://hub.docker.com/r/tanq16/kaname"><img alt="Docker Pulls" src="https://img.shields.io/docker/pulls/tanq16/kaname"></a><br><br>
</div>

A simple, self-hosted, and elegant web-based UI for running your predefined scripts. Kaname is designed for those who need a straightforward way to execute tasks on a server without exposing SSH or dealing with complex command-line interfaces.

The goal of the application is to provide a clean and modern interface for triggering scripts like shell or Python. You define your tasks and their parameters in a simple JSON file, and Kaname presents them as interactive cards in a beautiful UI. It's perfect for homelabs, small teams, or anyone looking to simplify their operational workflows.

## Features

- Beautiful Catppuccin Mocha themed application for a modern task runner interface.
- Dynamically loads tasks from a `commands.json` file, making configuration simple and centralized.
- Supports various script types, including `bash` and `python`, with easy extension for more.
- Allows for script parameterization with UI elements like text fields, dropdowns, checkboxes, and date pickers.
- Real-time streaming of both `stdout` and `stderr` to separate, organized tabs in the UI.
- Fully self-hosted with local assets, running as a single, self-contained binary or container.
- Efficient and lightweight, with a minimal binary and container size.

## Screenshots

| Desktop View | Mobile View |
| --- | --- |
| | |
| | |

## Usage

### Docker (Recommended)

The simplest way to run Kaname is with Docker, which ensures the correct directory structure for scripts.

```bash
# Create a directory to hold your scripts and configuration
mkdir -p $HOME/kaname/scripts

# Place your commands.json and scripts inside $HOME/kaname/scripts
````

```bash
docker run --rm -d --name kaname \
  -p 8080:8080 \
  -v $HOME/kaname/scripts:/app/scripts \
  tanq16/kaname:main
```

The application will be available at `http://localhost:8080` (or your server IP). You can also use the following compose file:

```yaml
services:
  kaname:
    image: tanq16/kaname:main
    container_name: kaname
    volumes:
      - /path/to/your/scripts:/app/scripts # Change as needed
    ports:
      - 8080:8080
    restart: unless-stopped
```

### Binary

To use the binary, download the latest version from the project releases. Note that Kaname expects the scripts and `commands.json` to be in a specific path (`/app/scripts`), so running it outside of Docker requires matching this structure.

### Local development

With `Go 1.23+` installed, run the following to download the binary to your GOBIN:

```bash
go install [github.com/tanq16/kaname@latest](https://github.com/tanq16/kaname@latest)
```

Or, you can build from source like so:

```bash
git clone [https://github.com/tanq16/kaname.git](https://github.com/tanq16/kaname.git) && \
cd kaname && \
go build .
```

## Configuration

Kaname is configured via a single `commands.json` file placed in your scripts directory. This file contains an array of command objects.

Here is an example of a command definition:

```json
[
  {
    "id": "python-task-example",
    "name": "Sample Python Task",
    "description": "An example task that runs a Python script with parameters.",
    "script_path": "/app/scripts/your-script.py",
    "script_type": "python",
    "icon": "fa-brands fa-python",
    "parameters": [
      {
        "name": "--input-file",
        "label": "Input File Path",
        "type": "text",
        "required": true,
        "default": "/app/data/default.txt"
      },
      {
        "name": "--mode",
        "label": "Processing Mode",
        "type": "select",
        "required": false,
        "default": "preview",
        "options": ["preview", "process"]
      },
      {
        "name": "--verbose",
        "label": "Enable Verbose Logging",
        "type": "checkbox",
        "required": false,
        "default": false
      }
    ]
  }
]
```

> [!NOTE]
> You can use any Font Awesome icon class for the `icon` field. The `name` of a parameter should match the command-line flag your script expects (e.g., `--input-file`).
