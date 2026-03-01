package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

func main() {
	// Define command line parameters
	host := flag.String("host", "", "SSH server IP address")
	port := flag.String("port", "22", "SSH server port")
	user := flag.String("user", "", "SSH username")
	password := flag.String("password", "", "SSH password, support HEX_ prefixed hex encoding")
	cmd := flag.String("cmd", "", "Command to execute, support HEX_ prefixed hex encoding")
	cmdFile := flag.String("cmdfile", "", "Read commands from file")
	typeFlag := flag.String("type", "cmd", "Function type: cmd (execute command), upload (upload file), download (download file)")
	localPath := flag.String("localPath", "", "Local file path")
	remotePath := flag.String("remotePath", "", "Remote file path")
	fileName := flag.String("fileName", "", "Target file name for upload or download")
	flag.Parse()

	// Validate required parameters
	if *host == "" || *user == "" || *password == "" {
		log.Fatal("Must specify host, user and password parameters")
	}

	// Process password
	pass := *password
	if strings.HasPrefix(pass, "HEX_") {
		// Decode hex password
		hexStr := strings.TrimPrefix(pass, "HEX_")
		decoded, err := hex.DecodeString(hexStr)
		if err != nil {
			log.Fatalf("Invalid HEX password: %v", err)
		}
		pass = string(decoded)
	}

	// Process host address
	hostAddr := *host
	if strings.HasPrefix(hostAddr, "HEX_") {
		hexStr := strings.TrimPrefix(hostAddr, "HEX_")
		decoded, err := hex.DecodeString(hexStr)
		if err != nil {
			log.Fatalf("Invalid HEX host address: %v", err)
		}
		hostAddr = string(decoded)
	}

	// Process port
	portStr := *port
	if strings.HasPrefix(portStr, "HEX_") {
		hexStr := strings.TrimPrefix(portStr, "HEX_")
		decoded, err := hex.DecodeString(hexStr)
		if err != nil {
			log.Fatalf("Invalid HEX port: %v", err)
		}
		portStr = string(decoded)
	}

	// Process username
	userName := *user
	if strings.HasPrefix(userName, "HEX_") {
		hexStr := strings.TrimPrefix(userName, "HEX_")
		decoded, err := hex.DecodeString(hexStr)
		if err != nil {
			log.Fatalf("Invalid HEX username: %v", err)
		}
		userName = string(decoded)
	}

	// Configure SSH client
	config := &ssh.ClientConfig{
		User: userName,
		Auth: []ssh.AuthMethod{
			ssh.Password(pass),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	// Connect to SSH server
	addr := net.JoinHostPort(hostAddr, portStr)
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		log.Fatalf("Connection failed: %v", err)
	}
	defer client.Close()

	// Execute different operations based on function type
	switch strings.ToLower(*typeFlag) {
	case "cmd":
		// Process command
		command := *cmd
		// Support hex encoded command
		if strings.HasPrefix(command, "HEX_") {
			hexStr := strings.TrimPrefix(command, "HEX_")
			decoded, err := hex.DecodeString(hexStr)
			if err != nil {
				log.Fatalf("Invalid HEX command: %v", err)
			}
			command = string(decoded)
		}

		// Read commands from file
		var commands []string
		if *cmdFile != "" {
			// Process cmdFile parameter
			cmdFilePath := *cmdFile
			if strings.HasPrefix(cmdFilePath, "HEX_") {
				hexStr := strings.TrimPrefix(cmdFilePath, "HEX_")
				decoded, err := hex.DecodeString(hexStr)
				if err != nil {
					log.Fatalf("Invalid HEX command file path: %v", err)
				}
				cmdFilePath = string(decoded)
			}

			data, err := os.ReadFile(cmdFilePath)
			if err != nil {
				log.Fatalf("Failed to read command file: %v", err)
			}
			lines := strings.Split(string(data), "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line != "" {
					commands = append(commands, line)
				}
			}
		} else {
			if command != "" {
				commands = append(commands, command)
			}
		}

		if len(commands) == 0 {
			log.Fatal("Must specify cmd or cmdfile parameter")
		}

		// Execute all commands
		for _, cmd := range commands {
			// Create SSH session
			session, err := client.NewSession()
			if err != nil {
				log.Fatalf("Failed to create session: %v", err)
			}

			// Execute command
			output, err := session.CombinedOutput(cmd)
			if err != nil {
				session.Close()
				log.Fatalf("Failed to execute command: %v", err)
			}

			// Print command execution result
			fmt.Printf("%s", output)
			session.Close()
		}

	case "upload":
		// Validate upload parameters
		if *localPath == "" || *remotePath == "" {
			log.Fatal("Must specify localPath and remotePath parameters for upload")
		}

		// Process local path
		local := *localPath
		if strings.HasPrefix(local, "HEX_") {
			hexStr := strings.TrimPrefix(local, "HEX_")
			decoded, err := hex.DecodeString(hexStr)
			if err != nil {
				log.Fatalf("Invalid HEX local path: %v", err)
			}
			local = string(decoded)
		}

		// Process remote path
		remote := *remotePath
		if strings.HasPrefix(remote, "HEX_") {
			hexStr := strings.TrimPrefix(remote, "HEX_")
			decoded, err := hex.DecodeString(hexStr)
			if err != nil {
				log.Fatalf("Invalid HEX remote path: %v", err)
			}
			remote = string(decoded)
		}

		// Process file name
		destFileName := *fileName
		if destFileName == "" {
			// Use original file name
			destFileName = filepath.Base(local)
		} else if strings.HasPrefix(destFileName, "HEX_") {
			// Decode hex file name
			hexStr := strings.TrimPrefix(destFileName, "HEX_")
			decoded, err := hex.DecodeString(hexStr)
			if err != nil {
				log.Fatalf("Invalid HEX file name: %v", err)
			}
			destFileName = string(decoded)
		}

		// Ensure remote path ends with /
		if !strings.HasSuffix(remote, "/") {
			remote += "/"
		}
		remoteFile := remote + destFileName

		// Create SFTP client
		sftpClient, err := sftp.NewClient(client)
		if err != nil {
			log.Fatalf("Failed to create SFTP client: %v", err)
		}
		defer sftpClient.Close()

		// Read local file
		data, err := os.ReadFile(local)
		if err != nil {
			log.Fatalf("Failed to read local file: %v", err)
		}

		// Create remote file and write data
		file, err := sftpClient.Create(remoteFile)
		if err != nil {
			log.Fatalf("Failed to create remote file: %v", err)
		}
		defer file.Close()

		written, err := file.Write(data)
		if err != nil {
			log.Fatalf("Failed to write remote file: %v", err)
		}

		fmt.Printf("Successfully uploaded %d bytes to %s\n", written, remoteFile)

	case "download":
		// Validate download parameters
		if *localPath == "" || *remotePath == "" {
			log.Fatal("Must specify localPath and remotePath parameters for download")
		}

		// Process local path
		local := *localPath
		if strings.HasPrefix(local, "HEX_") {
			hexStr := strings.TrimPrefix(local, "HEX_")
			decoded, err := hex.DecodeString(hexStr)
			if err != nil {
				log.Fatalf("Invalid HEX local path: %v", err)
			}
			local = string(decoded)
		}

		// Process remote path
		remote := *remotePath
		if strings.HasPrefix(remote, "HEX_") {
			hexStr := strings.TrimPrefix(remote, "HEX_")
			decoded, err := hex.DecodeString(hexStr)
			if err != nil {
				log.Fatalf("Invalid HEX remote path: %v", err)
			}
			remote = string(decoded)
		}

		// Process file name
		destFileName := *fileName
		if destFileName == "" {
			// Use original file name
			destFileName = filepath.Base(remote)
		} else if strings.HasPrefix(destFileName, "HEX_") {
			// Decode hex file name
			hexStr := strings.TrimPrefix(destFileName, "HEX_")
			decoded, err := hex.DecodeString(hexStr)
			if err != nil {
				log.Fatalf("Invalid HEX file name: %v", err)
			}
			destFileName = string(decoded)
		}

		// Ensure local path ends with / or \
		if !strings.HasSuffix(local, "/") && !strings.HasSuffix(local, "\\") {
			if strings.Contains(local, "\\") {
				local += "\\"
			} else {
				local += "/"
			}
		}
		localFile := local + destFileName

		// Create SFTP client
		sftpClient, err := sftp.NewClient(client)
		if err != nil {
			log.Fatalf("Failed to create SFTP client: %v", err)
		}
		defer sftpClient.Close()

		// Open remote file
		file, err := sftpClient.Open(remote)
		if err != nil {
			log.Fatalf("Failed to open remote file: %v", err)
		}
		defer file.Close()

		// Read remote file content
		data, err := io.ReadAll(file)
		if err != nil {
			log.Fatalf("Failed to read remote file: %v", err)
		}

		// Ensure local directory exists
		localDir := filepath.Dir(localFile)
		if err := os.MkdirAll(localDir, 0755); err != nil {
			log.Fatalf("Failed to create local directory: %v", err)
		}

		// Write to local file
		if err := os.WriteFile(localFile, data, 0644); err != nil {
			log.Fatalf("Failed to write local file: %v", err)
		}

		fmt.Printf("Successfully downloaded %d bytes to %s\n", len(data), localFile)

	default:
		log.Fatalf("Invalid function type: %s, optional values are cmd, upload, download", *typeFlag)
	}
}
