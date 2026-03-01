# sshrun

A simple SSH client tool implemented in Go language, capable of connecting to a specified SSH server, executing commands, and transferring files.

## Features

- Execute commands on remote SSH servers
- Upload local files to remote servers
- Download files from remote servers to local machine
- Support hex-encoded parameters to handle special characters
- Support reading commands from files
- Cross-platform support (Windows and Linux)

## Installation

### Prerequisites

- Go 1.18 or later

### Build from source

```bash
# Windows
go build -o sshrun.exe sshrun.go

# Linux
go build -o sshrun sshrun.go
```

## Usage

### Command Line Parameters

| Parameter | Description | Example |
|-----------|-------------|---------|
| `-host` | SSH server IP address (required) | `-host=192.168.1.1` |
| `-port` | SSH server port (default: 22) | `-port=22` |
| `-user` | SSH username (required) | `-user=root` |
| `-password` | SSH password, support HEX_ prefixed hex encoding | `-password=password` or `-password=HEX_70617373776F7264` |
| `-cmd` | Command to execute, support HEX_ prefixed hex encoding | `-cmd=date` or `-cmd=HEX_64617465` |
| `-cmdfile` | Read commands from file, each line in the file is a command | `-cmdfile=commands.txt` |
| `-type` | Function type: cmd (execute command), upload (upload file), download (download file), default value is cmd | `-type=upload` |
| `-localPath` | Local file path (required for upload or download) | `-localPath=C:\files\test.txt` |
| `-remotePath` | Remote file path (required for upload or download) | `-remotePath=/home/user/files/` |
| `-fileName` | Target file name for upload or download, default value is the original file name | `-fileName=newfile.txt` |

### Examples

#### Execute command

```bash
# Execute date command
sshrun.exe -host=192.168.1.1 -port=22 -user=root -password=password -cmd=date

# Execute multiple commands from file
sshrun.exe -host=192.168.1.1 -port=22 -user=root -password=password -cmdfile=commands.txt
```

#### Upload file

```bash
# Upload local file to remote server
sshrun.exe -host=192.168.1.1 -port=22 -user=root -password=password -type=upload -localPath=C:\files\test.txt -remotePath=/home/user/files/

# Upload file with custom name
sshrun.exe -host=192.168.1.1 -port=22 -user=root -password=password -type=upload -localPath=C:\files\test.txt -remotePath=/home/user/files/ -fileName=newfile.txt
```

#### Download file

```bash
# Download file from remote server
sshrun.exe -host=192.168.1.1 -port=22 -user=root -password=password -type=download -localPath=C:\files\ -remotePath=/home/user/files/test.txt

# Download file with custom name
sshrun.exe -host=192.168.1.1 -port=22 -user=root -password=password -type=download -localPath=C:\files\ -remotePath=/home/user/files/test.txt -fileName=downloaded.txt
```

## Documentation

For more detailed documentation, please refer to [sshrun.md](sshrun.md).

## Dependencies

- `golang.org/x/crypto/ssh` - SSH client implementation
- `github.com/pkg/sftp` - SFTP client implementation

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
