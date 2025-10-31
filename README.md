
# BnBt C2 Framework

A simple, proof-of-concept Command and Control (C2) framework written in Go. This tool uses WebSockets for lightweight, real-time communication between a central client and one or more payloads.



> **Disclaimer**
>
> This project was created for educational and research purposes only. It is intended to demonstrate how C2 frameworks operate. Unauthorized access to any computer system is illegal. The author is not responsible for any misuse or damage caused by this program. Use responsibly and ethically.

## Features

-   **Client-Server Architecture**: A central client to manage and command multiple payloads.
-   **WebSocket Communication**: Fast and persistent communication channel between client and server.
-   **Cross-Platform Payload Builder**: Easily compile payloads for Windows, Linux, and macOS.
-   **Network Scanner**: The client can scan the local network to automatically discover active payloads.
-   **Multi-Target Command Execution**: Send a single command to one, multiple, or all connected payloads simultaneously.
-   **Pure Go**: The entire framework is written in Go, making the payload a single, dependency-free executable.

## How It Works

The framework is split into two main components:

1.  **The C2 Client (`main.go`)**: This is the command-line interface you run on your machine. It's used to:
    -   Build and compile the payload for different operating systems.
    -   Scan the network for active payloads.
    -   Connect to payloads and send commands.

2.  **The Payload (`serverCode`)**: This is the server component that runs on the target machine. It:
    -   Listens for incoming WebSocket connections on port `1234`.
    -   Receives commands from the C2 client.
    -   Executes the commands on the target's shell.
    -   Sends the command output back to the client.

## Prerequisites

-   [Go](https://go.dev/doc/install) (version 1.18 or later is recommended).

## Installation

1.  Clone the repository or save the `main.go` file to your local machine.

2.  Install the required `gorilla/websocket` dependency:
    ```sh
    go get github.com/gorilla/websocket
    ```

## Usage Workflow

Here is a typical workflow for using the framework.

### Step 1: Run the C2 Client

Navigate to the directory containing `main.go` and run the application:

```sh
go run main.go
```
<img width="480" height="270" alt="image" src="https://github.com/user-attachments/assets/f95b127f-a78c-4979-b3d9-2dea2f4591d7" />


You will be greeted with the main menu.

### Step 2: Build the Payload

1.  From the main menu, select option `[2] Build Payload`.
2.  Choose the target operating system (e.g., `[2] Build for Windows`).
3.  The tool will compile the server code and create an executable in the same directory (e.g., `payload.exe`).

   
<img width="480" height="270" alt="image" src="https://github.com/user-attachments/assets/68d4c379-1386-44c5-9db4-b708e00d7c81" />

### Step 3: Deploy and Execute the Payload

1.  Transfer the generated payload executable (e.g., `payload.exe`) to your target machine.
2.  Execute the payload on the target machine. It will run in the background and start listening for connections on port `1234`.

### Step 4: Connect and Send Commands

1.  Return to the C2 client on your machine and select option `[1] Run Client`.
2.  The client will automatically scan the local network for machines running the payload on port `1234`.

   <img width="1005" height="349" alt="image" src="https://github.com/user-attachments/assets/ec85efeb-a76a-4733-9a55-39e66a74998b" />


    

4.  Once targets are found, they will be listed. You can choose to connect to a single target, a specific group (e.g., `1,3`), or all targets (`0`).
5.  After connecting, you will see a `>` prompt. You can now send any shell command (like `whoami`, `ls`, `pwd`, `ipconfig`, etc.) to the connected payload(s). The response from each target will be displayed.

    

6.  To change targets, type `!target`. To exit the client, type `exit`.

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.
